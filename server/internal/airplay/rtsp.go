package airplay

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// sessionEncryption wraps a net.Conn with HAP ChaCha20-Poly1305 session encryption.
// After pair-verify M4, all RTSP traffic is framed as:
//
//	[2-byte LE plaintext length][ciphertext][16-byte Poly1305 tag]
//
// The 2-byte length is also the AAD. The nonce is an 8-byte LE counter
// zero-padded to 12 bytes (bytes 0-3 = 0, bytes 4-11 = counter LE).
type sessionEncryption struct {
	conn       net.Conn
	readKey    []byte
	writeKey   []byte
	readNonce  uint64
	writeNonce uint64
	readBuf    []byte
}

func (e *sessionEncryption) Read(b []byte) (int, error) {
	if len(e.readBuf) > 0 {
		n := copy(b, e.readBuf)
		e.readBuf = e.readBuf[n:]
		return n, nil
	}
	var lenBuf [2]byte
	if _, err := io.ReadFull(e.conn, lenBuf[:]); err != nil {
		log.Printf("Session enc: read length error (nonce=%d): %v", e.readNonce, err)
		return 0, err
	}
	plen := int(binary.LittleEndian.Uint16(lenBuf[:]))
	log.Printf("Session enc: reading frame plen=%d nonce=%d", plen, e.readNonce)
	encrypted := make([]byte, plen+16)
	if _, err := io.ReadFull(e.conn, encrypted); err != nil {
		return 0, err
	}
	var nonce [12]byte
	binary.LittleEndian.PutUint64(nonce[4:], e.readNonce)
	e.readNonce++
	aead, _ := chacha20poly1305.New(e.readKey)
	plaintext, err := aead.Open(nil, nonce[:], encrypted, lenBuf[:])
	if err != nil {
		log.Printf("Session enc: decrypt FAILED nonce=%d plen=%d err=%v", e.readNonce-1, plen, err)
		return 0, fmt.Errorf("session decrypt: %w", err)
	}
	log.Printf("Session enc: decrypted %d bytes: %.80q", len(plaintext), plaintext)
	n := copy(b, plaintext)
	if n < len(plaintext) {
		e.readBuf = make([]byte, len(plaintext)-n)
		copy(e.readBuf, plaintext[n:])
	}
	return n, nil
}

func (e *sessionEncryption) Write(b []byte) (int, error) {
	const maxBlock = 1024
	total := 0
	for len(b) > 0 {
		block := b
		if len(block) > maxBlock {
			block = b[:maxBlock]
		}
		b = b[len(block):]
		var lenBuf [2]byte
		binary.LittleEndian.PutUint16(lenBuf[:], uint16(len(block)))
		var nonce [12]byte
		binary.LittleEndian.PutUint64(nonce[4:], e.writeNonce)
		e.writeNonce++
		aead, _ := chacha20poly1305.New(e.writeKey)
		encrypted := aead.Seal(nil, nonce[:], block, lenBuf[:])
		frame := make([]byte, 2+len(encrypted))
		copy(frame, lenBuf[:])
		copy(frame[2:], encrypted)
		if _, err := e.conn.Write(frame); err != nil {
			return total, err
		}
		total += len(block)
	}
	return total, nil
}

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
	var writer io.Writer = conn // switches to sessionEncryption after pair-verify M4

	for {
		req, err := readRTSPRequest(reader)
		if err != nil {
			log.Printf("RTSP read error (encrypted=%v): %v", writer != conn, err)
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
			if s.handleRTSPPairVerify(conn, req) {
				// Pair-verify M4 just sent — upgrade connection to session encryption.
				// Controller→Accessory uses "Control-Write-Encryption-Key" (our read key).
				// Accessory→Controller uses "Control-Read-Encryption-Key" (our write key).
				globalPairSession.mu.Lock()
				sharedSecret := make([]byte, len(globalPairSession.sharedSecret))
				copy(sharedSecret, globalPairSession.sharedSecret)
				globalPairSession.mu.Unlock()
				readKey := deriveKey(sharedSecret, "Control-Salt", "Control-Write-Encryption-Key")
				writeKey := deriveKey(sharedSecret, "Control-Salt", "Control-Read-Encryption-Key")
				enc := &sessionEncryption{conn: conn, readKey: readKey, writeKey: writeKey}
				reader = bufio.NewReader(enc)
				writer = enc
				log.Printf("RTSP session encrypted")
			}
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
				"Session":           "1",
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

		writeRTSPResponse(writer, status, req.CSeq, respHeaders, respBody)
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
		// M2: real SRP-6a parameters
		salt := srpNewSalt()
		v := srpVerifier(salt, s.Config.PIN)
		bPriv, bPub := srpServerKeys(v)

		globalPairSession.mu.Lock()
		globalPairSession.setupStep = 2
		globalPairSession.srpSalt = salt
		globalPairSession.srpV = v
		globalPairSession.srpBPriv = bPriv
		globalPairSession.srpBPub = bPub
		globalPairSession.mu.Unlock()

		resp = tlvEncode(map[byte][]byte{
			TLVState:     {0x02},
			TLVSalt:      salt,
			TLVPublicKey: srpPad(bPub),
		})

		s.EmitEvent("pairing", map[string]interface{}{
			"step":    "M1-M2",
			"message": "Pairing started - PIN: " + s.Config.PIN,
		})

	case 3:
		// M4: compute session key, send server proof
		aBytes := tlvs[TLVPublicKey]
		m1Client := tlvs[TLVProof]

		globalPairSession.mu.Lock()
		bPriv := globalPairSession.srpBPriv
		bPub := globalPairSession.srpBPub
		v := globalPairSession.srpV

		var m2 []byte
		var K []byte
		if bPriv != nil && bPub != nil && v != nil && len(aBytes) > 0 {
			A := new(big.Int).SetBytes(aBytes)
			K = srpSessionKey(A, bPub, bPriv, v)
			if K != nil {
				m2 = srpServerProof(A, m1Client, K)
			}
		}
		if m2 == nil {
			m2 = make([]byte, 64)
			rand.Read(m2)
		}
		globalPairSession.setupStep = 4
		globalPairSession.sharedSecret = K
		globalPairSession.srpK = K
		globalPairSession.mu.Unlock()

		resp = tlvEncode(map[byte][]byte{
			TLVState: {0x04},
			TLVProof: m2,
		})

	case 5:
		globalPairSession.mu.Lock()
		srpK := globalPairSession.srpK
		globalPairSession.setupStep = 6
		globalPairSession.paired = true
		globalPairSession.mu.Unlock()
		s.PairState.mu.Lock()
		s.PairState.Paired = true
		s.PairState.mu.Unlock()
		log.Printf("RTSP pairing complete!")
		s.EmitEvent("pairing", map[string]interface{}{"step": "M5-M6", "message": "Pairing complete!", "paired": true})
		encData := buildPairSetupM6(srpK, s.Config.DeviceID, globalPairSession.serverPrivKey, globalPairSession.serverPubKey)
		resp = tlvEncode(map[byte][]byte{TLVState: {0x06}, TLVEncData: encData})
	default:
		resp = tlvEncode(map[byte][]byte{
			TLVState: {state + 1},
		})
	}

	writeRTSPResponse(conn, "200 OK", req.CSeq, map[string]string{
		"Content-Type": "application/octet-stream",
	}, string(resp))
}

