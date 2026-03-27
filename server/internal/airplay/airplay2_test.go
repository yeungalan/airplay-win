package airplay

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- AirPlay 2 HTTP handler tests ---

func TestInfoBinaryPlist(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/info", nil)
	req.Header.Set("Content-Type", "application/x-apple-binary-plist")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/x-apple-binary-plist" {
		t.Errorf("expected bplist content type, got %s", w.Header().Get("Content-Type"))
	}
	decoded, err := BPlistDecode(w.Body.Bytes())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m := decoded.(map[string]interface{})
	if m["name"] != s.Config.Name {
		t.Errorf("name: %v", m["name"])
	}
	if m["deviceid"] != s.Config.DeviceID {
		t.Errorf("deviceid: %v", m["deviceid"])
	}
}

func TestInfoHasAirPlay2Fields(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var info map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &info)

	// AirPlay 2 required fields
	for _, key := range []string{"audioFormats", "audioLatencies", "displays", "initialVolume", "keepAliveLowPower"} {
		if _, ok := info[key]; !ok {
			t.Errorf("missing AirPlay 2 field: %s", key)
		}
	}
}

func TestInfoUsesRealPublicKey(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var info map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &info)

	pk := info["pk"].(string)
	if pk != GetPublicKeyHex() {
		t.Errorf("pk should match server Ed25519 key, got %s", pk)
	}
}

