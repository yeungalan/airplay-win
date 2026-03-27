package airplay

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

// Minimal binary plist decoder/encoder for AirPlay 2 protocol.
// Supports dict, array, string, integer, real, bool, data types.

// BPlist decodes Apple binary property list format.
func BPlistDecode(data []byte) (interface{}, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("bplist: too short")
	}
	magic := string(data[:8])
	if magic == "bplist00" {
		return decodeBplist00(data)
	}
	// Try XML plist fallback
	if bytes.HasPrefix(data, []byte("<?xml")) || bytes.HasPrefix(data, []byte("<plist")) {
		return decodeXMLPlist(data)
	}
	// Try text/parameters
	return nil, fmt.Errorf("bplist: unknown format: %.8s", data)
}

// BPlistEncode encodes a value as binary plist.
func BPlistEncode(v interface{}) ([]byte, error) {
	return encodeBplist00(v)
}

// --- binary plist 00 decoder ---

func decodeBplist00(data []byte) (interface{}, error) {
	if len(data) < 40 { // 8 header + 32 trailer
		return nil, fmt.Errorf("bplist: too short for trailer")
	}
	trailer := data[len(data)-32:]
	offsetSize := int(trailer[6])
	objRefSize := int(trailer[7])
	numObjects := int(binary.BigEndian.Uint64(trailer[8:16]))
	topObject := int(binary.BigEndian.Uint64(trailer[16:24]))
	offsetTableOffset := int(binary.BigEndian.Uint64(trailer[24:32]))

	if numObjects == 0 {
		return nil, fmt.Errorf("bplist: no objects")
	}

	// Read offset table
	offsets := make([]int, numObjects)
	for i := 0; i < numObjects; i++ {
		off := offsetTableOffset + i*offsetSize
		if off+offsetSize > len(data) {
			return nil, fmt.Errorf("bplist: offset table overflow")
		}
		offsets[i] = readSizedInt(data[off:off+offsetSize], offsetSize)
	}

	return readObject(data, offsets, objRefSize, topObject)
}

func readSizedInt(b []byte, size int) int {
	switch size {
	case 1:
		return int(b[0])
	case 2:
		return int(binary.BigEndian.Uint16(b))
	case 4:
		return int(binary.BigEndian.Uint32(b))
	case 8:
		return int(binary.BigEndian.Uint64(b))
	}
	return 0
}

func readObject(data []byte, offsets []int, refSize, idx int) (interface{}, error) {
	if idx >= len(offsets) {
		return nil, fmt.Errorf("bplist: invalid object index %d", idx)
	}
	off := offsets[idx]
	if off >= len(data) {
		return nil, fmt.Errorf("bplist: offset out of range")
	}
	marker := data[off]
	objType := marker >> 4
	objInfo := int(marker & 0x0f)

	switch objType {
	case 0x0: // singleton
		switch objInfo {
		case 0x0:
			return nil, nil // null
		case 0x8:
			return false, nil
		case 0x9:
			return true, nil
		}
	case 0x1: // int
		nbytes := 1 << objInfo
		return readBPInt(data[off+1:off+1+nbytes], nbytes), nil
	case 0x2: // real
		nbytes := 1 << objInfo
		if nbytes == 4 {
			bits := binary.BigEndian.Uint32(data[off+1 : off+5])
			return float64(math.Float32frombits(bits)), nil
		}
		bits := binary.BigEndian.Uint64(data[off+1 : off+9])
		return math.Float64frombits(bits), nil
	case 0x4: // data
		length, start := readBPLength(data, off, objInfo)
		return data[start : start+length], nil
	case 0x5: // ascii string
		length, start := readBPLength(data, off, objInfo)
		return string(data[start : start+length]), nil
	case 0x6: // unicode string
		length, start := readBPLength(data, off, objInfo)
		runes := make([]rune, length)
		for i := 0; i < length; i++ {
			runes[i] = rune(binary.BigEndian.Uint16(data[start+i*2 : start+i*2+2]))
		}
		return string(runes), nil
	case 0xa: // array
		count, start := readBPLength(data, off, objInfo)
		arr := make([]interface{}, count)
		for i := 0; i < count; i++ {
			ref := readSizedInt(data[start+i*refSize:start+i*refSize+refSize], refSize)
			v, err := readObject(data, offsets, refSize, ref)
			if err != nil {
				return nil, err
			}
			arr[i] = v
		}
		return arr, nil
	case 0xd: // dict
		count, start := readBPLength(data, off, objInfo)
		dict := make(map[string]interface{}, count)
		keysStart := start
		valsStart := start + count*refSize
		for i := 0; i < count; i++ {
			kRef := readSizedInt(data[keysStart+i*refSize:keysStart+i*refSize+refSize], refSize)
			vRef := readSizedInt(data[valsStart+i*refSize:valsStart+i*refSize+refSize], refSize)
			k, err := readObject(data, offsets, refSize, kRef)
			if err != nil {
				return nil, err
			}
			v, err := readObject(data, offsets, refSize, vRef)
			if err != nil {
				return nil, err
			}
			dict[fmt.Sprint(k)] = v
		}
		return dict, nil
	}
	return nil, fmt.Errorf("bplist: unsupported type 0x%x", objType)
}