// handleRTSPPairVerify handles pair-verify M1 and M3.
// Returns true when M4 (state=3 response) has been written — caller must
// immediately upgrade the connection to session encryption.
func (s *Server) handleRTSPPairVerify(conn net.Conn, req *RTSPRequest) bool {
	tlvs := tlvDecode(req.Body)
	state := byte(0)
	if st, ok := tlvs[TLVState]; ok && len(st) > 0 {
		state = st[0]
	}

	log.Printf("RTSP pair-verify state=%d", state)

	var resp []byte
	switch state {
	case 1:
		clientPub := tlvs[TLVPublicKey]

		// Generate fresh ephemeral Curve25519 keypair for this verify session
		var ephPriv, ephPub [32]byte
		rand.Read(ephPriv[:])
		curve25519.ScalarBaseMult(&ephPub, &ephPriv)

		globalPairSession.mu.Lock()
		globalPairSession.peerPub = clientPub
		globalPairSession.verifyStep = 2
		globalPairSession.curvePriv = ephPriv
		globalPairSession.curvePub = ephPub

		var shared [32]byte
		if len(clientPub) == 32 {
			var peerPub [32]byte
			copy(peerPub[:], clientPub)
			curve25519.ScalarMult(&shared, &ephPriv, &peerPub)
			globalPairSession.sharedSecret = shared[:]
		}
		globalPairSession.mu.Unlock()

		verifyKey := deriveKey(shared[:], "Pair-Verify-Encrypt-Salt", "Pair-Verify-Encrypt-Info")
		// Signature covers: server ephemeral pub || server pairing ID || client ephemeral pub
		sigMaterial := append(append(ephPub[:], []byte(s.Config.DeviceID)...), clientPub...)
		sig := ed25519.Sign(globalPairSession.serverPrivKey, sigMaterial)
		inner := tlvEncode(map[byte][]byte{
			TLVIdentifier: []byte(s.Config.DeviceID),
			TLVSignature:  sig,
		})
		nonce := [12]byte{}
		copy(nonce[4:], "PV-Msg02")
		aead, _ := chacha20poly1305.New(verifyKey)
		encData := aead.Seal(nil, nonce[:], inner, nil)
		resp = tlvEncode(map[byte][]byte{
			TLVState:     {0x02},
			TLVPublicKey: ephPub[:],
			TLVEncData:   encData,
		})

	case 3:
		// M4: pair-verify complete. Write M4 plaintext, then caller upgrades to encrypted.
		resp = tlvEncode(map[byte][]byte{TLVState: {0x04}})
		writeRTSPResponse(conn, "200 OK", req.CSeq, map[string]string{
			"Content-Type": "application/octet-stream",
		}, string(resp))
		log.Printf("RTSP pair-verify complete, upgrading to session encryption")
		return true

	default:
		resp = tlvEncode(map[byte][]byte{TLVState: {state + 1}})
	}

	writeRTSPResponse(conn, "200 OK", req.CSeq, map[string]string{
		"Content-Type": "application/octet-stream",
	}, string(resp))
	return false
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

	// Read body if Content-Length present (case-insensitive lookup per RTSP RFC)
	var clStr string
	for k, v := range req.Headers {
		if strings.EqualFold(k, "content-length") {
			clStr = v
			break
		}
	}
	if clStr != "" {
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

func writeRTSPResponse(w io.Writer, status, cseq string, headers map[string]string, body string) {
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
	w.Write([]byte(resp))
}
