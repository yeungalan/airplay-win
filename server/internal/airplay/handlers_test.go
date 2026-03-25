package airplay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer() *Server {
	cfg := DefaultConfig()
	cfg.Port = 0
	cfg.MirrorPort = 0
	cfg.AirTunesPort = 0
	return NewServer(cfg)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Port != 7000 {
		t.Errorf("expected port 7000, got %d", cfg.Port)
	}
	if cfg.MirrorPort != 7100 {
		t.Errorf("expected mirror port 7100, got %d", cfg.MirrorPort)
	}
	if cfg.Features&FeatureVideo == 0 {
		t.Error("expected Video feature to be set")
	}
	if cfg.Features&FeaturePhoto == 0 {
		t.Error("expected Photo feature to be set")
	}
	if cfg.Features&FeatureScreen == 0 {
		t.Error("expected Screen feature to be set")
	}
	if cfg.Features&FeatureAudio == 0 {
		t.Error("expected Audio feature to be set")
	}
	if cfg.PIN != "3939" {
		t.Errorf("expected PIN 3939, got %s", cfg.PIN)
	}
	if cfg.DeviceID == "" {
		t.Error("expected non-empty DeviceID")
	}
}

func TestServerInfo(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/server-info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, s.Config.DeviceID) {
		t.Error("response should contain device ID")
	}
	if !strings.Contains(body, s.Config.Model) {
		t.Error("response should contain model")
	}
	if w.Header().Get("Content-Type") != "text/x-apple-plist+xml" {
		t.Errorf("expected plist content type, got %s", w.Header().Get("Content-Type"))
	}
}

func TestInfo(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var info map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if info["deviceid"] != s.Config.DeviceID {
		t.Error("deviceid mismatch")
	}
	if info["name"] != s.Config.Name {
		t.Error("name mismatch")
	}
	if info["model"] != s.Config.Model {
		t.Error("model mismatch")
	}
}

