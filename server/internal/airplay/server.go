package airplay

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// Feature bits per AirPlay spec (openairplay.github.io/airplay-spec/features.html)
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
	FeatureAuthentication4        uint64 = 1 << 14
	FeatureMetadataArtwork        uint64 = 1 << 15
	FeatureMetadataProgress       uint64 = 1 << 16
	FeatureMetadataText           uint64 = 1 << 17
	FeaturePhotoCaching           uint64 = 1 << 13
	FeatureAudioFormat1           uint64 = 1 << 18
	FeatureAudioFormat2           uint64 = 1 << 19 // required for AirPlay 2
	FeatureAudioFormat3           uint64 = 1 << 20 // required for AirPlay 2
	FeatureAudioFormat4           uint64 = 1 << 21
	FeatureHasUnifiedAdvertiser   uint64 = 1 << 26
	FeatureLegacyPairing          uint64 = 1 << 27
	FeatureRAOP                   uint64 = 1 << 30
	FeatureSupportsVolume         uint64 = 1 << 32
	FeatureBufferedAudio          uint64 = 1 << 40 // AirPlay 2 buffered audio
	FeatureSupportsPTP            uint64 = 1 << 41 // AirPlay 2 PTP clock
	FeatureScreenMultiCodec       uint64 = 1 << 42
	FeatureSystemPairing          uint64 = 1 << 43
	FeatureHKPairingAccessControl uint64 = 1 << 46
	FeatureTransientPairing       uint64 = 1 << 48
	FeatureSupportsAirPlayVideoV2 uint64 = 1 << 49
)

// StatusFlag bits
const (
	StatusProblemDetected uint32 = 1 << 0
	StatusNotConfigured   uint32 = 1 << 1
	StatusAudioCableAttached uint32 = 1 << 2
	StatusPINRequired     uint32 = 1 << 3
)

type ServerConfig struct {
	Name         string
	DeviceID     string // MAC address format
	Model        string
	SrcVersion   string
	Features     uint64
	StatusFlags  uint32
	Port         int
	MirrorPort   int
	AirTunesPort int
	Width        int
	Height       int
	PIN          string
	UIPort       int
	EventPort    int // AirPlay 2 event data port (UDP)
	DataPort     int // AirPlay 2 buffered audio data port (UDP)
}

func DefaultConfig() ServerConfig {
	return ServerConfig{
		Name:         "AirPlay Server",
		DeviceID:     generateMACAddress(),
		Model:      "AppleTV3,2",
		SrcVersion: "220.68",
		// AirPlay 1 feature set: no HAP/homekit pairing bits so iOS connects directly
		Features: FeatureVideo | FeaturePhoto | FeatureVideoVolumeControl |
			FeatureVideoHTTPLiveStreams | FeatureSlideshow |
			FeatureScreen | FeatureScreenRotate |
			FeatureAudio | FeatureAudioRedundant |
			FeaturePhotoCaching |
			FeatureMetadataText | FeatureMetadataArtwork | FeatureMetadataProgress |
			FeatureAuthentication4 |
			FeatureAudioFormat1 |
			FeatureHasUnifiedAdvertiser |
			FeatureRAOP,
		StatusFlags:  0x4, // AudioCableAttached only
		Port:         7000,
		MirrorPort:   7100,
		AirTunesPort: 5000,
		Width:        1920,
		Height:       1080,
		PIN:          "",
		UIPort:       7777,
		EventPort:    0, // dynamically assigned
		DataPort:     0, // dynamically assigned
	}
}

// generateMACAddress returns a stable locally-administered MAC derived from the hostname.
func generateMACAddress() string {
	host, _ := os.Hostname()
	h := md5.Sum([]byte("airplay-win:" + host))
	// Set locally administered bit, clear multicast bit
	h[0] = (h[0] | 0x02) & 0xFE
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", h[0], h[1], h[2], h[3], h[4], h[5])
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
	StaticFS  fs.FS // Embedded frontend files
	stopCh    chan struct{}

	// AirPlay 2 session state
	sessionMu    sync.RWMutex
	eventPort    int // bound event data port
	dataPort     int // bound buffered audio data port
	activeStream string // "audio", "mirror", or ""

	// RAOP audio UDP sockets (AirPlay 1)
	raopDataConn    net.PacketConn // RTP audio data
	raopControlConn net.PacketConn // RTCP control
	raopTimingConn  net.PacketConn // NTP timing
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
	log.Printf("Starting AirPlay server '%s' on ports %d (airplay), %d (ui), %d (mirror), %d (audio)",
		s.Config.Name, s.Config.Port, s.Config.UIPort, s.Config.MirrorPort, s.Config.AirTunesPort)

	errCh := make(chan error, 4)

	// Main AirPlay HTTP server (port 7000) - protocol only
	go func() {
		mux := s.buildAirPlayMux()
		addr := fmt.Sprintf(":%d", s.Config.Port)
		log.Printf("AirPlay HTTP server listening on %s", addr)
		errCh <- http.ListenAndServe(addr, mux)
	}()

	// UI server (port 7777) - frontend + websocket + API
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/ws", s.handleWebSocket)
		mux.HandleFunc("/api/status", s.handleAPIStatus)
		mux.HandleFunc("/api/photo", s.handleAPIPhoto)
		if s.StaticFS != nil {
			fileServer := http.FileServer(http.FS(s.StaticFS))
			mux.Handle("/", fileServer)
		}
		addr := fmt.Sprintf(":%d", s.Config.UIPort)
		log.Printf("UI server listening on %s", addr)
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

	// RAOP audio UDP ports (AirPlay 1)
	s.startRAOPServers()

	// AirPlay 2: start event and data UDP listeners
	go s.startEventDataPort()
	go s.startBufferedDataPort()

	return <-errCh
}

