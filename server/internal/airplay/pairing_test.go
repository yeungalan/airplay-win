package airplay

import (
	"testing"
)

func TestTLVEncodeDecode(t *testing.T) {
	original := map[byte][]byte{
		TLVState:     {0x01},
		TLVPublicKey: {0xAA, 0xBB, 0xCC, 0xDD},
	}

	encoded := tlvEncode(original)
	decoded := tlvDecode(encoded)

	if len(decoded[TLVState]) != 1 || decoded[TLVState][0] != 0x01 {
		t.Error("TLV state mismatch")
	}
	if len(decoded[TLVPublicKey]) != 4 {
		t.Errorf("TLV public key length mismatch: %d", len(decoded[TLVPublicKey]))
	}
}

func TestTLVLargeValue(t *testing.T) {
	// Test with value > 255 bytes (should be split into chunks)
	largeVal := make([]byte, 300)
	for i := range largeVal {
		largeVal[i] = byte(i % 256)
	}

	original := map[byte][]byte{
		TLVPublicKey: largeVal,
	}

	encoded := tlvEncode(original)
	decoded := tlvDecode(encoded)

	if len(decoded[TLVPublicKey]) != 300 {
		t.Errorf("expected 300 bytes, got %d", len(decoded[TLVPublicKey]))
	}
	for i := 0; i < 300; i++ {
		if decoded[TLVPublicKey][i] != byte(i%256) {
			t.Errorf("byte %d mismatch", i)
			break
		}
	}
}

func TestTLVEmpty(t *testing.T) {
	decoded := tlvDecode([]byte{})
	if len(decoded) != 0 {
		t.Error("expected empty map for empty input")
	}
}

func TestPairSetupM1(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// M1 request
	body := tlvEncode(map[byte][]byte{
		TLVState:  {0x01},
		TLVMethod: {0x00},
	})

	req := newPostRequest("/pair-setup", body)
	w := recordRequest(mux, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	resp := tlvDecode(w.Body.Bytes())
	if len(resp[TLVState]) == 0 || resp[TLVState][0] != 0x02 {
		t.Error("expected state M2")
	}
	if len(resp[TLVSalt]) == 0 {
		t.Error("expected salt in M2")
	}
	if len(resp[TLVPublicKey]) == 0 {
		t.Error("expected public key in M2")
	}
}

func TestPairSetupM3(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	body := tlvEncode(map[byte][]byte{
		TLVState:     {0x03},
		TLVPublicKey: make([]byte, 384),
		TLVProof:     make([]byte, 64),
	})

	req := newPostRequest("/pair-setup", body)
	w := recordRequest(mux, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	resp := tlvDecode(w.Body.Bytes())
	if len(resp[TLVState]) == 0 || resp[TLVState][0] != 0x04 {
		t.Error("expected state M4")
	}
}

func TestPairSetupM5(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	// Need to do M1 and M3 first to set up shared secret
	m1Body := tlvEncode(map[byte][]byte{TLVState: {0x01}})
	recordRequest(mux, newPostRequest("/pair-setup", m1Body))

	m3Body := tlvEncode(map[byte][]byte{TLVState: {0x03}, TLVProof: make([]byte, 64)})
	recordRequest(mux, newPostRequest("/pair-setup", m3Body))

	// M5
	body := tlvEncode(map[byte][]byte{
		TLVState:   {0x05},
		TLVEncData: make([]byte, 32),
	})

	req := newPostRequest("/pair-setup", body)
	w := recordRequest(mux, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	resp := tlvDecode(w.Body.Bytes())
	if len(resp[TLVState]) == 0 || resp[TLVState][0] != 0x06 {
		t.Error("expected state M6")
	}

	s.PairState.mu.RLock()
	if !s.PairState.Paired {
		t.Error("expected paired to be true after M5-M6")
	}
	s.PairState.mu.RUnlock()
}

func TestPairVerifyM1(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	clientPub := make([]byte, 32)
	body := tlvEncode(map[byte][]byte{
		TLVState:     {0x01},
		TLVPublicKey: clientPub,
	})

	req := newPostRequest("/pair-verify", body)
	w := recordRequest(mux, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	resp := tlvDecode(w.Body.Bytes())
	if len(resp[TLVState]) == 0 || resp[TLVState][0] != 0x02 {
		t.Error("expected state M2")
	}
	if len(resp[TLVPublicKey]) != 32 {
		t.Errorf("expected 32-byte public key, got %d", len(resp[TLVPublicKey]))
	}
}

func TestPairVerifyM3(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	body := tlvEncode(map[byte][]byte{
		TLVState:   {0x03},
		TLVEncData: make([]byte, 16),
	})

	req := newPostRequest("/pair-verify", body)
	w := recordRequest(mux, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	resp := tlvDecode(w.Body.Bytes())
	if len(resp[TLVState]) == 0 || resp[TLVState][0] != 0x04 {
		t.Error("expected state M4")
	}
}

func TestFPSetup(t *testing.T) {
	s := newTestServer()
	mux := s.buildAirPlayMux()

	req := newPostRequest("/fp-setup", []byte{0x46, 0x50, 0x4C, 0x59, 0x03, 0x01})
	w := recordRequest(mux, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.Bytes()
	if len(body) < 4 || body[0] != 'F' || body[1] != 'P' || body[2] != 'L' || body[3] != 'Y' {
		t.Error("expected FPLY response")
	}
}

func TestGetPublicKeyHex(t *testing.T) {
	pk := GetPublicKeyHex()
	if len(pk) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected 64 hex chars, got %d", len(pk))
	}
}

func TestDeriveKey(t *testing.T) {
	secret := []byte("test-secret-key-for-hkdf-derivation")
	key := deriveKey(secret, "Pair-Setup-Encrypt-Salt", "Pair-Setup-Encrypt-Info")
	if len(key) != 32 {
		t.Errorf("expected 32-byte key, got %d", len(key))
	}

	// Same inputs should produce same output
	key2 := deriveKey(secret, "Pair-Setup-Encrypt-Salt", "Pair-Setup-Encrypt-Info")
	for i := range key {
		if key[i] != key2[i] {
			t.Error("deterministic key derivation failed")
			break
		}
	}

	// Different salt should produce different key
	key3 := deriveKey(secret, "Different-Salt", "Pair-Setup-Encrypt-Info")
	same := true
	for i := range key {
		if key[i] != key3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different salts should produce different keys")
	}
}