func TestPlay(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	body := "Content-Location: http://example.com/video.mp4\nStart-Position: 0.5\n"
	req := httptest.NewRequest("POST", "/play", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	s.Playback.mu.RLock()
	defer s.Playback.mu.RUnlock()
	if !s.Playback.Playing {
		t.Error("expected playing to be true")
	}
	if s.Playback.URL != "http://example.com/video.mp4" {
		t.Errorf("expected URL, got %s", s.Playback.URL)
	}
	if s.Playback.Position != 0.5 {
		t.Errorf("expected position 0.5, got %f", s.Playback.Position)
	}
}

func TestScrub(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// POST scrub
	req := httptest.NewRequest("POST", "/scrub?position=42.5", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	s.Playback.mu.RLock()
	if s.Playback.Position != 42.5 {
		t.Errorf("expected position 42.5, got %f", s.Playback.Position)
	}
	s.Playback.mu.RUnlock()

	// GET scrub
	s.Playback.mu.Lock()
	s.Playback.Duration = 100.0
	s.Playback.mu.Unlock()

	req = httptest.NewRequest("GET", "/scrub", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "duration: 100") {
		t.Errorf("expected duration in response, got: %s", body)
	}
	if !strings.Contains(body, "position: 42.5") {
		t.Errorf("expected position in response, got: %s", body)
	}
}

func TestRate(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// Pause
	req := httptest.NewRequest("POST", "/rate?value=0.0", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	s.Playback.mu.RLock()
	if s.Playback.Playing {
		t.Error("expected not playing after rate=0")
	}
	s.Playback.mu.RUnlock()

	// Play
	req = httptest.NewRequest("POST", "/rate?value=1.0", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	s.Playback.mu.RLock()
	if !s.Playback.Playing {
		t.Error("expected playing after rate=1")
	}
	if s.Playback.Rate != 1.0 {
		t.Errorf("expected rate 1.0, got %f", s.Playback.Rate)
	}
	s.Playback.mu.RUnlock()
}

func TestStop(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// Start playing first
	s.Playback.mu.Lock()
	s.Playback.Playing = true
	s.Playback.URL = "http://example.com/video.mp4"
	s.Playback.mu.Unlock()

	req := httptest.NewRequest("POST", "/stop", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	s.Playback.mu.RLock()
	if s.Playback.Playing {
		t.Error("expected not playing after stop")
	}
	if s.Playback.URL != "" {
		t.Error("expected empty URL after stop")
	}
	s.Playback.mu.RUnlock()
}

func TestPhoto(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10} // Fake JPEG header
	req := httptest.NewRequest("PUT", "/photo", bytes.NewReader(jpegData))
	req.Header.Set("X-Apple-AssetKey", "test-asset-123")
	req.Header.Set("X-Apple-Transition", "Dissolve")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	s.PhotoMu.RLock()
	if len(s.PhotoData) != len(jpegData) {
		t.Errorf("expected photo data length %d, got %d", len(jpegData), len(s.PhotoData))
	}
	s.PhotoMu.RUnlock()
}

func TestPhotoWrongMethod(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/photo", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSlideshowFeatures(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/slideshow-features", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Dissolve") {
		t.Error("expected Dissolve theme")
	}
	if !strings.Contains(body, "Classic") {
		t.Error("expected Classic theme")
	}
}

func TestPlaybackInfo(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	s.Playback.mu.Lock()
	s.Playback.Playing = true
	s.Playback.Duration = 120.0
	s.Playback.Position = 30.0
	s.Playback.Rate = 1.0
	s.Playback.mu.Unlock()

	req := httptest.NewRequest("GET", "/playback-info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<real>120") {
		t.Error("expected duration in response")
	}
	if !strings.Contains(body, "<real>30") {
		t.Error("expected position in response")
	}
	if !strings.Contains(body, "<true/>") {
		t.Error("expected readyToPlay true")
	}
}

func TestSetProperty(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("PUT", "/setProperty?forwardEndTime", strings.NewReader("<plist/>"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "errorCode") {
		t.Error("expected errorCode in response")
	}
}

func TestAPIStatus(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var status map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if status["name"] != s.Config.Name {
		t.Error("name mismatch in status")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header")
	}
}

func TestAPIPhotoNotFound(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("GET", "/api/photo", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAPIPhotoFound(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	s.PhotoMu.Lock()
	s.PhotoData = []byte{0xFF, 0xD8, 0xFF}
	s.PhotoMu.Unlock()

	req := httptest.NewRequest("GET", "/api/photo", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Error("expected image/jpeg content type")
	}
}

func TestFeedback(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("POST", "/feedback", strings.NewReader(""))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCommand(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := httptest.NewRequest("POST", "/command", strings.NewReader(`{"type":"cycleAudioRoute"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestParseTextParameters(t *testing.T) {
	input := "Content-Location: http://example.com/video.mp4\nStart-Position: 0.5\n"
	params := parseTextParameters(input)

	if params["Content-Location"] != "http://example.com/video.mp4" {
		t.Errorf("expected URL, got %s", params["Content-Location"])
	}
	if params["Start-Position"] != "0.5" {
		t.Errorf("expected 0.5, got %s", params["Start-Position"])
	}
}

func TestEmitEvent(t *testing.T) {
	s := newTestServer()

	s.EmitEvent("test", map[string]string{"key": "value"})

	select {
	case evt := <-s.EventCh:
		if evt.Type != "test" {
			t.Errorf("expected event type 'test', got '%s'", evt.Type)
		}
	default:
		t.Error("expected event in channel")
	}
}

func TestNTPTimestamp(t *testing.T) {
	ts := ntpTimestamp(time.Now())
	if ts == 0 {
		t.Error("expected non-zero NTP timestamp")
	}
}

// Benchmark the server info handler
func BenchmarkServerInfo(b *testing.B) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/server-info", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}
}

// Integration test: full play/scrub/stop cycle
func TestPlaybackCycle(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// Play
	body := "Content-Location: http://example.com/test.mp4\nStart-Position: 0.0\n"
	req := httptest.NewRequest("POST", "/play", strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal("play failed")
	}

	// Scrub
	req = httptest.NewRequest("POST", "/scrub?position=50.0", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal("scrub failed")
	}

	// Rate (pause)
	req = httptest.NewRequest("POST", "/rate?value=0", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Rate (resume)
	req = httptest.NewRequest("POST", "/rate?value=1", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Get playback info
	req = httptest.NewRequest("GET", "/playback-info", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "<true/>") {
		t.Error("expected readyToPlay true")
	}

	// Stop
	req = httptest.NewRequest("POST", "/stop", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	s.Playback.mu.RLock()
	if s.Playback.Playing {
		t.Error("should not be playing after stop")
	}
	s.Playback.mu.RUnlock()
}

// Test concurrent access
func TestConcurrentAccess(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			url := fmt.Sprintf("http://example.com/video%d.mp4", i)
			body := fmt.Sprintf("Content-Location: %s\nStart-Position: 0.0\n", url)
			req := httptest.NewRequest("POST", "/play", strings.NewReader(body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			req = httptest.NewRequest("GET", "/api/status", nil)
			w = httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			done <- w.Code == 200
		}(i)
	}

	for i := 0; i < 10; i++ {
		if !<-done {
			t.Error("concurrent request failed")
		}
	}
}

// Drain events helper
func drainEvents(s *Server) {
	for {
		select {
		case <-s.EventCh:
		default:
			return
		}
	}
}

func init() {
	// Suppress log output during tests
	_ = io.Discard
}
