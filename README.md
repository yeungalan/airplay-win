# AirPlay Server

An AirPlay receiver implementation with a Go backend and Next.js frontend display.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   iOS / macOS Device                 │
│                  (AirPlay Sender)                    │
└──────────┬──────────┬──────────┬───────────┬────────┘
           │          │          │           │
     mDNS Discovery  HTTP     RTSP      TCP Mirror
     (_airplay._tcp) (7000)   (5000)     (7100)
     (_raop._tcp)
           │          │          │           │
┌──────────▼──────────▼──────────▼───────────▼────────┐
│                   Go Backend                         │
│                                                      │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐ │
│  │  mDNS    │ │ AirPlay  │ │  RTSP    │ │ Mirror │ │
│  │ Service  │ │ HTTP     │ │ Audio    │ │ Server │ │
│  │ Zeroconf │ │ Server   │ │ Server   │ │ H.264  │ │
│  └──────────┘ └──────────┘ └──────────┘ └────────┘ │
│                                                      │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐            │
│  │ Pairing  │ │   NTP    │ │WebSocket │            │
│  │ Setup/   │ │  Sync    │ │  Events  │            │
│  │ Verify   │ │ (7010)   │ │  to UI   │            │
│  └──────────┘ └──────────┘ └──────────┘            │
└──────────────────────┬──────────────────────────────┘
                       │ WebSocket (ws://localhost:7000/api/ws)
                       │
┌──────────────────────▼──────────────────────────────┐
│              Next.js Frontend (3000)                  │
│                                                      │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐            │
│  │  Video   │ │  Photo   │ │  Mirror  │            │
│  │  Player  │ │  Viewer  │ │  View    │            │
│  └──────────┘ └──────────┘ └──────────┘            │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐            │
│  │  Audio   │ │ Pairing  │ │  Event   │            │
│  │  Player  │ │  Panel   │ │   Log    │            │
│  └──────────┘ └──────────┘ └──────────┘            │
└─────────────────────────────────────────────────────┘
```

## Features

- **mDNS Discovery**: Registers `_airplay._tcp` and `_raop._tcp` services via Zeroconf/Bonjour
- **Photos**: Receives JPEG images via `PUT /photo`, supports slideshows and transitions
- **Video**: Handles `POST /play`, `/scrub`, `/rate`, `/stop` for URL-based video playback
- **Audio**: RTSP server for audio streaming (ANNOUNCE, SETUP, RECORD, TEARDOWN)
- **Screen Mirroring**: TCP server on port 7100 for H.264 stream with 128-byte packet headers
- **Pairing**: HomeKit-based pair-setup and pair-verify with TLV encoding, ed25519 + Curve25519 + HKDF-SHA512
- **NTP Time Sync**: UDP server on port 7010 for audio/video synchronization
- **Real-time UI**: WebSocket connection pushes all events to the Next.js frontend

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 7000 | HTTP | AirPlay commands + API |
| 7100 | TCP | Screen mirroring |
| 5000 | RTSP | Audio streaming |
| 7010 | UDP | NTP time sync |
| 3000 | HTTP | Next.js frontend |

## Quick Start

```bash
./start.sh
```

Or manually:

```bash
# Terminal 1: Backend
cd server
go run ./cmd/ --name "My AirPlay" --pin 1234

# Terminal 2: Frontend
cd frontend
npm run dev
```

Open http://localhost:3000 to see the display.

## AirPlay Protocol Endpoints

### Main HTTP Server (port 7000)
- `GET /server-info` - Server capabilities (XML plist)
- `GET /info` - Extended server info (JSON)
- `POST /play` - Start video playback
- `POST /scrub` - Seek to position
- `GET /scrub` - Get current position
- `POST /rate` - Set playback rate (0=pause, 1=play)
- `POST /stop` - Stop playback
- `PUT /photo` - Receive photo (JPEG)
- `GET /slideshow-features` - Available transitions
- `PUT /slideshows/1` - Start/stop slideshow
- `GET /playback-info` - Full playback status (XML plist)
- `POST /pair-setup` - Pairing setup (TLV)
- `POST /pair-verify` - Pairing verification (TLV)
- `POST /fp-setup` - FairPlay setup
- `POST /feedback` - Client feedback
- `POST /command` - Remote commands

### Mirror Server (port 7100)
- `GET /stream.xml` - Display capabilities
- `POST /stream` - Start H.264 mirror stream

### RTSP/Audio Server (port 5000)
- `OPTIONS` - Supported methods
- `GET /info` - Audio server info
- `ANNOUNCE` - Audio session SDP
- `SETUP` - Configure audio transport
- `RECORD` - Start audio streaming
- `SET_PARAMETER` - Volume, metadata, artwork
- `FLUSH` - Flush audio buffer
- `TEARDOWN` - End audio session
- `POST /pair-setup` - Audio pairing
- `POST /pair-verify` - Audio pair verify

## Testing

```bash
# Backend tests (37 tests)
cd server && go test ./... -v

# Frontend tests (20 tests)
cd frontend && npx jest
```

## Configuration

```bash
./bin/airplay-server \
  --name "Living Room TV" \
  --port 7000 \
  --mirror-port 7100 \
  --audio-port 5000 \
  --width 1920 \
  --height 1080 \
  --pin 3939
```

## Tech Stack

- **Backend**: Go 1.21+ with `grandcat/zeroconf` for mDNS, `golang.org/x/crypto` for ed25519/Curve25519/HKDF
- **Frontend**: Next.js 16 + TypeScript + Tailwind CSS
- **Communication**: WebSocket for real-time event streaming from backend to frontend
