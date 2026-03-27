package mdns

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/grandcat/zeroconf"
)

type MDNSService struct {
	Name         string
	DeviceID     string
	Model        string
	Features     uint64
	SrcVersion   string
	StatusFlags  uint32
	Port         int
	AirTunesPort int
	airplayServer *zeroconf.Server
	raopServer    *zeroconf.Server
}

func NewMDNSService(name, deviceID, model, srcVersion string, features uint64, statusFlags uint32, port, airTunesPort int) *MDNSService {
	return &MDNSService{
		Name:         name,
		DeviceID:     deviceID,
		Model:        model,
		Features:     features,
		SrcVersion:   srcVersion,
		StatusFlags:  statusFlags,
		Port:         port,
		AirTunesPort: airTunesPort,
	}
}

func (m *MDNSService) Start() error {
	var err error

	// Register _airplay._tcp service
	featLow := fmt.Sprintf("0x%x", uint32(m.Features&0xFFFFFFFF))
	featHigh := fmt.Sprintf("0x%x", uint32(m.Features>>32))
	featStr := featLow
	if m.Features>>32 > 0 {
		featStr = featLow + "," + featHigh
	}

	airplayTXT := []string{
		"deviceid=" + m.DeviceID,
		"features=" + featStr,
		"model=" + m.Model,
		"srcvers=" + m.SrcVersion,
		fmt.Sprintf("flags=0x%x", m.StatusFlags),
		"protovers=1.1",
		"pw=false",
	}

	m.airplayServer, err = zeroconf.Register(
		m.Name,
		"_airplay._tcp",
		"local.",
		m.Port,
		airplayTXT,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register _airplay._tcp: %w", err)
	}
	log.Printf("Registered mDNS: %s._airplay._tcp on port %d", m.Name, m.Port)

	// Register _raop._tcp service
	macClean := strings.ReplaceAll(m.DeviceID, ":", "")
	raopName := fmt.Sprintf("%s@%s", macClean, m.Name)

	raopTXT := []string{
		"txtvers=1",
		"ch=2",
		"cn=0,1",
		"da=true",
		"et=0,3",
		"md=0,1,2",
		"pw=false",
		"sv=false",
		"sr=44100",
		"ss=16",
		"tp=UDP",
		"vn=3",
		"vs=" + m.SrcVersion,
		"am=" + m.Model,
		fmt.Sprintf("sf=0x%x", m.StatusFlags),
	}

	m.raopServer, err = zeroconf.Register(
		raopName,
		"_raop._tcp",
		"local.",
		m.AirTunesPort,
		raopTXT,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register _raop._tcp: %w", err)
	}
	log.Printf("Registered mDNS: %s._raop._tcp on port %d", raopName, m.AirTunesPort)

	return nil
}

func (m *MDNSService) Stop() {
	if m.airplayServer != nil {
		m.airplayServer.Shutdown()
	}
	if m.raopServer != nil {
		m.raopServer.Shutdown()
	}
	log.Printf("mDNS services stopped")
}

// GetLocalIP returns the first non-loopback IPv4 address
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}
