package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/airplay-win/server/internal/airplay"
	"github.com/airplay-win/server/internal/mdns"
)

func main() {
	name := flag.String("name", "AirPlay Server", "Server display name")
	port := flag.Int("port", 7000, "AirPlay HTTP port")
	mirrorPort := flag.Int("mirror-port", 7100, "Mirror port")
	audioPort := flag.Int("audio-port", 5000, "RTSP/Audio port")
	width := flag.Int("width", 1920, "Display width")
	height := flag.Int("height", 1080, "Display height")
	pin := flag.String("pin", "3939", "Pairing PIN")
	flag.Parse()

	cfg := airplay.DefaultConfig()
	cfg.Name = *name
	cfg.Port = *port
	cfg.MirrorPort = *mirrorPort
	cfg.AirTunesPort = *audioPort
	cfg.Width = *width
	cfg.Height = *height
	cfg.PIN = *pin

	server := airplay.NewServer(cfg)

	// Start mDNS discovery
	mdnsSvc := mdns.NewMDNSService(
		cfg.Name,
		cfg.DeviceID,
		cfg.Model,
		cfg.SrcVersion,
		airplay.GetPublicKeyHex(),
		cfg.Features,
		cfg.StatusFlags,
		cfg.Port,
		cfg.AirTunesPort,
	)

	if err := mdnsSvc.Start(); err != nil {
		log.Printf("Warning: mDNS registration failed: %v", err)
		log.Printf("The server will still work but won't be auto-discoverable")
	}

	localIP := mdns.GetLocalIP()
	log.Printf("=== AirPlay Server ===")
	log.Printf("Name:       %s", cfg.Name)
	log.Printf("Device ID:  %s", cfg.DeviceID)
	log.Printf("Local IP:   %s", localIP)
	log.Printf("AirPlay:    http://%s:%d", localIP, cfg.Port)
	log.Printf("Mirror:     %s:%d", localIP, cfg.MirrorPort)
	log.Printf("Audio:      %s:%d", localIP, cfg.AirTunesPort)
	log.Printf("Frontend:   http://localhost:3000")
	log.Printf("PIN:        %s", cfg.PIN)
	log.Printf("======================")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Printf("Shutting down...")
		mdnsSvc.Stop()
		server.Stop()
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
