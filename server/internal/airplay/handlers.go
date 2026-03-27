package airplay

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) buildAirPlayMux() *http.ServeMux {
	mux := http.NewServeMux()

	// AirPlay protocol endpoints
	mux.HandleFunc("/server-info", s.handleServerInfo)
	mux.HandleFunc("/info", s.handleInfo)
	mux.HandleFunc("/play", s.handlePlay)
	mux.HandleFunc("/scrub", s.handleScrub)
	mux.HandleFunc("/rate", s.handleRate)
	mux.HandleFunc("/stop", s.handleStop)
	mux.HandleFunc("/photo", s.handlePhoto)
	mux.HandleFunc("/slideshow-features", s.handleSlideshowFeatures)
	mux.HandleFunc("/slideshows/1", s.handleSlideshow)
	mux.HandleFunc("/playback-info", s.handlePlaybackInfo)
	mux.HandleFunc("/setProperty", s.handleSetProperty)
	mux.HandleFunc("/getProperty", s.handleGetProperty)
	mux.HandleFunc("/pair-pin-start", s.handlePairPinStart)
	mux.HandleFunc("/pair-setup-pin", s.handlePairSetup) // PIN-based pair-setup (same handler, TLV format)
	mux.HandleFunc("/pair-setup", s.handlePairSetup)
	mux.HandleFunc("/pair-verify", s.handlePairVerify)
	mux.HandleFunc("/fp-setup", s.handleFPSetup)
	mux.HandleFunc("/feedback", s.handleFeedback)
	mux.HandleFunc("/command", s.handleCommand)
	mux.HandleFunc("/audioMode", s.handleAudioMode)

	// AirPlay 2 endpoints
	mux.HandleFunc("/stream", s.handleStream) // POST /stream for AirPlay 2 video

	// Frontend API endpoints (WebSocket + REST)
	mux.HandleFunc("/api/ws", s.handleWebSocket)
	mux.HandleFunc("/api/status", s.handleAPIStatus)
	mux.HandleFunc("/api/photo", s.handleAPIPhoto)

	return mux
}

