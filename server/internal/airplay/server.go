package airplay

import (
	"encoding/binary"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// Feature bits per AirPlay spec
const (
	FeatureVideo                  uint64 = 1 << 0
	FeaturePhoto                  uint64 = 1 << 1
	FeatureVideoFairPlay          uint64 = 1 << 2
	FeatureVideoVolumeControl     uint64 = 1 << 3
	FeatureVideoHTTPLiveStreams   uint64 = 1 << 4
	FeatureSlideshow              uint64 = 1 << 5
	FeatureScreen                 uint64 = 1 << 7
	FeatureScreenRotate           uint64 = 1 << 8
	FeatureAudio                  uint64 = 1 << 9
	FeatureAudioRedundant         uint64 = 1 << 11
	FeaturePhotoCaching           uint64 = 1 << 13
	FeatureMetadataText           uint64 = 1 << 17
	FeatureMetadataArtwork        uint64 = 1 << 15
	FeatureMetadataProgress       uint64 = 1 << 16
	FeatureLegacyPairing          uint64 = 1 << 27
	FeatureRAOP                   uint64 = 1 << 30
	FeatureTransientPairing       uint64 = 1 << 48
	FeatureHKPairingAccessControl uint64 = 1 << 46
	FeatureSystemPairing          uint64 = 1 << 43
)

// StatusFlag bits
const (
	StatusProblemDetected uint32 = 1 << 0
	StatusNotConfigured   uint32 = 1 << 1
	StatusAudioCableAttached uint32 = 1 << 2
	StatusPINRequired     uint32 = 1 << 3
)

type ServerConfig struct {
	Name       string
	DeviceID   string // MAC address format
	Model      string
	SrcVersion string
	Features   uint64
	StatusFlags uint32
	Port       int
	MirrorPort int
	AirTunesPort int
	Width      int
	Height     int
	PIN        string
}

func DefaultConfig() ServerConfig {
	return ServerConfig{
		Name:         "AirPlay Server",
		DeviceID:     generateMACAddress(),
		Model:        "AppleTV6,2",
		SrcVersion:   "380.20.1",
		Features:     FeatureVideo | FeaturePhoto | FeatureVideoVolumeControl | FeatureVideoHTTPLiveStreams | FeatureSlideshow | FeatureScreen | FeatureScreenRotate | FeatureAudio | FeatureAudioRedundant | FeaturePhotoCaching | FeatureMetadataText | FeatureMetadataArtwork | FeatureMetadataProgress | FeatureLegacyPairing | FeatureRAOP,
		StatusFlags:  0x10644,
		Port:         7000,
		MirrorPort:   7100,
		AirTunesPort: 5000,
		Width:        1920,
		Height:       1080,
		PIN:          "3939",
	}
}

func generateMACAddress() string {
	b := make([]byte, 6)
	b[0] = 0xAA
	b[1] = 0xBB
	b[2] = 0xCC
	t := time.Now().UnixNano()
	binary.BigEndian.PutUint32(b[2:], uint32(t))
	b[0] = 0xAA
	b[1] = 0xBB
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[0], b[1], b[2], b[3], b[4], b[5])
}

type PlaybackState struct {
	mu       sync.RWMutex
	Playing  bool
	URL      string
	Position float64
	Duration float64
	Rate     float64
}

type Server struct {
	Config    ServerConfig
	Playback  *PlaybackState
	PhotoData []byte
	PhotoMu   sync.RWMutex
	MirrorCh  chan []byte // H.264 stream data
	AudioCh   chan []byte // Audio data
	EventCh   chan Event  // Events to frontend via WebSocket
	PairState *PairingState
	StaticFS  fs.FS       // Embedded frontend files
	stopCh    chan struct{}
}

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type PairingState struct {
	mu       sync.RWMutex
	Paired   bool
	PeerID   string
	PIN      string
}

func NewServer(cfg ServerConfig) *Server {
	return &Server{
		Config:   cfg,
		Playback: &PlaybackState{Rate: 1.0},
		MirrorCh: make(chan []byte, 120),
		AudioCh:  make(chan []byte, 256),
		EventCh:  make(chan Event, 64),
		PairState: &PairingState{PIN: cfg.PIN},
		stopCh:   make(chan struct{}),
	}
}

func (s *Server) Start() error {
	log.Printf("Starting AirPlay server '%s' on ports %d (airplay), %d (mirror), %d (audio)",
		s.Config.Name, s.Config.Port, s.Config.MirrorPort, s.Config.AirTunesPort)

	errCh := make(chan error, 3)

	// Main AirPlay HTTP server (port 7000)
	go func() {
		mux := s.BuildAirPlayMux(s.StaticFS)
		addr := fmt.Sprintf(":%d", s.Config.Port)
		log.Printf("AirPlay HTTP server listening on %s", addr)
		errCh <- http.ListenAndServe(addr, mux)
	}()

	// Mirror server (port 7100)
	go func() {
		errCh <- s.startMirrorServer()
	}()

	// RTSP/Audio server (port 5000)
	go func() {
		errCh <- s.startRTSPServer()
	}()

	// NTP server for time sync (port 7010)
	go s.startNTPServer()

	return <-errCh
}

func (s *Server) Stop() {
	close(s.stopCh)
}

func (s *Server) startMirrorServer() error {
	addr := fmt.Sprintf(":%d", s.Config.MirrorPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("mirror server listen: %w", err)
	}
	defer listener.Close()
	log.Printf("Mirror server listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return nil
			default:
				log.Printf("Mirror accept error: %v", err)
				continue
			}
		}
		go s.handleMirrorConnection(conn)
	}
}

func (s *Server) startRTSPServer() error {
	addr := fmt.Sprintf(":%d", s.Config.AirTunesPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("RTSP server listen: %w", err)
	}
	defer listener.Close()
	log.Printf("RTSP/Audio server listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return nil
			default:
				log.Printf("RTSP accept error: %v", err)
				continue
			}
		}
		go s.handleRTSPConnection(conn)
	}
}

func (s *Server) startNTPServer() {
	addr := fmt.Sprintf(":%d", 7010)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Printf("NTP server error: %v", err)
		return
	}
	defer conn.Close()
	log.Printf("NTP time sync server listening on %s", addr)

	buf := make([]byte, 128)
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}
		if n >= 32 {
			// NTP response: copy origin timestamp, set receive/transmit
			resp := make([]byte, 32)
			resp[0] = 0x24 // version 4, server mode
			now := ntpTimestamp(time.Now())
			// Origin = client's transmit
			copy(resp[8:16], buf[24:32])
			// Receive timestamp
			binary.BigEndian.PutUint64(resp[16:24], now)
			// Transmit timestamp
			binary.BigEndian.PutUint64(resp[24:32], now)
			conn.WriteTo(resp, remoteAddr)
		}
	}
}

func ntpTimestamp(t time.Time) uint64 {
	// NTP epoch is Jan 1, 1900
	const ntpEpoch = 2208988800
	secs := uint64(t.Unix()) + ntpEpoch
	frac := uint64(t.Nanosecond()) * (1 << 32) / 1e9
	return (secs << 32) | frac
}

func (s *Server) EmitEvent(eventType string, data interface{}) {
	select {
	case s.EventCh <- Event{Type: eventType, Data: data}:
	default:
	}
}
