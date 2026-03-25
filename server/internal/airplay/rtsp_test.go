package airplay

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadRTSPRequest(t *testing.T) {
	raw := "OPTIONS * RTSP/1.0\r\nCSeq: 1\r\nUser-Agent: AirPlay/380.20.1\r\nContent-Length: 0\r\n\r\n"
	reader := bufio.NewReader(strings.NewReader(raw))

	req, err := readRTSPRequest(reader)
	if err != nil {
		t.Fatalf("failed to read RTSP request: %v", err)
	}

	if req.Method != "OPTIONS" {
		t.Errorf("expected OPTIONS, got %s", req.Method)
	}
	if req.URI != "*" {
		t.Errorf("expected *, got %s", req.URI)
	}
	if req.CSeq != "1" {
		t.Errorf("expected CSeq 1, got %s", req.CSeq)
	}
}

func TestReadRTSPRequestWithBody(t *testing.T) {
	body := "v=0\r\no=iTunes 123456 0 IN IP4 192.168.1.1\r\n"
	raw := "ANNOUNCE rtsp://192.168.1.2/123 RTSP/1.0\r\n" +
		"CSeq: 2\r\n" +
		"Content-Type: application/sdp\r\n" +
		"Content-Length: " + string(rune(len(body))) + "\r\n" +
		"\r\n" +
		body

	// Use explicit content length
	raw = "ANNOUNCE rtsp://192.168.1.2/123 RTSP/1.0\r\nCSeq: 2\r\nContent-Type: application/sdp\r\nContent-Length: 42\r\n\r\n" + body

	reader := bufio.NewReader(strings.NewReader(raw))
	req, err := readRTSPRequest(reader)
	if err != nil {
		t.Fatalf("failed to read RTSP request: %v", err)
	}

	if req.Method != "ANNOUNCE" {
		t.Errorf("expected ANNOUNCE, got %s", req.Method)
	}
	if len(req.Body) != 42 {
		t.Errorf("expected body length 42, got %d", len(req.Body))
	}
}

func TestReadRTSPRequestSetup(t *testing.T) {
	raw := "SETUP rtsp://192.168.1.2/123 RTSP/1.0\r\n" +
		"CSeq: 3\r\n" +
		"Transport: RTP/AVP/UDP;unicast;interleaved=0-1;mode=record;control_port=6001;timing_port=6002\r\n" +
		"Content-Length: 0\r\n\r\n"

	reader := bufio.NewReader(strings.NewReader(raw))
	req, err := readRTSPRequest(reader)
	if err != nil {
		t.Fatalf("failed to read RTSP request: %v", err)
	}

	if req.Method != "SETUP" {
		t.Errorf("expected SETUP, got %s", req.Method)
	}
	transport := req.Headers["Transport"]
	if !strings.Contains(transport, "RTP/AVP/UDP") {
		t.Error("expected transport header")
	}
}

func TestMirrorPacketHeader(t *testing.T) {
	// Test packet type constants
	if PacketTypeVideo != 0 {
		t.Error("video type should be 0")
	}
	if PacketTypeCodecData != 1 {
		t.Error("codec data type should be 1")
	}
	if PacketTypeHeartbeat != 2 {
		t.Error("heartbeat type should be 2")
	}
	if MirrorHeaderSize != 128 {
		t.Error("header size should be 128")
	}
}
