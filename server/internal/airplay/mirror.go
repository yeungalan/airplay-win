package airplay

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
)

// Mirror stream packet types per AirPlay spec
const (
	PacketTypeVideo     = 0 // H.264 video bitstream
	PacketTypeCodecData = 1 // H.264 codec data (avcC format)
	PacketTypeHeartbeat = 2 // Heartbeat (no payload)
)

const MirrorHeaderSize = 128

type MirrorPacketHeader struct {
	PayloadSize  uint32
	PayloadType  uint16
	HeaderExtra  uint16
	NTPTimestamp uint64
}

func (s *Server) handleMirrorConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("Mirror connection from %s", conn.RemoteAddr())

	buf := make([]byte, 4096)
	httpBuf := make([]byte, 0, 8192)

	// First read the HTTP request (GET /stream.xml or POST /stream)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("Mirror read error: %v", err)
			return
		}
		httpBuf = append(httpBuf, buf[:n]...)

		req := string(httpBuf)
		if len(req) > 4 {
			if req[:3] == "GET" {
				s.handleMirrorGET(conn, req)
				httpBuf = httpBuf[:0]
				continue
			}
			if req[:4] == "POST" {
				s.handleMirrorPOST(conn, req, httpBuf)
				return // POST /stream transitions to binary stream
			}
		}
	}
}

func (s *Server) handleMirrorGET(conn net.Conn, req string) {
	log.Printf("Mirror GET /stream.xml")
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>height</key>
	<integer>%d</integer>
	<key>overscanned</key>
	<true/>
	<key>refreshRate</key>
	<real>0.016666666666666666</real>
	<key>version</key>
	<string>%s</string>
	<key>width</key>
	<integer>%d</integer>
</dict>
</plist>`, s.Config.Height, s.Config.SrcVersion, s.Config.Width)

	resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/x-apple-plist+xml\r\nContent-Length: %d\r\n\r\n%s", len(plist), plist)
	conn.Write([]byte(resp))
}

func (s *Server) handleMirrorPOST(conn net.Conn, req string, httpBuf []byte) {
	log.Printf("Mirror POST /stream - starting mirror session")

	// Send 200 OK
	conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	s.EmitEvent("mirror_start", map[string]interface{}{
		"width":  s.Config.Width,
		"height": s.Config.Height,
	})

	// Read binary stream packets
	s.readMirrorStream(conn)

	s.EmitEvent("mirror_stop", nil)
}

func (s *Server) readMirrorStream(conn net.Conn) {
	header := make([]byte, MirrorHeaderSize)

	for {
		// Read 128-byte header
		_, err := io.ReadFull(conn, header)
		if err != nil {
			if err != io.EOF {
				log.Printf("Mirror stream header read error: %v", err)
			}
			return
		}

		pkt := MirrorPacketHeader{
			PayloadSize:  binary.LittleEndian.Uint32(header[0:4]),
			PayloadType:  binary.LittleEndian.Uint16(header[4:6]),
			HeaderExtra:  binary.LittleEndian.Uint16(header[6:8]),
			NTPTimestamp: binary.LittleEndian.Uint64(header[8:16]),
		}

		switch pkt.PayloadType {
		case PacketTypeHeartbeat:
			log.Printf("Mirror heartbeat")
			continue

		case PacketTypeCodecData:
			if pkt.PayloadSize > 0 {
				payload := make([]byte, pkt.PayloadSize)
				if _, err := io.ReadFull(conn, payload); err != nil {
					log.Printf("Mirror codec data read error: %v", err)
					return
				}
				s.EmitEvent("mirror_codec", map[string]interface{}{
					"size": pkt.PayloadSize,
				})
				log.Printf("Mirror codec data: %d bytes", pkt.PayloadSize)
			}

		case PacketTypeVideo:
			if pkt.PayloadSize > 0 && pkt.PayloadSize < 10*1024*1024 {
				payload := make([]byte, pkt.PayloadSize)
				if _, err := io.ReadFull(conn, payload); err != nil {
					log.Printf("Mirror video read error: %v", err)
					return
				}
				// Send to channel for frontend consumption
				select {
				case s.MirrorCh <- payload:
				default:
					// Drop frame if channel full
				}
			}

		default:
			// Skip unknown packet types
			if pkt.PayloadSize > 0 {
				io.CopyN(io.Discard, conn, int64(pkt.PayloadSize))
			}
		}
	}
}
