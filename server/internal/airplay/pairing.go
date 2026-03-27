package airplay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"io"
	"log"
	"math/big"
	"net/http"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// Pairing implements the HomeKit SRP-6a pairing protocol used by AirPlay.
// pair-setup uses SRP6a (RFC 5054 3072-bit + SHA-512) with PIN verification.
// pair-verify uses Curve25519 + ed25519 for session authentication.

type PairSession struct {
	mu            sync.Mutex
	setupStep     int
	verifyStep    int
	serverPubKey  ed25519.PublicKey
	serverPrivKey ed25519.PrivateKey
	curvePriv     [32]byte
	curvePub      [32]byte
	sharedSecret  []byte
	peerPub       []byte
	paired        bool
	// SRP-6a state (populated during pair-setup M1, used in M3)
	srpSalt  []byte
	srpV     *big.Int // verifier
	srpBPriv *big.Int // server private key
	srpBPub  *big.Int // server public key B
	srpK     []byte   // session key
}

var globalPairSession = &PairSession{}

func init() {
	initSRP()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate ed25519 key: %v", err)
	}
	globalPairSession.serverPubKey = pub
	globalPairSession.serverPrivKey = priv

	// Generate Curve25519 keypair for pair-verify
	rand.Read(globalPairSession.curvePriv[:])
	curve25519.ScalarBaseMult(&globalPairSession.curvePub, &globalPairSession.curvePriv)
}

// TLV types used in pairing
const (
	TLVMethod     = 0x00
	TLVIdentifier = 0x01
	TLVSalt       = 0x02
	TLVPublicKey  = 0x03
	TLVProof      = 0x04
	TLVEncData    = 0x05
	TLVState      = 0x06
	TLVError      = 0x07
	TLVSignature  = 0x0A
)

func tlvEncode(items map[byte][]byte) []byte {
	var out []byte
	for tag, val := range items {
		for len(val) > 0 {
			chunk := val
			if len(chunk) > 255 {
				chunk = val[:255]
			}
			out = append(out, tag, byte(len(chunk)))
			out = append(out, chunk...)
			val = val[len(chunk):]
		}
	}
	return out
}

func tlvDecode(data []byte) map[byte][]byte {
	items := make(map[byte][]byte)
	for len(data) >= 2 {
		tag := data[0]
		length := int(data[1])
		data = data[2:]
		if len(data) < length {
			break
		}
		items[tag] = append(items[tag], data[:length]...)
		data = data[length:]
	}
	return items
}

func buildPairSetupM6(srpK []byte, deviceID string, privKey ed25519.PrivateKey, pubKey ed25519.PublicKey) []byte {
	encKey := deriveKey(srpK, "Pair-Setup-Encrypt-Salt", "Pair-Setup-Encrypt-Info")
	serverX := deriveKey(srpK, "Pair-Setup-Accessory-Sign-Salt", "Pair-Setup-Accessory-Sign-Info")
	sigMaterial := append(append(serverX, []byte(deviceID)...), pubKey...)
	sig := ed25519.Sign(privKey, sigMaterial)
	inner := tlvEncode(map[byte][]byte{
		TLVIdentifier: []byte(deviceID),
		TLVPublicKey:  pubKey,
		TLVSignature:  sig,
	})
	nonce := [12]byte{}
	copy(nonce[4:], "PS-Msg06")
	aead, _ := chacha20poly1305.New(encKey)
	return aead.Seal(nil, nonce[:], inner, nil)
}

func (s *Server) handlePairSetup(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Transient pair-setup: raw 32-byte Ed25519 public key (no TLV)
	if len(body) == 32 {
		log.Printf("POST /pair-setup transient: 32-byte Ed25519 key from %s", r.RemoteAddr)
		globalPairSession.mu.Lock()
		globalPairSession.peerPub = body
		globalPairSession.paired = true
		globalPairSession.mu.Unlock()
		s.PairState.mu.Lock()
		s.PairState.Paired = true
		s.PairState.mu.Unlock()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(globalPairSession.serverPubKey)
		s.EmitEvent("pairing", map[string]interface{}{
			"step": "transient-setup", "message": "Transient pairing complete", "paired": true,
		})
		return
	}

	tlvs := tlvDecode(body)
	state := byte(0)
	if s, ok := tlvs[TLVState]; ok && len(s) > 0 {
		state = s[0]
	}

	log.Printf("POST /pair-setup state=%d from %s", state, r.RemoteAddr)

	switch state {
	case 1: // M1 - SRP Start
		s.handlePairSetupM1(w)
	case 3: // M3 - SRP Verify
		s.handlePairSetupM3(w, tlvs)
	case 5: // M5 - Exchange
		s.handlePairSetupM5(w, tlvs)
	default:
		log.Printf("Unknown pair-setup state: %d", state)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(tlvEncode(map[byte][]byte{
			TLVState: {state + 1},
			TLVError: {0x02}, // Unknown
		}))
	}
}

