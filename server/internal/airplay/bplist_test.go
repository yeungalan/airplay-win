package airplay

import (
	"testing"
)

func TestBPlistEncodeDecodeDict(t *testing.T) {
	original := map[string]interface{}{
		"name":    "test",
		"version": int64(2),
		"active":  true,
	}
	data, err := BPlistEncode(original)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if string(data[:8]) != "bplist00" {
		t.Fatal("missing bplist00 magic")
	}

	decoded, err := BPlistDecode(data)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	m, ok := decoded.(map[string]interface{})
	if !ok {
		t.Fatal("expected map")
	}
	if m["name"] != "test" {
		t.Errorf("name: got %v", m["name"])
	}
	if m["version"] != int64(2) {
		t.Errorf("version: got %v", m["version"])
	}
	if m["active"] != true {
		t.Errorf("active: got %v", m["active"])
	}
}

func TestBPlistEncodeDecodeArray(t *testing.T) {
	original := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}
	data, err := BPlistEncode(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := BPlistDecode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m := decoded.(map[string]interface{})
	arr, ok := m["items"].([]interface{})
	if !ok || len(arr) != 3 {
		t.Fatalf("expected 3-element array, got %v", m["items"])
	}
	if arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
		t.Errorf("array values: %v", arr)
	}
}

func TestBPlistEncodeDecodeNested(t *testing.T) {
	// Simulates AirPlay 2 SETUP streams response
	original := map[string]interface{}{
		"eventPort": int64(12345),
		"streams": []interface{}{
			map[string]interface{}{
				"type":     int64(96),
				"dataPort": int64(5001),
			},
		},
	}
	data, err := BPlistEncode(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := BPlistDecode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m := decoded.(map[string]interface{})
	if m["eventPort"] != int64(12345) {
		t.Errorf("eventPort: %v", m["eventPort"])
	}
	streams := m["streams"].([]interface{})
	s0 := streams[0].(map[string]interface{})
	if s0["type"] != int64(96) {
		t.Errorf("stream type: %v", s0["type"])
	}
}

func TestBPlistEncodeDecodeFloat(t *testing.T) {
	original := map[string]interface{}{
		"volume": float64(-20.5),
	}
	data, _ := BPlistEncode(original)
	decoded, _ := BPlistDecode(data)
	m := decoded.(map[string]interface{})
	if m["volume"] != float64(-20.5) {
		t.Errorf("volume: %v", m["volume"])
	}
}

func TestBPlistEncodeDecodeBool(t *testing.T) {
	original := map[string]interface{}{
		"on":  true,
		"off": false,
	}
	data, _ := BPlistEncode(original)
	decoded, _ := BPlistDecode(data)
	m := decoded.(map[string]interface{})
	if m["on"] != true {
		t.Errorf("on: %v", m["on"])
	}
	if m["off"] != false {
		t.Errorf("off: %v", m["off"])
	}
}

func TestBPlistDecodeXMLFallback(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>name</key><string>test</string><key>count</key><integer>42</integer></dict></plist>`
	decoded, err := BPlistDecode([]byte(xml))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m := decoded.(map[string]interface{})
	if m["name"] != "test" {
		t.Errorf("name: %v", m["name"])
	}
	if m["count"] != int64(42) {
		t.Errorf("count: %v", m["count"])
	}
}

func TestBPlistDecodeTooShort(t *testing.T) {
	_, err := BPlistDecode([]byte("short"))
	if err == nil {
		t.Error("expected error for short input")
	}
}

func TestBPlistDecodeUnknownFormat(t *testing.T) {
	_, err := BPlistDecode([]byte("XXXXXXXX_not_a_plist"))
	if err == nil {
		t.Error("expected error for unknown format")
	}
}