func readBPInt(b []byte, n int) int64 {
	switch n {
	case 1:
		return int64(b[0])
	case 2:
		return int64(binary.BigEndian.Uint16(b))
	case 4:
		return int64(int32(binary.BigEndian.Uint32(b)))
	case 8:
		return int64(binary.BigEndian.Uint64(b))
	}
	return 0
}

func readBPLength(data []byte, off, info int) (int, int) {
	if info < 0x0f {
		return info, off + 1
	}
	// Extended length: next byte is 0x1N where N is log2(byte count)
	sizeMarker := data[off+1]
	sizeBytes := 1 << (sizeMarker & 0x0f)
	length := readSizedInt(data[off+2:off+2+sizeBytes], sizeBytes)
	return length, off + 2 + sizeBytes
}

// --- binary plist 00 encoder ---

type bpEncoder struct {
	objects []interface{}
	objMap  map[string]int // dedup key -> index
}

func encodeBplist00(root interface{}) ([]byte, error) {
	enc := &bpEncoder{objMap: make(map[string]int)}
	topIdx := enc.addObject(root)

	numObjects := len(enc.objects)
	refSize := 1
	if numObjects > 255 {
		refSize = 2
	}

	var buf bytes.Buffer
	buf.WriteString("bplist00")

	offsets := make([]int, numObjects)
	for i, obj := range enc.objects {
		offsets[i] = buf.Len()
		enc.writeObject(&buf, obj, refSize)
	}

	offsetTableOffset := buf.Len()
	offsetSize := 1
	if offsetTableOffset > 0xffff {
		offsetSize = 4
	} else if offsetTableOffset > 0xff {
		offsetSize = 2
	}
	for _, off := range offsets {
		writeSizedInt(&buf, off, offsetSize)
	}

	// Trailer: 6 unused bytes, offsetSize, refSize, numObjects(8), topObject(8), offsetTableOffset(8)
	buf.Write(make([]byte, 6))
	buf.WriteByte(byte(offsetSize))
	buf.WriteByte(byte(refSize))
	binary.Write(&buf, binary.BigEndian, uint64(numObjects))
	binary.Write(&buf, binary.BigEndian, uint64(topIdx))
	binary.Write(&buf, binary.BigEndian, uint64(offsetTableOffset))

	return buf.Bytes(), nil
}

func (enc *bpEncoder) addObject(v interface{}) int {
	switch val := v.(type) {
	case map[string]interface{}:
		idx := len(enc.objects)
		enc.objects = append(enc.objects, v)
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		keyIdxs := make([]int, len(keys))
		valIdxs := make([]int, len(keys))
		for i, k := range keys {
			keyIdxs[i] = enc.addObject(k)
			valIdxs[i] = enc.addObject(val[k])
		}
		enc.objects[idx] = bpDict{keyIdxs, valIdxs, len(keys)}
		return idx
	case []interface{}:
		idx := len(enc.objects)
		enc.objects = append(enc.objects, v)
		refs := make([]int, len(val))
		for i, item := range val {
			refs[i] = enc.addObject(item)
		}
		enc.objects[idx] = bpArray{refs}
		return idx
	default:
		key := fmt.Sprintf("%T:%v", v, v)
		if idx, ok := enc.objMap[key]; ok {
			return idx
		}
		idx := len(enc.objects)
		enc.objects = append(enc.objects, v)
		enc.objMap[key] = idx
		return idx
	}
}

type bpDict struct {
	keys, vals []int
	count      int
}
type bpArray struct {
	refs []int
}