func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET /server-info from %s", r.RemoteAddr)
	featLow := uint32(s.Config.Features & 0xFFFFFFFF)
	featHigh := uint32(s.Config.Features >> 32)
	featuresStr := fmt.Sprintf("%d", featLow)
	if featHigh > 0 {
		featuresStr = fmt.Sprintf("%d", s.Config.Features)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>deviceid</key>
	<string>%s</string>
	<key>features</key>
	<integer>%s</integer>
	<key>model</key>
	<string>%s</string>
	<key>protovers</key>
	<string>1.1</string>
	<key>srcvers</key>
	<string>%s</string>
	<key>statusFlags</key>
	<integer>%d</integer>
</dict>
</plist>`, s.Config.DeviceID, featuresStr, s.Config.Model, s.Config.SrcVersion, s.Config.StatusFlags)

	w.Header().Set("Content-Type", "text/x-apple-plist+xml")
	w.Write([]byte(plist))
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET /info from %s", r.RemoteAddr)

	ct := r.Header.Get("Content-Type")
	// AirPlay 2 clients may send binary plist body with qualifier
	if r.Body != nil && ct == "application/x-apple-binary-plist" {
		io.ReadAll(r.Body)
	}

	info := map[string]interface{}{
		"deviceid":                 s.Config.DeviceID,
		"features":                 int64(s.Config.Features),
		"model":                    s.Config.Model,
		"protovers":                "1.1",
		"srcvers":                  s.Config.SrcVersion,
		"name":                     s.Config.Name,
		"statusFlags":              int64(s.Config.StatusFlags),
		"keepAliveLowPower":        int64(1),
		"keepAliveSendStatsAsBody": int64(1),
		"initialVolume":            float64(-20.0),
		"audioFormats": []interface{}{
			map[string]interface{}{
				"type":               int64(96),
				"audioInputFormats":  int64(0x01000000), // AAC-ELD
				"audioOutputFormats": int64(0x01000000),
			},
		},
		"audioLatencies": []interface{}{
			map[string]interface{}{
				"type":                int64(96),
				"audioType":           "default",
				"inputLatencyMicros":  int64(0),
				"outputLatencyMicros": int64(400000),
			},
			map[string]interface{}{
				"type":                int64(103),
				"audioType":           "default",
				"inputLatencyMicros":  int64(0),
				"outputLatencyMicros": int64(400000),
			},
		},
		"displays": []interface{}{
			map[string]interface{}{
				"width":                  int64(s.Config.Width),
				"height":                 int64(s.Config.Height),
				"uuid":                   "e5f7a68d-7b2f-4b3e-b1d1-fd2d5cf74634",
				"widthPixels":            int64(s.Config.Width),
				"heightPixels":           int64(s.Config.Height),
				"rotation":               true,
				"overscanned":            false,
				"features":               int64(14),
				"widthPhysical":          int64(0),
				"heightPhysical":         int64(0),
				"refreshRate":            float64(60.0),
				"maxFPS":                 int64(30),
				"primaryInputDevice":     int64(1),
			},
		},
	}

	// Respond with binary plist if client accepts it, else JSON
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/x-apple-binary-plist") || ct == "application/x-apple-binary-plist" {
		data, err := BPlistEncode(info)
		if err == nil {
			w.Header().Set("Content-Type", "application/x-apple-binary-plist")
			w.Write(data)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("POST /play from %s", r.RemoteAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var url string
	var startPos float64

	ct := r.Header.Get("Content-Type")
	// AirPlay 2 may send binary plist
	if ct == "application/x-apple-binary-plist" || (len(body) > 8 && string(body[:8]) == "bplist00") {
		parsed, perr := BPlistDecode(body)
		if perr == nil {
			if m, ok := parsed.(map[string]interface{}); ok {
				if u, ok := m["Content-Location"].(string); ok {
					url = u
				}
				if sp, ok := m["Start-Position"].(float64); ok {
					startPos = sp
				}
				if sp, ok := m["Start-Position"].(int64); ok {
					startPos = float64(sp)
				}
			}
		}
	} else {
		// AirPlay 1 text/parameters format
		params := parseTextParameters(string(body))
		url = params["Content-Location"]
		if sp, ok := params["Start-Position"]; ok {
			startPos, _ = strconv.ParseFloat(strings.TrimSpace(sp), 64)
		}
	}

	s.Playback.mu.Lock()
	s.Playback.Playing = true
	s.Playback.URL = url
	s.Playback.Position = startPos
	s.Playback.Rate = 1.0
	s.Playback.mu.Unlock()

	s.EmitEvent("play", map[string]interface{}{
		"url":           url,
		"startPosition": startPos,
	})

	log.Printf("Playing: %s at position %.3f", url, startPos)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleScrub(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		posStr := r.URL.Query().Get("position")
		pos, _ := strconv.ParseFloat(posStr, 64)
		s.Playback.mu.Lock()
		s.Playback.Position = pos
		s.Playback.mu.Unlock()
		s.EmitEvent("scrub", map[string]interface{}{"position": pos})
		log.Printf("POST /scrub position=%.3f", pos)
		w.WriteHeader(http.StatusOK)
		return
	}

	// GET /scrub - return current position
	s.Playback.mu.RLock()
	pos := s.Playback.Position
	dur := s.Playback.Duration
	s.Playback.mu.RUnlock()

	w.Header().Set("Content-Type", "text/parameters")
	fmt.Fprintf(w, "duration: %.6f\nposition: %.6f\n", dur, pos)
}

func (s *Server) handleRate(w http.ResponseWriter, r *http.Request) {
	valStr := r.URL.Query().Get("value")
	rate, _ := strconv.ParseFloat(valStr, 64)
	s.Playback.mu.Lock()
	s.Playback.Rate = rate
	if rate == 0 {
		s.Playback.Playing = false
	} else {
		s.Playback.Playing = true
	}
	s.Playback.mu.Unlock()
	s.EmitEvent("rate", map[string]interface{}{"value": rate})
	log.Printf("POST /rate value=%.3f", rate)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	log.Printf("POST /stop from %s", r.RemoteAddr)
	s.Playback.mu.Lock()
	s.Playback.Playing = false
	s.Playback.URL = ""
	s.Playback.Position = 0
	s.Playback.Rate = 0
	s.Playback.mu.Unlock()
	s.EmitEvent("stop", nil)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handlePhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("PUT /photo from %s", r.RemoteAddr)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	assetKey := r.Header.Get("X-Apple-AssetKey")
	transition := r.Header.Get("X-Apple-Transition")

	s.PhotoMu.Lock()
	s.PhotoData = data
	s.PhotoMu.Unlock()

	s.EmitEvent("photo", map[string]interface{}{
		"assetKey":   assetKey,
		"transition": transition,
		"size":       len(data),
		"dataBase64": base64.StdEncoding.EncodeToString(data),
	})

	log.Printf("Photo received: %d bytes, key=%s, transition=%s", len(data), assetKey, transition)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSlideshowFeatures(w http.ResponseWriter, r *http.Request) {
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>themes</key>
	<array>
		<dict><key>key</key><string>Dissolve</string><key>name</key><string>Dissolve</string></dict>
		<dict><key>key</key><string>Classic</string><key>name</key><string>Classic</string></dict>
		<dict><key>key</key><string>Reflections</string><key>name</key><string>Reflections</string></dict>
		<dict><key>key</key><string>KenBurns</string><key>name</key><string>Ken Burns</string></dict>
	</array>
</dict>
</plist>`
	w.Header().Set("Content-Type", "text/x-apple-plist+xml")
	w.Write([]byte(plist))
}

