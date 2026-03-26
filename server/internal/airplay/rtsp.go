package airplay

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
)

// RTSP request parsed from raw TCP
type RTSPRequest struct {
	Method  string
	URI     string
	Version string
	Headers map[string]string
	Body    []byte
	CSeq    string
}

func (s *Server) handleRTSPConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("RTSP connection from %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)

	for {
		req, err := readRTSPRequest(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("RTSP read error: %v", err)
			}
			return
		}

		log.Printf("RTSP %s %s CSeq=%s", req.Method, req.URI, req.CSeq)

		var respBody string
		var respHeaders map[string]string
		status := "200 OK"

		switch {
		case req.Method == "OPTIONS":
			respHeaders = map[string]string{
				"Public": "ANNOUNCE, SETUP, RECORD, PAUSE, FLUSH, FLUSHBUFFERED, TEARDOWN, OPTIONS, POST, GET, SET_PARAMETER",
			}

		case req.Method == "GET" && strings.HasSuffix(req.URI, "/info"):
			respBody = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>deviceid</key><string>%s</string>
	<key>features</key><integer>%d</integer>
	<key>model</key><string>%s</string>
	<key>name</key><string>%s</string>
	<key>srcvers</key><string>%s</string>
	<key>statusFlags</key><integer>%d</integer>
</dict>
</plist>`, s.Config.DeviceID, s.Config.Features, s.Config.Model, s.Config.Name, s.Config.SrcVersion, s.Config.StatusFlags)
			respHeaders = map[string]string{"Content-Type": "text/x-apple-plist+xml"}

		case req.Method == "POST" && strings.HasSuffix(req.URI, "/pair-pin-start"):
			log.Printf("RTSP pair-pin-start from %s (PIN: %s)", conn.RemoteAddr(), s.Config.PIN)
			s.EmitEvent("pairing", map[string]interface{}{
				"step":    "pin-start",
				"message": "PIN pairing initiated - PIN: " + s.Config.PIN,
			})
			writeRTSPResponse(conn, "200 OK", req.CSeq, nil, "")
			continue

		case req.Method == "POST" && strings.HasSuffix(req.URI, "/pair-setup"):
			// Forward to pairing handler
			s.handleRTSPPairSetup(conn, req)
			continue

		case req.Method == "POST" && strings.HasSuffix(req.URI, "/pair-verify"):
			s.handleRTSPPairVerify(conn, req)
			continue

		case req.Method == "ANNOUNCE":
			s.EmitEvent("audio_announce", map[string]interface{}{
				"sdp": string(req.Body),
			})
			log.Printf("RTSP ANNOUNCE: audio session announced")

		case req.Method == "SETUP":
			// Parse transport header for audio setup
			transport := req.Headers["Transport"]
			log.Printf("RTSP SETUP transport: %s", transport)
			s.EmitEvent("audio_setup", map[string]interface{}{
				"transport": transport,
			})
			respHeaders = map[string]string{
				"Transport": fmt.Sprintf("RTP/AVP/UDP;unicast;mode=record;server_port=%d;control_port=%d;timing_port=%d",
					s.Config.AirTunesPort+1, s.Config.AirTunesPort+2, s.Config.AirTunesPort+3),
				"Session": "1",
				"Audio-Jack-Status": "connected; type=analog",
			}

		case req.Method == "RECORD":
			log.Printf("RTSP RECORD: audio streaming started")
			s.EmitEvent("audio_start", nil)

		case req.Method == "SET_PARAMETER":
			s.handleRTSPSetParameter(req)

		case req.Method == "FLUSH":
			log.Printf("RTSP FLUSH")
			s.EmitEvent("audio_flush", nil)

		case req.Method == "TEARDOWN":
			log.Printf("RTSP TEARDOWN: audio session ended")
			s.EmitEvent("audio_stop", nil)

		case req.Method == "POST" && strings.HasSuffix(req.URI, "/feedback"):
			// Feedback - just acknowledge

		case req.Method == "POST" && strings.HasSuffix(req.URI, "/command"):
			log.Printf("RTSP command: %s", string(req.Body))

		case req.Method == "POST" && strings.HasSuffix(req.URI, "/audioMode"):
			log.Printf("RTSP audioMode: %s", string(req.Body))

		default:
			log.Printf("Unhandled RTSP: %s %s", req.Method, req.URI)
		}

		writeRTSPResponse(conn, status, req.CSeq, respHeaders, respBody)
	}
}

func (s *Server) handleRTSPPairSetup(conn net.Conn, req *RTSPRequest) {
	tlvs := tlvDecode(req.Body)
	state := byte(0)
	if st, ok := tlvs[TLVState]; ok && len(st) > 0 {
		state = st[0]
	}

	log.Printf("RTSP pair-setup state=%d", state)

	var resp []byte
	switch state {
	case 1:
		salt := make([]byte, 16)
		serverPub := make([]byte, 384)
		resp = tlvEncode(map[byte][]byte{
			TLVState:     {0x02},
			TLVSalt:      salt,
			TLVPublicKey: serverPub,
		})
	case 3:
		proof := make([]byte, 64)
		resp = tlvEncode(map[byte][]byte{
			TLVState: {0x04},
			TLVProof: proof,
		})
	case 5:
		resp = tlvEncode(map[byte][]byte{
			TLVState: {0x06},
		})
	default:
		resp = tlvEncode(map[byte][]byte{
			TLVState: {state + 1},
		})
	}

	writeRTSPResponse(conn, "200 OK", req.CSeq, map[string]string{
		"Content-Type": "application/octet-stream",
	}, string(resp))
}

func (s *Server) handleRTSPPairVerify(conn net.Conn, req *RTSPRequest) {
	tlvs := tlvDecode(req.Body)
	state := byte(0)
	if st, ok := tlvs[TLVState]; ok && len(st) > 0 {
		state = st[0]
	}

	log.Printf("RTSP pair-verify state=%d", state)

	var resp []byte
	switch state {
	case 1:
		resp = tlvEncode(map[byte][]byte{
			TLVState:     {0x02},
			TLVPublicKey: globalPairSession.curvePub[:],
			TLVEncData:   []byte("verified"),
		})
	case 3:
		resp = tlvEncode(map[byte][]byte{
			TLVState: {0x04},
		})
	default:
		resp = tlvEncode(map[byte][]byte{
			TLVState: {state + 1},
		})
	}

	writeRTSPResponse(conn, "200 OK", req.CSeq, map[string]string{
		"Content-Type": "application/octet-stream",
	}, string(resp))
}

func (s *Server) handleRTSPSetParameter(req *RTSPRequest) {
	ct := req.Headers["Content-Type"]
	switch ct {
	case "text/parameters":
		params := parseTextParameters(string(req.Body))
		if vol, ok := params["volume"]; ok {
			v, _ := strconv.ParseFloat(strings.TrimSpace(vol), 64)
			s.EmitEvent("volume", map[string]interface{}{"volume": v})
			log.Printf("Volume: %.1f", v)
		}
	case "image/jpeg", "image/png":
		s.EmitEvent("audio_artwork", map[string]interface{}{
			"size":        len(req.Body),
			"contentType": ct,
		})
	default:
		// Could be DMAP metadata
		s.EmitEvent("audio_metadata", map[string]interface{}{
			"contentType": ct,
			"size":        len(req.Body),
		})
	}
}

func readRTSPRequest(reader *bufio.Reader) (*RTSPRequest, error) {
	// Read request line
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid RTSP request line: %s", line)
	}

	req := &RTSPRequest{
		Method:  parts[0],
		URI:     parts[1],
		Headers: make(map[string]string),
	}
	if len(parts) > 2 {
		req.Version = parts[2]
	}

	// Read headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if idx := strings.Index(line, ": "); idx > 0 {
			key := line[:idx]
			val := line[idx+2:]
			req.Headers[key] = val
			if strings.EqualFold(key, "CSeq") {
				req.CSeq = val
			}
		}
	}

	// Read body if Content-Length present
	if clStr, ok := req.Headers["Content-Length"]; ok {
		cl, _ := strconv.Atoi(clStr)
		if cl > 0 {
			req.Body = make([]byte, cl)
			_, err := io.ReadFull(reader, req.Body)
			if err != nil {
				return nil, err
			}
		}
	}

	return req, nil
}

func writeRTSPResponse(conn net.Conn, status, cseq string, headers map[string]string, body string) {
	resp := fmt.Sprintf("RTSP/1.0 %s\r\n", status)
	if cseq != "" {
		resp += fmt.Sprintf("CSeq: %s\r\n", cseq)
	}
	resp += fmt.Sprintf("Content-Length: %d\r\n", len(body))
	for k, v := range headers {
		resp += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	resp += "\r\n"
	resp += body
	conn.Write([]byte(resp))
}