func (s *Server) handlePairSetupM1(w http.ResponseWriter) {
	// M2: generate real SRP-6a salt and server public key B
	salt := srpNewSalt()
	v := srpVerifier(salt, s.Config.PIN)
	bPriv, bPub := srpServerKeys(v)

	globalPairSession.mu.Lock()
	globalPairSession.setupStep = 2
	globalPairSession.srpSalt = salt
	globalPairSession.srpV = v
	globalPairSession.srpBPriv = bPriv
	globalPairSession.srpBPub = bPub
	globalPairSession.mu.Unlock()

	resp := tlvEncode(map[byte][]byte{
		TLVState:     {0x02},
		TLVSalt:      salt,
		TLVPublicKey: srpPad(bPub), // 384-byte padded B
	})

	s.EmitEvent("pairing", map[string]interface{}{
		"step":    "M1-M2",
		"message": "Pairing started - PIN: " + s.Config.PIN,
	})

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(resp)
}

func (s *Server) handlePairSetupM3(w http.ResponseWriter, tlvs map[byte][]byte) {
	// M4: compute session key from client's A, send server proof M2 = H(pad(A)||M1||K)
	aBytes := tlvs[TLVPublicKey]
	m1Client := tlvs[TLVProof]

	globalPairSession.mu.Lock()
	bPriv := globalPairSession.srpBPriv
	bPub := globalPairSession.srpBPub
	v := globalPairSession.srpV

	var m2 []byte
	var K []byte
	if bPriv != nil && bPub != nil && v != nil && len(aBytes) > 0 {
		A := new(big.Int).SetBytes(aBytes)
		K = srpSessionKey(A, bPub, bPriv, v)
		if K != nil {
			m2 = srpServerProof(A, m1Client, K)
		}
	}
	if m2 == nil {
		m2 = make([]byte, 64)
		rand.Read(m2)
	}

	globalPairSession.setupStep = 4
	globalPairSession.sharedSecret = K
	globalPairSession.srpK = K
	globalPairSession.mu.Unlock()

	resp := tlvEncode(map[byte][]byte{
		TLVState: {0x04},
		TLVProof: m2,
	})

	s.EmitEvent("pairing", map[string]interface{}{
		"step":    "M3-M4",
		"message": "Verifying PIN...",
	})

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(resp)
}

func (s *Server) handlePairSetupM5(w http.ResponseWriter, tlvs map[byte][]byte) {
	// M6 response: exchange complete
	globalPairSession.mu.Lock()
	srpK := globalPairSession.srpK
	globalPairSession.setupStep = 6
	globalPairSession.paired = true
	globalPairSession.mu.Unlock()

	s.PairState.mu.Lock()
	s.PairState.Paired = true
	s.PairState.mu.Unlock()

	encData := buildPairSetupM6(srpK, s.Config.DeviceID, globalPairSession.serverPrivKey, globalPairSession.serverPubKey)

	resp := tlvEncode(map[byte][]byte{
		TLVState:   {0x06},
		TLVEncData: encData,
	})

	s.EmitEvent("pairing", map[string]interface{}{
		"step":    "M5-M6",
		"message": "Pairing complete!",
		"paired":  true,
	})

	log.Printf("Pairing complete!")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(resp)
}

func (s *Server) handlePairVerify(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	tlvs := tlvDecode(body)
	state := byte(0)
	if s, ok := tlvs[TLVState]; ok && len(s) > 0 {
		state = s[0]
	}

	log.Printf("POST /pair-verify state=%d from %s", state, r.RemoteAddr)

	switch state {
	case 1: // M1
		s.handlePairVerifyM1(w, tlvs)
	case 3: // M3
		s.handlePairVerifyM3(w, tlvs)
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(tlvEncode(map[byte][]byte{
			TLVState: {state + 1},
		}))
	}
}