func (s *Server) handleSlideshow(w http.ResponseWriter, r *http.Request) {
	log.Printf("PUT /slideshows/1 from %s", r.RemoteAddr)
	body, _ := io.ReadAll(r.Body)
	s.EmitEvent("slideshow", map[string]interface{}{"body": string(body)})
	w.Header().Set("Content-Type", "text/x-apple-plist+xml")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict/></plist>`))
}

func (s *Server) handlePlaybackInfo(w http.ResponseWriter, r *http.Request) {
	s.Playback.mu.RLock()
	dur := s.Playback.Duration
	pos := s.Playback.Position
	rate := s.Playback.Rate
	playing := s.Playback.Playing
	s.Playback.mu.RUnlock()

	readyToPlay := "false"
	if playing {
		readyToPlay = "true"
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>duration</key><real>%f</real>
	<key>position</key><real>%f</real>
	<key>rate</key><real>%f</real>
	<key>readyToPlay</key><%s/>
	<key>playbackBufferEmpty</key><true/>
	<key>playbackBufferFull</key><false/>
	<key>playbackLikelyToKeepUp</key><true/>
	<key>loadedTimeRanges</key>
	<array><dict><key>duration</key><real>%f</real><key>start</key><real>0.0</real></dict></array>
	<key>seekableTimeRanges</key>
	<array><dict><key>duration</key><real>%f</real><key>start</key><real>0.0</real></dict></array>
</dict>
</plist>`, dur, pos, rate, readyToPlay, dur, dur)

	w.Header().Set("Content-Type", "text/x-apple-plist+xml")
	w.Write([]byte(plist))
}

func (s *Server) handleSetProperty(w http.ResponseWriter, r *http.Request) {
	log.Printf("PUT /setProperty from %s, query=%s", r.RemoteAddr, r.URL.RawQuery)
	io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/x-apple-binary-plist")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>errorCode</key><integer>0</integer></dict></plist>`))
}

func (s *Server) handleGetProperty(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET /getProperty from %s, query=%s", r.RemoteAddr, r.URL.RawQuery)
	w.Header().Set("Content-Type", "application/x-apple-binary-plist")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>errorCode</key><integer>0</integer></dict></plist>`))
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	io.ReadAll(r.Body)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	log.Printf("POST /command: %s", string(body))
	s.EmitEvent("command", map[string]interface{}{"body": string(body)})
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleAudioMode(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	log.Printf("POST /audioMode: %s", string(body))
	w.WriteHeader(http.StatusOK)
}

func parseTextParameters(body string) map[string]string {
	params := make(map[string]string)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, ": "); idx > 0 {
			params[line[:idx]] = line[idx+2:]
		}
	}
	return params
}

// handleStream handles POST /stream for AirPlay 2 video streaming.
// The client sends a binary plist with stream parameters, then the connection
// transitions to a raw H.264 binary stream (same as mirror port 7100).
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("POST /stream from %s", r.RemoteAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Parse binary plist stream parameters
	var streamInfo map[string]interface{}
	if len(body) > 0 {
		parsed, perr := BPlistDecode(body)
		if perr == nil {
			if m, ok := parsed.(map[string]interface{}); ok {
				streamInfo = m
			}
		}
	}
	if streamInfo == nil {
		streamInfo = make(map[string]interface{})
	}

	log.Printf("Stream info: sessionID=%v, latencyMs=%v", streamInfo["sessionID"], streamInfo["latencyMs"])

	s.EmitEvent("mirror_start", map[string]interface{}{
		"width":  s.Config.Width,
		"height": s.Config.Height,
		"source": "stream",
	})

	// Hijack the connection to read raw H.264 stream
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijack not supported", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		log.Printf("Stream hijack error: %v", err)
		return
	}
	defer conn.Close()

	// Send 200 OK
	bufrw.WriteString("HTTP/1.1 200 OK\r\n\r\n")
	bufrw.Flush()

	// Read mirror stream packets (same 128-byte header format)
	s.readMirrorStream(conn)

	s.EmitEvent("mirror_stop", nil)
}