func (enc *bpEncoder) writeObject(w *bytes.Buffer, obj interface{}, refSize int) {
	switch val := obj.(type) {
	case nil:
		w.WriteByte(0x00)
	case bool:
		if val {
			w.WriteByte(0x09)
		} else {
			w.WriteByte(0x08)
		}
	case int:
		writeInt(w, int64(val))
	case int64:
		writeInt(w, val)
	case float64:
		w.WriteByte(0x23) // 8-byte real
		binary.Write(w, binary.BigEndian, val)
	case string:
		writeBPLength(w, 0x50, len(val))
		w.WriteString(val)
	case []byte:
		writeBPLength(w, 0x40, len(val))
		w.Write(val)
	case bpDict:
		writeBPLength(w, 0xd0, val.count)
		for _, k := range val.keys {
			writeSizedInt(w, k, refSize)
		}
		for _, v := range val.vals {
			writeSizedInt(w, v, refSize)
		}
	case bpArray:
		writeBPLength(w, 0xa0, len(val.refs))
		for _, r := range val.refs {
			writeSizedInt(w, r, refSize)
		}
	}
}

func writeInt(w *bytes.Buffer, v int64) {
	if v >= 0 && v <= 0xff {
		w.WriteByte(0x10)
		w.WriteByte(byte(v))
	} else if v >= 0 && v <= 0xffff {
		w.WriteByte(0x11)
		binary.Write(w, binary.BigEndian, uint16(v))
	} else if v >= math.MinInt32 && v <= math.MaxInt32 {
		w.WriteByte(0x12)
		binary.Write(w, binary.BigEndian, int32(v))
	} else {
		w.WriteByte(0x13)
		binary.Write(w, binary.BigEndian, v)
	}
}

func writeBPLength(w *bytes.Buffer, marker byte, length int) {
	if length < 15 {
		w.WriteByte(marker | byte(length))
	} else {
		w.WriteByte(marker | 0x0f)
		if length <= 0xff {
			w.WriteByte(0x10)
			w.WriteByte(byte(length))
		} else if length <= 0xffff {
			w.WriteByte(0x11)
			binary.Write(w, binary.BigEndian, uint16(length))
		} else {
			w.WriteByte(0x12)
			binary.Write(w, binary.BigEndian, int32(length))
		}
	}
}

func writeSizedInt(w io.Writer, v, size int) {
	switch size {
	case 1:
		binary.Write(w, binary.BigEndian, uint8(v))
	case 2:
		binary.Write(w, binary.BigEndian, uint16(v))
	case 4:
		binary.Write(w, binary.BigEndian, uint32(v))
	}
}

// --- XML plist decoder (fallback) ---

func decodeXMLPlist(data []byte) (interface{}, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	// Skip to first <dict> or <array>
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			switch se.Name.Local {
			case "dict":
				return decodeXMLDict(dec)
			case "array":
				return decodeXMLArray(dec)
			}
		}
	}
}

func decodeXMLDict(dec *xml.Decoder) (map[string]interface{}, error) {
	dict := make(map[string]interface{})
	for {
		tok, err := dec.Token()
		if err != nil {
			return dict, nil
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name.Local == "dict" {
				return dict, nil
			}
		case xml.StartElement:
			if t.Name.Local == "key" {
				var key string
				dec.DecodeElement(&key, &t)
				val, _ := decodeXMLValue(dec)
				dict[key] = val
			}
		}
	}
}

func decodeXMLArray(dec *xml.Decoder) ([]interface{}, error) {
	var arr []interface{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return arr, nil
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name.Local == "array" {
				return arr, nil
			}
		case xml.StartElement:
			val, _ := decodeXMLValueStart(dec, t)
			arr = append(arr, val)
		}
	}
}

func decodeXMLValue(dec *xml.Decoder) (interface{}, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return decodeXMLValueStart(dec, se)
		}
	}
}

func decodeXMLValueStart(dec *xml.Decoder, se xml.StartElement) (interface{}, error) {
	switch se.Name.Local {
	case "dict":
		return decodeXMLDict(dec)
	case "array":
		return decodeXMLArray(dec)
	case "string":
		var s string
		dec.DecodeElement(&s, &se)
		return s, nil
	case "integer":
		var s string
		dec.DecodeElement(&s, &se)
		var v int64
		fmt.Sscanf(s, "%d", &v)
		return v, nil
	case "real":
		var s string
		dec.DecodeElement(&s, &se)
		var v float64
		fmt.Sscanf(s, "%f", &v)
		return v, nil
	case "true":
		dec.Skip()
		return true, nil
	case "false":
		dec.Skip()
		return false, nil
	case "data":
		var s string
		dec.DecodeElement(&s, &se)
		s = strings.TrimSpace(s)
		return []byte(s), nil
	default:
		dec.Skip()
		return nil, nil
	}
}
