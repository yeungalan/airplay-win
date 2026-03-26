package airplay

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type wsMsg struct {
	isBinary bool
	data     []byte
}

type wsClient struct {
	conn *websocket.Conn
	send chan wsMsg
}

var (
	wsClients   = make(map[*wsClient]bool)
	wsClientsMu sync.RWMutex
)

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	wsHandler := websocket.Handler(func(ws *websocket.Conn) {
		client := &wsClient{
			conn: ws,
			send: make(chan wsMsg, 64),
		}

		wsClientsMu.Lock()
		wsClients[client] = true
		wsClientsMu.Unlock()

		log.Printf("WebSocket client connected: %s", ws.Request().RemoteAddr)

		// Send initial status
		s.sendStatus(client)

		// Writer goroutine
		done := make(chan struct{})
		go func() {
			defer close(done)
			for msg := range client.send {
				if msg.isBinary {
					websocket.Message.Send(ws, msg.data)
				} else {
					websocket.Message.Send(ws, string(msg.data))
				}
			}
		}()

		// Reader goroutine (handle commands from frontend)
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := ws.Read(buf)
				if err != nil {
					break
				}
				s.handleWSMessage(client, buf[:n])
			}
			wsClientsMu.Lock()
			delete(wsClients, client)
			wsClientsMu.Unlock()
			close(client.send)
		}()

		// Event broadcaster
		for {
			select {
			case evt, ok := <-s.EventCh:
				if !ok {
					return
				}
				data, _ := json.Marshal(evt)
				s.broadcastWS(data)
			case <-done:
				return
			}
		}
	})

	wsHandler.ServeHTTP(w, r)
}

func (s *Server) broadcastWS(data []byte) {
	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()
	for client := range wsClients {
		msg := wsMsg{isBinary: false, data: data}
		select {
		case client.send <- msg:
		default:
		}
	}
}

func (s *Server) broadcastBinary(data []byte) {
	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()
	for client := range wsClients {
		msg := wsMsg{isBinary: true, data: data}
		select {
		case client.send <- msg:
		default:
		}
	}
}

func (s *Server) sendStatus(client *wsClient) {
	s.Playback.mu.RLock()
	s.PairState.mu.RLock()
	status := map[string]interface{}{
		"type": "status",
		"data": map[string]interface{}{
			"name":     s.Config.Name,
			"deviceId": s.Config.DeviceID,
			"model":    s.Config.Model,
			"paired":   s.PairState.Paired,
			"playing":  s.Playback.Playing,
			"url":      s.Playback.URL,
			"position": s.Playback.Position,
			"duration": s.Playback.Duration,
			"rate":     s.Playback.Rate,
			"width":    s.Config.Width,
			"height":   s.Config.Height,
		},
	}
	s.PairState.mu.RUnlock()
	s.Playback.mu.RUnlock()

	data, _ := json.Marshal(status)
	select {
	case client.send <- wsMsg{isBinary: false, data: data}:
	default:
	}
}

func (s *Server) handleWSMessage(client *wsClient, msg []byte) {
	var cmd struct {
		Action string          `json:"action"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msg, &cmd); err != nil {
		return
	}

	switch cmd.Action {
	case "get_status":
		s.sendStatus(client)
	case "update_position":
		var d struct {
			Position float64 `json:"position"`
			Duration float64 `json:"duration"`
		}
		json.Unmarshal(cmd.Data, &d)
		s.Playback.mu.Lock()
		s.Playback.Position = d.Position
		s.Playback.Duration = d.Duration
		s.Playback.mu.Unlock()
	}
}

func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	s.Playback.mu.RLock()
	s.PairState.mu.RLock()
	status := map[string]interface{}{
		"name":     s.Config.Name,
		"deviceId": s.Config.DeviceID,
		"model":    s.Config.Model,
		"paired":   s.PairState.Paired,
		"playing":  s.Playback.Playing,
		"url":      s.Playback.URL,
		"position": s.Playback.Position,
		"duration": s.Playback.Duration,
		"rate":     s.Playback.Rate,
		"width":    s.Config.Width,
		"height":   s.Config.Height,
		"uptime":   time.Now().Unix(),
	}
	s.PairState.mu.RUnlock()
	s.Playback.mu.RUnlock()

	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleAPIPhoto(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.PhotoMu.RLock()
	data := s.PhotoData
	s.PhotoMu.RUnlock()

	if len(data) == 0 {
		http.Error(w, "No photo", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(data)
}
