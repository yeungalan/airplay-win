package airplay

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func TestDeriveKeyLegacy(t *testing.T) {
	secret := []byte("test-shared-secret")
	key := deriveKeyLegacy(secret, "Pair-Verify-AES-Key")
	if len(key) != 16 {
		t.Fatalf("expected 16 bytes, got %d", len(key))
	}
	// Deterministic
	key2 := deriveKeyLegacy(secret, "Pair-Verify-AES-Key")
	for i := range key {
		if key[i] != key2[i] {
			t.Fatal("not deterministic")
		}
	}
	// Different info → different key
	iv := deriveKeyLegacy(secret, "Pair-Verify-AES-IV")
	same := true
	for i := range key {
		if key[i] != iv[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("key and iv should differ")
	}
}

func TestAesCTR128RoundTrip(t *testing.T) {
	key := make([]byte, 16)
	iv := make([]byte, 16)
	rand.Read(key)
	rand.Read(iv)

	plaintext := []byte("hello airplay legacy pair-verify signature data!!")
	encrypted := aesCTR128(key, iv, plaintext)

	// Encrypting again with same key/iv decrypts (CTR is symmetric)
	decrypted := aesCTR128(key, iv, encrypted)
	if string(decrypted) != string(plaintext) {
		t.Errorf("roundtrip failed: %q", decrypted)
	}
}

func TestLegacyPairVerifyM1Format(t *testing.T) {
	// Simulate what a client sends: {1,0,0,0} | ECDH_PK(32) | Ed25519_PK(32)
	var clientECDHPriv [32]byte
	rand.Read(clientECDHPriv[:])
	var clientECDHPub [32]byte
	curve25519.ScalarBaseMult(&clientECDHPub, &clientECDHPriv)

	clientEdPub, _, _ := ed25519.GenerateKey(rand.Reader)

	msg := make([]byte, 68)
	msg[0] = 1 // header byte
	copy(msg[4:36], clientECDHPub[:])
	copy(msg[36:68], clientEdPub)

	// Verify format detection
	if msg[0] != 1 || msg[1] != 0 || msg[2] != 0 || msg[3] != 0 {
		t.Error("M1 header should be {1,0,0,0}")
	}
	if len(msg) != 68 {
		t.Errorf("M1 should be 68 bytes, got %d", len(msg))
	}
}

func TestLegacyPairVerifyM3Format(t *testing.T) {
	// M3: {0,0,0,0} | encrypted_signature(64)
	msg := make([]byte, 68)
	// header is all zeros
	rand.Read(msg[4:]) // fake encrypted signature

	if msg[0] != 0 || msg[1] != 0 || msg[2] != 0 || msg[3] != 0 {
		t.Error("M3 header should be {0,0,0,0}")
	}
}

func TestTransientPairSetupRoundTrip(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// Client generates Ed25519 keypair
	clientPub, _, _ := ed25519.GenerateKey(rand.Reader)

	req := newPostRequest("/pair-setup", clientPub)
	w := recordRequest(mux, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	serverPub := w.Body.Bytes()
	if len(serverPub) != 32 {
		t.Fatalf("expected 32-byte server pub, got %d", len(serverPub))
	}

	// Verify server stored client key
	globalPairSession.mu.Lock()
	stored := globalPairSession.peerPub
	globalPairSession.mu.Unlock()
	for i := range clientPub {
		if stored[i] != clientPub[i] {
			t.Fatal("server didn't store client public key")
		}
	}
}

func TestSRPVerifierDeterministic(t *testing.T) {
	salt := []byte("fixed-salt-16byt")
	v1 := srpVerifier(salt, "1234")
	v2 := srpVerifier(salt, "1234")
	if v1.Cmp(v2) != 0 {
		t.Error("same salt+pin should produce same verifier")
	}
	v3 := srpVerifier(salt, "5678")
	if v1.Cmp(v3) == 0 {
		t.Error("different pin should produce different verifier")
	}
}

func TestSRPServerKeysNonZero(t *testing.T) {
	salt := srpNewSalt()
	v := srpVerifier(salt, "3939")
	bPriv, bPub := srpServerKeys(v)
	if bPriv.Sign() == 0 {
		t.Error("bPriv should be non-zero")
	}
	if bPub.Sign() == 0 {
		t.Error("bPub should be non-zero")
	}
}
