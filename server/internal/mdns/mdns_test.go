package mdns

import (
	"strings"
	"testing"
)

func TestGetLocalIP(t *testing.T) {
	ip := GetLocalIP()
	if ip == "" {
		t.Error("expected non-empty IP")
	}
	// Should be a valid IPv4 address
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		t.Errorf("expected IPv4 format, got %s", ip)
	}
}

func TestNewMDNSService(t *testing.T) {
	svc := NewMDNSService(
		"Test AirPlay",
		"AA:BB:CC:DD:EE:FF",
		"AppleTV6,2",
		"380.20.1",
		"abcdef1234567890",
		0x39f7,
		0x10644,
		7000,
		5000,
	)

	if svc.Name != "Test AirPlay" {
		t.Errorf("expected name 'Test AirPlay', got '%s'", svc.Name)
	}
	if svc.DeviceID != "AA:BB:CC:DD:EE:FF" {
		t.Error("device ID mismatch")
	}
	if svc.Port != 7000 {
		t.Errorf("expected port 7000, got %d", svc.Port)
	}
	if svc.AirTunesPort != 5000 {
		t.Errorf("expected audio port 5000, got %d", svc.AirTunesPort)
	}
}