func (s *Server) Stop() {
	close(s.stopCh)
}

// startRAOPServers opens three UDP sockets for AirPlay 1 audio streaming:
// data (RTP), control (RTCP), and timing (NTP-style clock sync).
func (s *Server) startRAOPServers() {
	var err error
	s.raopDataConn, err = net.ListenPacket("udp", ":0")
	if err != nil {
		log.Printf("RAOP data port error: %v", err)
	}
	s.raopControlConn, err = net.ListenPacket("udp", ":0")
	if err != nil {
		log.Printf("RAOP control port error: %v", err)
	}
	s.raopTimingConn, err = net.ListenPacket("udp", ":0")
	if err != nil {
		log.Printf("RAOP timing port error: %v", err)
	}
	log.Printf("RAOP UDP ports: data=%d control=%d timing=%d",
		s.raopDataPort(), s.raopControlPort(), s.raopTimingPort())

	go s.receiveRAOPData()
	go s.receiveRAOPTiming()
}

func (s *Server) raopDataPort() int {
	if s.raopDataConn == nil {
		return 0
	}
	return s.raopDataConn.LocalAddr().(*net.UDPAddr).Port
}

func (s *Server) raopControlPort() int {
	if s.raopControlConn == nil {
		return 0
	}
	return s.raopControlConn.LocalAddr().(*net.UDPAddr).Port
}

func (s *Server) raopTimingPort() int {
	if s.raopTimingConn == nil {
		return 0
	}
	return s.raopTimingConn.LocalAddr().(*net.UDPAddr).Port
}

// receiveRAOPData reads incoming RTP audio packets and forwards them to AudioCh.
func (s *Server) receiveRAOPData() {
	if s.raopDataConn == nil {
		return
	}
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		s.raopDataConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, _, err := s.raopDataConn.ReadFrom(buf)
		if err != nil {
			continue
		}
		if n > 12 { // RTP header is 12 bytes minimum
			select {
			case s.AudioCh <- append([]byte(nil), buf[12:n]...): // strip RTP header
			default:
			}
		}
	}
}

// receiveRAOPTiming responds to NTP-style timing packets from iOS.
func (s *Server) receiveRAOPTiming() {
	if s.raopTimingConn == nil {
		return
	}
	buf := make([]byte, 128)
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		s.raopTimingConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := s.raopTimingConn.ReadFrom(buf)
		if err != nil {
			continue
		}
		if n >= 32 {
			resp := make([]byte, 32)
			resp[0] = 0x80
			resp[1] = 0xD3 // timing response type
			copy(resp[2:4], buf[2:4])   // sequence
			now := ntpTimestamp(time.Now())
			copy(resp[8:16], buf[24:32])  // reference = client transmit
			binary.BigEndian.PutUint64(resp[16:24], now) // receive
			binary.BigEndian.PutUint64(resp[24:32], now) // transmit
			s.raopTimingConn.WriteTo(resp, addr)
		}
	}
}

// startEventDataPort opens a TCP listener for AirPlay 2 event channel. opens a TCP listener for AirPlay 2 event channel.
// Per protocol spec, the event channel is TCP and must be open for RTSP to proceed.
// The port is dynamically assigned and reported in SETUP responses.
func (s *Server) startEventDataPort() {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Printf("Event port error: %v", err)
		return
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	s.sessionMu.Lock()
	s.eventPort = port
	s.sessionMu.Unlock()
	log.Printf("AirPlay 2 event channel (TCP) listening on :%d", port)

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			log.Printf("Event channel connection from %s", c.RemoteAddr())
			buf := make([]byte, 4096)
			for {
				c.SetReadDeadline(time.Now().Add(30 * time.Second))
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				if n > 0 {
					log.Printf("Event channel: %d bytes", n)
				}
			}
		}(conn)
	}
}

// startBufferedDataPort opens a UDP listener for AirPlay 2 buffered audio data.
func (s *Server) startBufferedDataPort() {
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		log.Printf("Buffered data port error: %v", err)
		return
	}
	defer conn.Close()
	port := conn.LocalAddr().(*net.UDPAddr).Port
	s.sessionMu.Lock()
	s.dataPort = port
	s.sessionMu.Unlock()
	log.Printf("AirPlay 2 buffered audio data port listening on :%d", port)

	buf := make([]byte, 65536)
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}
		if n > 0 {
			select {
			case s.AudioCh <- append([]byte(nil), buf[:n]...):
			default:
			}
		}
	}
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
