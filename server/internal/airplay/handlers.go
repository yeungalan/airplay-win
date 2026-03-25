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
	mux.HandleFunc("/pair-setup", s.handlePairSetup)
	mux.HandleFunc("/pair-verify", s.handlePairVerify)
	mux.HandleFunc("/fp-setup", s.handleFPSetup)
	mux.HandleFunc("/feedback", s.handleFeedback)
	mux.HandleFunc("/command", s.handleCommand)
	mux.HandleFunc("/audioMode", s.handleAudioMode)

	// Frontend API endpoints (WebSocket + REST)
	mux.HandleFunc("/api/ws", s.handleWebSocket)
	mux.HandleFunc("/api/status", s.handleAPIStatus)
	mux.HandleFunc("/api/photo", s.handleAPIPhoto)

	// CORS middleware wrapper
	return mux
}

func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET /server-info from %s", r.RemoteAddr)
	featLow := uint32(s.Config.Features & 0xFFFFFFFF)
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>deviceid</key>
	<string>%s</string>
	<key>features</key>
	<integer>%d</integer>
	<key>model</key>
	<string>%s</string>
	<key>protovers</key>
	<string>1.0</string>
	<key>srcvers</key>
	<string>%s</string>
</dict>
</plist>`, s.Config.DeviceID, featLow, s.Config.Model, s.Config.SrcVersion)

	w.Header().Set("Content-Type", "text/x-apple-plist+xml")
	w.Write([]byte(plist))
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET /info from %s", r.RemoteAddr)
	info := map[string]interface{}{
		"deviceid":     s.Config.DeviceID,
		"features":     s.Config.Features,
		"model":        s.Config.Model,
		"protovers":    "1.1",
		"srcvers":      s.Config.SrcVersion,
		"name":         s.Config.Name,
		"statusFlags":  s.Config.StatusFlags,
		"pi":           "b08f5a79-db29-4384-b456-a4784d9e6055",
		"pk":           "99FD4299889422515FBD27949E4E1E21B2AF50A454499E3D4BE75A4E0F55FE63",
		"vv":           2,
		"audioFormats": []map[string]interface{}{
			{"type": 96, "audioInputFormats": 67108860, "audioOutputFormats": 67108860},
		},
		"audioLatencies": []map[string]interface{}{
			{"type": 96, "audioType": "default", "inputLatencyMicros": 0, "outputLatencyMicros": 400000},
		},
		"displays": []map[string]interface{}{
			{
				"width":       s.Config.Width,
				"height":      s.Config.Height,
				"uuid":        "e5f7a68d-7b2f-4b3e-b1d1-fd2d5cf74634",
				"widthPixels": s.Config.Width,
				"heightPixels": s.Config.Height,
				"rotation":    true,
				"overscanned": false,
				"features":    14,
			},
		},
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

	// Parse text/parameters format
	params := parseTextParameters(string(body))
	url := params["Content-Location"]
	startPos := 0.0
	if sp, ok := params["Start-Position"]; ok {
		startPos, _ = strconv.ParseFloat(strings.TrimSpace(sp), 64)
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