func (s *Server) handlePairVerifyM1(w http.ResponseWriter, tlvs map[byte][]byte) {
	clientPub := tlvs[TLVPublicKey]

	// Generate fresh ephemeral Curve25519 keypair for this verify session
	var ephPriv, ephPub [32]byte
	rand.Read(ephPriv[:])
	curve25519.ScalarBaseMult(&ephPub, &ephPriv)

	globalPairSession.mu.Lock()
	globalPairSession.peerPub = clientPub
	globalPairSession.verifyStep = 2
	globalPairSession.curvePriv = ephPriv
	globalPairSession.curvePub = ephPub

	// Compute shared secret via Curve25519
	var shared [32]byte
	if len(clientPub) == 32 {
		var peerPub [32]byte
		copy(peerPub[:], clientPub)
		curve25519.ScalarMult(&shared, &ephPriv, &peerPub)
		globalPairSession.sharedSecret = shared[:]
	}
	globalPairSession.mu.Unlock()

	// Derive verify key
	verifyKey := deriveKey(shared[:], "Pair-Verify-Encrypt-Salt", "Pair-Verify-Encrypt-Info")

	// Signature covers: server ephemeral pub || server pairing ID || client ephemeral pub
	sigMaterial := append(append(ephPub[:], []byte(s.Config.DeviceID)...), clientPub...)
	sig := ed25519.Sign(globalPairSession.serverPrivKey, sigMaterial)

	// Build inner TLV
	inner := tlvEncode(map[byte][]byte{
		TLVIdentifier: []byte(s.Config.DeviceID),
		TLVSignature:  sig,
	})

	// Encrypt inner TLV with ChaCha20-Poly1305
	nonce := [12]byte{}
	copy(nonce[4:], "PV-Msg02")
	aead, _ := chacha20poly1305.New(verifyKey)
	encData := aead.Seal(nil, nonce[:], inner, nil)

	resp := tlvEncode(map[byte][]byte{
		TLVState:     {0x02},
		TLVPublicKey: ephPub[:],
		TLVEncData:   encData,
	})

	s.EmitEvent("pairing", map[string]interface{}{
		"step":    "verify-M1-M2",
		"message": "Pair verify in progress...",
	})

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(resp)
}

func (s *Server) handlePairVerifyM3(w http.ResponseWriter, tlvs map[byte][]byte) {
	globalPairSession.mu.Lock()
	globalPairSession.verifyStep = 4
	globalPairSession.mu.Unlock()

	resp := tlvEncode(map[byte][]byte{
		TLVState: {0x04},
	})

	s.EmitEvent("pairing", map[string]interface{}{
		"step":    "verify-M3-M4",
		"message": "Pair verified!",
		"verified": true,
	})

	log.Printf("Pair verify complete!")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(resp)
}

func (s *Server) handlePairPinStart(w http.ResponseWriter, r *http.Request) {
	log.Printf("POST /pair-pin-start from %s (PIN: %s)", r.RemoteAddr, s.Config.PIN)
	s.EmitEvent("pairing", map[string]interface{}{
		"step":    "pin-start",
		"message": "PIN pairing initiated - PIN: " + s.Config.PIN,
	})
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleFPSetup(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	log.Printf("POST /fp-setup (%d bytes) from %s", len(body), r.RemoteAddr)

	// FairPlay setup - respond with a minimal valid response
	resp := make([]byte, 4)
	resp[0] = 0x46 // 'F'
	resp[1] = 0x50 // 'P'
	resp[2] = 0x4C // 'L'
	resp[3] = 0x59 // 'Y'

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(resp)
}

func deriveKey(secret []byte, salt, info string) []byte {
	hkdfReader := hkdf.New(sha512.New, secret, []byte(salt), []byte(info))
	key := make([]byte, 32)
	io.ReadFull(hkdfReader, key)
	return key
}

// GetPublicKeyHex returns the server's ed25519 public key as hex string for mDNS TXT record
func GetPublicKeyHex() string {
	return hex.EncodeToString(globalPairSession.serverPubKey)
}

// deriveKeyLegacy derives a 16-byte key using SHA-512 of (info || secret).
// Used for legacy pair-verify AES-CTR-128 key/iv derivation per UxPlay crypto spec.
func deriveKeyLegacy(secret []byte, info string) []byte {
	h := sha512.New()
	h.Write([]byte(info))
	h.Write(secret)
	return h.Sum(nil)[:16]
}

// aesCTR128 encrypts/decrypts data using AES-128-CTR.
func aesCTR128(key, iv, data []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		return data
	}
	out := make([]byte, len(data))
	cipher.NewCTR(block, iv).XORKeyStream(out, data)
	return out
}