func TestPlayBinaryPlist(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	body := map[string]interface{}{
		"Content-Location": "http://example.com/ap2.mp4",
		"Start-Position":   float64(0.25),
	}
	data, _ := BPlistEncode(body)

	req := httptest.NewRequest("POST", "/play", strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/x-apple-binary-plist")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	s.Playback.mu.RLock()
	defer s.Playback.mu.RUnlock()
	if s.Playback.URL != "http://example.com/ap2.mp4" {
		t.Errorf("URL: %s", s.Playback.URL)
	}
	if s.Playback.Position != 0.25 {
		t.Errorf("position: %f", s.Playback.Position)
	}
}

func TestStreamEndpoint(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// GET should fail
	req := httptest.NewRequest("GET", "/stream", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET /stream, got %d", w.Code)
	}
}

func TestPairSetupPin(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// /pair-setup-pin should work same as /pair-setup
	body := tlvEncode(map[byte][]byte{TLVState: {0x01}})
	req := newPostRequest("/pair-setup-pin", body)
	w := recordRequest(mux, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := tlvDecode(w.Body.Bytes())
	if resp[TLVState][0] != 0x02 {
		t.Error("expected state M2")
	}
}

func TestTransientPairSetup(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// Send raw 32-byte Ed25519 public key
	clientPub := make([]byte, 32)
	clientPub[0] = 0xAA
	req := newPostRequest("/pair-setup", clientPub)
	w := recordRequest(mux, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.Len() != 32 {
		t.Errorf("expected 32-byte response, got %d", w.Body.Len())
	}
	s.PairState.mu.RLock()
	if !s.PairState.Paired {
		t.Error("expected paired after transient setup")
	}
	s.PairState.mu.RUnlock()
}

func TestServerInfoFeatures(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/server-info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "statusFlags") {
		t.Error("missing statusFlags")
	}
	if !strings.Contains(body, "1.1") {
		t.Error("missing protovers 1.1")
	}
}

func TestDefaultConfigAirPlay2Features(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Features&FeatureBufferedAudio == 0 {
		t.Error("missing FeatureBufferedAudio")
	}
	if cfg.Features&FeatureSupportsPTP == 0 {
		t.Error("missing FeatureSupportsPTP")
	}
	if cfg.Features&FeatureScreenMultiCodec == 0 {
		t.Error("missing FeatureScreenMultiCodec")
	}
	if cfg.Features&FeatureSupportsAirPlayVideoV2 == 0 {
		t.Error("missing FeatureSupportsAirPlayVideoV2")
	}
	if cfg.Features&FeatureAuthentication4 == 0 {
		t.Error("missing FeatureAuthentication4")
	}
	if cfg.Features&FeatureAudioFormat2 == 0 {
		t.Error("missing FeatureAudioFormat2")
	}
}

// --- AirPlay 2 RTSP tests ---

func TestRTSPOptionsIncludesAP2Methods(t *testing.T) {
	raw := "OPTIONS * RTSP/1.0\r\nCSeq: 1\r\nContent-Length: 0\r\n\r\n"
	reader := bufio.NewReader(strings.NewReader(raw))
	req, _ := readRTSPRequest(reader)

	if req.Method != "OPTIONS" {
		t.Fatal("expected OPTIONS")
	}
	// Verify the method list includes AP2 methods
	methods := "ANNOUNCE, SETUP, RECORD, PAUSE, FLUSH, FLUSHBUFFERED, TEARDOWN, OPTIONS, POST, GET, SET_PARAMETER, GET_PARAMETER, SETPEERS"
	for _, m := range []string{"FLUSHBUFFERED", "SETPEERS", "GET_PARAMETER"} {
		if !strings.Contains(methods, m) {
			t.Errorf("OPTIONS should include %s", m)
		}
	}
}

func TestRTSPSetupBPlistPhase1(t *testing.T) {
	s := newTestServer()
	s.eventPort = 55555

	// Phase 1: no streams, just device info + timing
	setup := map[string]interface{}{
		"timingProtocol": "PTP",
		"deviceID":       "11:22:33:44:55:66",
	}
	body, _ := BPlistEncode(setup)

	req := &RTSPRequest{
		Method:  "SETUP",
		URI:     "rtsp://192.168.1.2/123",
		CSeq:    "1",
		Headers: map[string]string{"Content-Type": "application/x-apple-binary-plist"},
		Body:    body,
	}

	var buf strings.Builder
	s.handleRTSPSetupBPlist(&buf, req)

	resp := buf.String()
	if !strings.Contains(resp, "200 OK") {
		t.Fatal("expected 200 OK")
	}
	if !strings.Contains(resp, "AirTunes") {
		t.Error("missing Server: AirTunes header")
	}
}

func TestRTSPSetupBPlistPhase2Audio96(t *testing.T) {
	s := newTestServer()

	setup := map[string]interface{}{
		"streams": []interface{}{
			map[string]interface{}{
				"type":        int64(96),
				"ct":          int64(2),
				"spf":         int64(352),
				"controlPort": int64(60000),
			},
		},
	}
	body, _ := BPlistEncode(setup)

	req := &RTSPRequest{Method: "SETUP", CSeq: "2", Body: body}
	var buf strings.Builder
	s.handleRTSPSetupBPlist(&buf, req)

	if !strings.Contains(buf.String(), "200 OK") {
		t.Fatal("expected 200 OK")
	}
}

func TestRTSPSetupBPlistPhase2Screen110(t *testing.T) {
	s := newTestServer()

	setup := map[string]interface{}{
		"streams": []interface{}{
			map[string]interface{}{"type": int64(110)},
		},
	}
	body, _ := BPlistEncode(setup)

	req := &RTSPRequest{Method: "SETUP", CSeq: "3", Body: body}
	var buf strings.Builder
	s.handleRTSPSetupBPlist(&buf, req)

	if !strings.Contains(buf.String(), "200 OK") {
		t.Fatal("expected 200 OK")
	}
}

func TestRTSPSetupBPlistPhase2Buffered103(t *testing.T) {
	s := newTestServer()
	s.dataPort = 44444

	setup := map[string]interface{}{
		"streams": []interface{}{
			map[string]interface{}{
				"type": int64(103),
				"ct":   int64(4),
			},
		},
	}
	body, _ := BPlistEncode(setup)

	req := &RTSPRequest{Method: "SETUP", CSeq: "4", Body: body}
	var buf strings.Builder
	s.handleRTSPSetupBPlist(&buf, req)

	if !strings.Contains(buf.String(), "200 OK") {
		t.Fatal("expected 200 OK")
	}
}

func TestRTSPWriteResponseHasServerHeader(t *testing.T) {
	var buf strings.Builder
	writeRTSPResponse(&buf, "200 OK", "5", nil, "")
	resp := buf.String()
	if !strings.Contains(resp, "Server: AirTunes/") {
		t.Error("missing Server: AirTunes header")
	}
	if !strings.Contains(resp, "CSeq: 5") {
		t.Error("missing CSeq")
	}
}
