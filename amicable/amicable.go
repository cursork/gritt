// Package amicable implements Dyalog APL's 220⌶ binary array serialization format.
// Named after 220, the first amicable number.
//
// The format encodes APL arrays as a vector of signed bytes (-128 to 127).
// It preserves exact types, shapes, and nesting. Often paired with 219⌶
// (compression) for wire transport.
//
// Usage:
//
//	// Deserialize bytes from Dyalog
//	val, err := amicable.Unmarshal(data)
//
//	// Serialize Go values for Dyalog
//	data, err := amicable.Marshal(val)
//
// Go type mapping matches the codec package:
//   - int, float64, complex128 for numeric scalars
//   - string for character vectors and scalars
//   - []any for nested vectors
//   - *codec.Array for shaped arrays (rank ≥ 2, or any with explicit shape)
//   - bool for boolean scalars
package amicable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"github.com/cursork/gritt/codec"
)

// Architecture constants derived from the magic byte.
const (
	magicByte0 = 0xDF
	magic32    = 0x94 // 32-bit interpreter
	magic64    = 0xA4 // 64-bit interpreter

	ptrSize32 = 4
	ptrSize64 = 8
)

// Internal type codes used in the serialized format.
const (
	typeBool    = 0x21 // 1-bit boolean
	typeInt8    = 0x22 // 8-bit signed integer
	typeInt16   = 0x23 // 16-bit signed integer
	typeInt32   = 0x24 // 32-bit signed integer
	typeFloat64 = 0x25 // 64-bit IEEE 754 float
	typeChar8   = 0x27 // 8-bit character
	typeChar16  = 0x28 // 16-bit character
	typeChar32  = 0x29 // 32-bit character (Unicode)
	typeComplex = 0x2A // 128-bit complex (two float64)
	typeDec128  = 0x2E // 128-bit decimal float
	typePointer = 0x06 // nested/boxed array
)

// Flags in the low nibble of the rank/flags byte.
const (
	flagSimple = 0x0F
	flagNested = 0x07
)

// Unmarshal deserializes a 220⌶ byte vector into Go values.
// The input should be the raw bytes (converted from signed to unsigned).
func Unmarshal(data []byte) (any, error) {
	if len(data) < 2 {
		return nil, errors.New("amicable: data too short for magic")
	}
	if data[0] != magicByte0 {
		return nil, fmt.Errorf("amicable: invalid magic byte 0: 0x%02X", data[0])
	}
	var ptrSize int
	switch data[1] {
	case magic64:
		ptrSize = ptrSize64
	case magic32:
		ptrSize = ptrSize32
	default:
		return nil, fmt.Errorf("amicable: unknown architecture magic: 0x%02X", data[1])
	}
	// Peek at the type code to detect opaque internal types (⎕OR, namespaces).
	// Type code is at offset: 2 (magic) + ptrSize (size field) + 1 (rank/flags byte).
	typeOff := 2 + ptrSize + 1
	if typeOff < len(data) && data[typeOff] == 0x00 {
		raw := make(Raw, len(data))
		copy(raw, data)
		// Namespaces (byte 0x22 high nibble >= 0xA0) have parseable structure.
		// Parse into *codec.Namespace. Functions/⎕OR stay as Raw.
		if len(data) > 0x22 && data[0x22]&0xF0 >= 0xA0 {
			return raw.unmarshalNamespace()
		}
		return raw, nil
	}

	r := &reader{data: data, pos: 2, ptrSize: ptrSize}
	return r.readArray()
}

// unmarshalNamespace parses a Raw namespace blob into *codec.Namespace.
// Extracts member values directly from the blob as typed Go values.
//
// Blob layout after the name table: member values in reverse name-table order,
// then metadata tail (settings, translation table, workspace info).
func (r Raw) unmarshalNamespace() (any, error) {
	data := []byte(r)
	members := r.extractNsMembers()
	nameTableEnd := r.findNameTableEnd()

	// Separate namespace name from value members.
	var memberList []nsMember
	for _, m := range members {
		if m.class != 9 {
			memberList = append(memberList, m)
		}
	}

	// Values are stored in reverse name-table order.
	reversed := make([]nsMember, len(memberList))
	for i, m := range memberList {
		reversed[len(memberList)-1-i] = m
	}

	// Walk sequentially from nameTableEnd, collecting one value per member.
	values := make([]any, 0, len(memberList))
	pos := nameTableEnd
	for _, m := range reversed {
		switch m.class {
		case 3: // function — embedded sub-blob, different format from standalone ⎕OR
			fnStart := pos
			newPos, ok := skipFnBlob(data, pos)
			if !ok {
				goto done
			}
			// Store the raw embedded bytes for future use.
			blob := make(Raw, newPos-fnStart)
			copy(blob, data[fnStart:newPos])
			values = append(values, blob)
			pos = newPos
		default: // variable — standard sub-array
			val, newPos, ok := findNextSubArray(data, pos)
			if !ok {
				goto done
			}
			values = append(values, val)
			pos = newPos
		}
	}
done:

	// Reverse back to name-table order.
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}

	ns := &codec.Namespace{
		Keys:   make([]string, 0, len(memberList)),
		Values: make(map[string]any, len(memberList)),
	}
	for i, m := range memberList {
		ns.Keys = append(ns.Keys, m.name)
		if i < len(values) {
			ns.Values[m.name] = values[i]
		}
	}
	return ns, nil
}

// findNextSubArray scans forward from 'from' for the next standard sub-array
// (simple or nested) and parses it. Returns (value, posAfter, true) on success.
func findNextSubArray(data []byte, from int) (any, int, bool) {
	for j := from; j < len(data)-17; j++ {
		rf := data[j+8]
		tc := data[j+9]
		flags := rf & 0x0F
		rank := int(rf >> 4)

		// Accept simple (0x0F) and nested (0x07) sub-arrays.
		if flags != flagSimple && flags != flagNested {
			continue
		}
		if rank > 15 {
			continue
		}
		// Validate type code for simple arrays.
		if flags == flagSimple && !validTypeCode(tc) {
			continue
		}
		// Nested must be typePointer (0x06).
		if flags == flagNested && tc != typePointer {
			continue
		}
		// Zero padding after type/rank.
		allZero := true
		for _, b := range data[j+10 : min(j+16, len(data))] {
			if b != 0 {
				allZero = false
			}
		}
		if !allZero {
			continue
		}

		subR := &reader{data: data, pos: j, ptrSize: 8}
		val, err := subR.readArray()
		if err != nil {
			continue
		}

		// Skip bytecode char8 vectors (FF FF header — these are inside function blobs).
		if tc == typeChar8 && rank == 1 {
			if s, ok := val.(string); ok && len(s) >= 2 && s[0] == 0xFF && s[1] == 0xFF {
				j = subR.pos - 1
				continue
			}
		}

		return val, subR.pos, true
	}
	return nil, from, false
}

// skipFnBlob scans forward from 'from' past an embedded function sub-blob.
// Identifies the blob by the FF FF bytecode marker, parses the bytecode vector
// to find its end, then skips past any trailing sub-arrays (literal pool).
// Returns the position after the function blob.
//
// Embedded functions use a different internal encoding than standalone ⎕OR blobs
// so we don't attempt to extract them as Raw. The caller stores the raw bytes
// for future use.
func skipFnBlob(data []byte, from int) (int, bool) {
	// Find the FF FF bytecode marker.
	bcStart := -1
	for j := from; j < len(data)-1; j++ {
		if data[j] == 0xFF && data[j+1] == 0xFF {
			bcStart = j
			break
		}
	}
	if bcStart < 0 {
		return from, false
	}

	// Find the char8 vector containing the bytecode by scanning backwards
	// for 0x1F 0x27 (rank=1, flags=0x0F, type=char8).
	vecStart := -1
	for j := bcStart; j >= from && j >= bcStart-40; j-- {
		if j+9 < len(data) && data[j+8] == 0x1F && data[j+9] == typeChar8 {
			vecStart = j
			break
		}
	}
	if vecStart < 0 {
		return bcStart + 2, false
	}

	// Parse the bytecode vector to find its end.
	subR := &reader{data: data, pos: vecStart, ptrSize: 8}
	_, err := subR.readArray()
	if err != nil {
		return bcStart + 2, false
	}
	end := subR.pos

	// Skip past trailing sub-arrays (literal pool + footer values).
	for {
		_, newPos, ok := findNextSubArray(data, end)
		if !ok || newPos-end > 64 {
			break
		}
		end = newPos
	}

	return end, true
}

func validTypeCode(tc byte) bool {
	switch tc {
	case typeBool, typeInt8, typeInt16, typeInt32, typeFloat64,
		typeChar8, typeChar16, typeChar32, typeComplex, typeDec128:
		return true
	}
	return false
}

// Marshal serializes a Go value into 220⌶ format (64-bit little-endian).
// Raw values are returned as-is (they already contain the full serialized form).
func Marshal(v any) ([]byte, error) {
	if raw, ok := v.(Raw); ok {
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, nil
	}
	w := &writer{ptrSize: ptrSize64}
	w.writeByte(magicByte0)
	w.writeByte(magic64)
	if err := w.writeArray(v); err != nil {
		return nil, err
	}
	return w.buf, nil
}

// SignedToBytes converts a slice of signed APL integers (-128..127) to unsigned bytes.
func SignedToBytes(signed []int8) []byte {
	out := make([]byte, len(signed))
	for i, v := range signed {
		out[i] = byte(v)
	}
	return out
}

// BytesToSigned converts unsigned bytes to signed APL integers (-128..127).
func BytesToSigned(data []byte) []int8 {
	out := make([]int8, len(data))
	for i, v := range data {
		out[i] = int8(v)
	}
	return out
}

// --- Reader ---

type reader struct {
	data    []byte
	pos     int
	ptrSize int
}

func (r *reader) remaining() int { return len(r.data) - r.pos }

func (r *reader) readBytes(n int) ([]byte, error) {
	if r.remaining() < n {
		return nil, fmt.Errorf("amicable: need %d bytes at offset %d, have %d", n, r.pos, r.remaining())
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *reader) readPtr() (uint64, error) {
	b, err := r.readBytes(r.ptrSize)
	if err != nil {
		return 0, err
	}
	if r.ptrSize == 8 {
		return binary.LittleEndian.Uint64(b), nil
	}
	return uint64(binary.LittleEndian.Uint32(b)), nil
}

func (r *reader) readArray() (any, error) {
	// Size field
	size, err := r.readPtr()
	if err != nil {
		return nil, err
	}

	// Type/rank: 2 bytes + padding to ptrSize
	typeRankBytes, err := r.readBytes(r.ptrSize)
	if err != nil {
		return nil, err
	}
	rankFlags := typeRankBytes[0]
	typeCode := typeRankBytes[1]
	rank := int(rankFlags >> 4)
	isNested := (rankFlags & 0x08) == 0

	// Shape
	shape := make([]int, rank)
	totalElements := 1
	for i := range rank {
		dim, err := r.readPtr()
		if err != nil {
			return nil, err
		}
		shape[i] = int(dim)
		totalElements *= shape[i]
	}

	_ = size // size is used for skipping; we parse structurally

	if isNested {
		return r.readNested(rank, shape, totalElements)
	}
	return r.readSimple(typeCode, rank, shape, totalElements)
}

func (r *reader) readSimple(typeCode byte, rank int, shape []int, totalElements int) (any, error) {
	var dataBytes int
	switch typeCode {
	case typeBool:
		dataBytes = (totalElements + 7) / 8
	case typeInt8, typeChar8:
		dataBytes = totalElements
	case typeInt16, typeChar16:
		dataBytes = totalElements * 2
	case typeInt32, typeChar32:
		dataBytes = totalElements * 4
	case typeFloat64:
		dataBytes = totalElements * 8
	case typeComplex:
		dataBytes = totalElements * 16
	case typeDec128:
		dataBytes = totalElements * 16
	default:
		return nil, fmt.Errorf("amicable: unknown type code 0x%02X", typeCode)
	}

	// Empty arrays have no data section
	if totalElements == 0 {
		switch typeCode {
		case typeChar8, typeChar16, typeChar32:
			if rank <= 1 {
				return "", nil
			}
		}
		return wrapResult(nil, rank, shape), nil
	}

	// Read data padded to ptrSize
	padded := ((dataBytes + r.ptrSize - 1) / r.ptrSize) * r.ptrSize
	raw, err := r.readBytes(padded)
	if err != nil {
		return nil, err
	}

	// Decode elements
	switch typeCode {
	case typeBool:
		return r.decodeBool(raw, rank, shape, totalElements)
	case typeInt8:
		return r.decodeInt8(raw, rank, shape, totalElements)
	case typeInt16:
		return r.decodeInt16(raw, rank, shape, totalElements)
	case typeInt32:
		return r.decodeInt32(raw, rank, shape, totalElements)
	case typeFloat64:
		return r.decodeFloat64(raw, rank, shape, totalElements)
	case typeChar8:
		return r.decodeChar8(raw, rank, shape, totalElements)
	case typeChar16:
		return r.decodeChar16(raw, rank, shape, totalElements)
	case typeChar32:
		return r.decodeChar32(raw, rank, shape, totalElements)
	case typeComplex:
		return r.decodeComplex(raw, rank, shape, totalElements)
	case typeDec128:
		return r.decodeDec128(raw, rank, shape, totalElements)
	default:
		return nil, fmt.Errorf("amicable: unknown type code 0x%02X", typeCode)
	}
}

func (r *reader) readNested(rank int, shape []int, totalElements int) (any, error) {
	// For empty nested arrays, there's a prototype child
	numChildren := totalElements
	if numChildren == 0 {
		numChildren = 1
	}

	children := make([]any, numChildren)
	for i := range numChildren {
		child, err := r.readArray()
		if err != nil {
			return nil, fmt.Errorf("amicable: reading nested child %d: %w", i, err)
		}
		children[i] = child
	}

	if totalElements == 0 {
		// Empty nested: prototype was read but we return empty
		if rank == 0 {
			// Shouldn't happen for empty, but be safe
			return children[0], nil
		}
		return &codec.Array{Data: []any{}, Shape: shape}, nil
	}

	// Scalar enclosed (rank 0): unwrap
	if rank == 0 {
		return children[0], nil
	}

	// Vector
	if rank == 1 {
		return children, nil
	}

	// Higher rank
	return &codec.Array{Data: children, Shape: shape}, nil
}

// --- Decoders ---

func (r *reader) decodeBool(raw []byte, rank int, shape []int, n int) (any, error) {
	vals := make([]any, n)
	for i := range n {
		byteIdx := i / 8
		bitIdx := 7 - (i % 8) // MSB first
		if raw[byteIdx]&(1<<bitIdx) != 0 {
			vals[i] = 1
		} else {
			vals[i] = 0
		}
	}
	return wrapResult(vals, rank, shape), nil
}

func (r *reader) decodeInt8(raw []byte, rank int, shape []int, n int) (any, error) {
	vals := make([]any, n)
	for i := range n {
		vals[i] = int(int8(raw[i]))
	}
	return wrapResult(vals, rank, shape), nil
}

func (r *reader) decodeInt16(raw []byte, rank int, shape []int, n int) (any, error) {
	vals := make([]any, n)
	for i := range n {
		vals[i] = int(int16(binary.LittleEndian.Uint16(raw[i*2:])))
	}
	return wrapResult(vals, rank, shape), nil
}

func (r *reader) decodeInt32(raw []byte, rank int, shape []int, n int) (any, error) {
	vals := make([]any, n)
	for i := range n {
		vals[i] = int(int32(binary.LittleEndian.Uint32(raw[i*4:])))
	}
	return wrapResult(vals, rank, shape), nil
}

func (r *reader) decodeFloat64(raw []byte, rank int, shape []int, n int) (any, error) {
	vals := make([]any, n)
	for i := range n {
		bits := binary.LittleEndian.Uint64(raw[i*8:])
		vals[i] = math.Float64frombits(bits)
	}
	return wrapResult(vals, rank, shape), nil
}

func (r *reader) decodeChar8(raw []byte, rank int, shape []int, n int) (any, error) {
	if rank <= 1 {
		// Character vectors and scalars → string
		return string(raw[:n]), nil
	}
	// Higher-rank char arrays: keep as Array with string data
	vals := make([]any, n)
	for i := range n {
		vals[i] = string(rune(raw[i]))
	}
	return &codec.Array{Data: vals, Shape: shape}, nil
}

func (r *reader) decodeChar16(raw []byte, rank int, shape []int, n int) (any, error) {
	if rank <= 1 {
		runes := make([]rune, n)
		for i := range n {
			runes[i] = rune(binary.LittleEndian.Uint16(raw[i*2:]))
		}
		return string(runes), nil
	}
	vals := make([]any, n)
	for i := range n {
		vals[i] = string(rune(binary.LittleEndian.Uint16(raw[i*2:])))
	}
	return &codec.Array{Data: vals, Shape: shape}, nil
}

func (r *reader) decodeChar32(raw []byte, rank int, shape []int, n int) (any, error) {
	if rank <= 1 {
		runes := make([]rune, n)
		for i := range n {
			runes[i] = rune(binary.LittleEndian.Uint32(raw[i*4:]))
		}
		return string(runes), nil
	}
	vals := make([]any, n)
	for i := range n {
		vals[i] = string(rune(binary.LittleEndian.Uint32(raw[i*4:])))
	}
	return &codec.Array{Data: vals, Shape: shape}, nil
}

func (r *reader) decodeComplex(raw []byte, rank int, shape []int, n int) (any, error) {
	vals := make([]any, n)
	for i := range n {
		re := math.Float64frombits(binary.LittleEndian.Uint64(raw[i*16:]))
		im := math.Float64frombits(binary.LittleEndian.Uint64(raw[i*16+8:]))
		vals[i] = complex(re, im)
	}
	return wrapResult(vals, rank, shape), nil
}

func (r *reader) decodeDec128(raw []byte, rank int, shape []int, n int) (any, error) {
	// Decimal128 has no Go equivalent. Store as [16]byte wrapped in a struct.
	vals := make([]any, n)
	for i := range n {
		var d Decimal128
		copy(d[:], raw[i*16:i*16+16])
		vals[i] = d
	}
	return wrapResult(vals, rank, shape), nil
}

// Decimal128 holds a 128-bit IEEE 754 decimal float as raw bytes.
// Go has no native decimal128; this preserves the exact bits for round-tripping.
type Decimal128 [16]byte

// Raw holds the complete serialized bytes of an opaque 220⌶ value (including
// magic header). Used for types that amicable can't parse structurally — e.g.
// ⎕OR (object representation) and namespaces — but can round-trip exactly.
type Raw []byte

// wrapResult converts a flat slice into the appropriate Go type based on rank/shape.
func wrapResult(vals []any, rank int, shape []int) any {
	if rank == 0 {
		if len(vals) == 0 {
			return 0
		}
		return vals[0]
	}
	if rank == 1 {
		if vals == nil {
			return []any{}
		}
		return vals
	}
	return &codec.Array{Data: vals, Shape: shape}
}

// --- Writer ---

type writer struct {
	buf     []byte
	ptrSize int
}

func (w *writer) writeByte(b byte) {
	w.buf = append(w.buf, b)
}

func (w *writer) writePtr(v uint64) {
	if w.ptrSize == 8 {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, v)
		w.buf = append(w.buf, b...)
	} else {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, uint32(v))
		w.buf = append(w.buf, b...)
	}
}

func (w *writer) writeTypeRank(typeCode byte, rank int, nested bool) {
	flags := byte(flagSimple)
	if nested {
		flags = flagNested
	}
	rankByte := byte(rank<<4) | flags
	// 2 bytes type/rank + padding to ptrSize
	w.writeByte(rankByte)
	w.writeByte(typeCode)
	for range w.ptrSize - 2 {
		w.writeByte(0)
	}
}

// padDataFrom pads the buffer so that the region starting at dataStart
// is a multiple of ptrSize bytes. This is used after writing raw array data.
func (w *writer) padDataFrom(dataStart int) {
	written := len(w.buf) - dataStart
	if written == 0 {
		return
	}
	rem := written % w.ptrSize
	if rem != 0 {
		for range w.ptrSize - rem {
			w.writeByte(0)
		}
	}
}

func (w *writer) writeArray(v any) error {
	switch val := v.(type) {
	case bool:
		if val {
			return w.writeSimpleScalar(typeInt8, []byte{1})
		}
		return w.writeSimpleScalar(typeInt8, []byte{0})

	case int:
		return w.writeIntScalar(val)

	case float64:
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, math.Float64bits(val))
		return w.writeSimpleScalar(typeFloat64, b)

	case complex128:
		b := make([]byte, 16)
		binary.LittleEndian.PutUint64(b[:8], math.Float64bits(real(val)))
		binary.LittleEndian.PutUint64(b[8:], math.Float64bits(imag(val)))
		return w.writeSimpleScalar(typeComplex, b)

	case Decimal128:
		return w.writeSimpleScalar(typeDec128, val[:])

	case string:
		return w.writeString(val)

	case []any:
		return w.writeVector(val)

	case *codec.Array:
		return w.writeShapedArray(val)

	default:
		return fmt.Errorf("amicable: unsupported type %T", v)
	}
}

func (w *writer) writeIntScalar(v int) error {
	if v >= -128 && v <= 127 {
		return w.writeSimpleScalar(typeInt8, []byte{byte(int8(v))})
	}
	if v >= -32768 && v <= 32767 {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, uint16(int16(v)))
		return w.writeSimpleScalar(typeInt16, b)
	}
	if v >= -2147483648 && v <= 2147483647 {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, uint32(int32(v)))
		return w.writeSimpleScalar(typeInt32, b)
	}
	// Large ints go as float64 (Dyalog does this too)
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, math.Float64bits(float64(v)))
	return w.writeSimpleScalar(typeFloat64, b)
}

func (w *writer) writeSimpleScalar(typeCode byte, data []byte) error {
	dataWords := (len(data) + w.ptrSize - 1) / w.ptrSize
	if dataWords < 1 {
		dataWords = 1
	}
	size := uint64(3 + dataWords) // size + type + data
	w.writePtr(size)
	w.writeTypeRank(typeCode, 0, false)
	w.buf = append(w.buf, data...)
	// Pad data to ptrSize words
	target := len(w.buf) + (dataWords*w.ptrSize - len(data))
	for len(w.buf) < target {
		w.writeByte(0)
	}
	return nil
}

func (w *writer) writeString(s string) error {
	runes := []rune(s)
	n := len(runes)

	// Determine character width
	maxVal := rune(0)
	for _, r := range runes {
		if r > maxVal {
			maxVal = r
		}
	}

	var typeCode byte
	var elemSize int
	switch {
	case maxVal <= 0xFF:
		typeCode = typeChar8
		elemSize = 1
	case maxVal <= 0xFFFF:
		typeCode = typeChar16
		elemSize = 2
	default:
		typeCode = typeChar32
		elemSize = 4
	}

	if n == 0 {
		// Empty string: rank-1 vector with 0 elements
		size := uint64(3 + 1) // size + type + shape + empty data word
		w.writePtr(size)
		w.writeTypeRank(typeCode, 1, false)
		w.writePtr(0) // shape[0] = 0
		return nil
	}

	// Encode data
	dataBytes := n * elemSize
	dataWords := (dataBytes + w.ptrSize - 1) / w.ptrSize
	if dataWords < 1 {
		dataWords = 1
	}

	// Single char → scalar (rank 0)
	if n == 1 {
		data := make([]byte, elemSize)
		switch typeCode {
		case typeChar8:
			data[0] = byte(runes[0])
		case typeChar16:
			binary.LittleEndian.PutUint16(data, uint16(runes[0]))
		case typeChar32:
			binary.LittleEndian.PutUint32(data, uint32(runes[0]))
		}
		return w.writeSimpleScalar(typeCode, data)
	}

	// Vector (rank 1)
	size := uint64(3 + 1 + dataWords) // size + type + shape + data
	w.writePtr(size)
	w.writeTypeRank(typeCode, 1, false)
	w.writePtr(uint64(n))

	dataStart := len(w.buf)
	for _, r := range runes {
		switch typeCode {
		case typeChar8:
			w.writeByte(byte(r))
		case typeChar16:
			b := make([]byte, 2)
			binary.LittleEndian.PutUint16(b, uint16(r))
			w.buf = append(w.buf, b...)
		case typeChar32:
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, uint32(r))
			w.buf = append(w.buf, b...)
		}
	}
	w.padDataFrom(dataStart)
	return nil
}

func (w *writer) writeVector(vals []any) error {
	if len(vals) == 0 {
		// Empty numeric vector (zilde-like)
		size := uint64(3 + 1) // size + type + shape
		w.writePtr(size)
		w.writeTypeRank(typeInt8, 1, false)
		w.writePtr(0)
		return nil
	}

	// Check if all elements are the same simple type
	if typeCode, ok := homogeneousType(vals); ok {
		return w.writeHomogeneousVector(typeCode, vals)
	}

	// Nested vector
	return w.writeNestedVector(vals)
}

func (w *writer) writeHomogeneousVector(typeCode byte, vals []any) error {
	n := len(vals)

	var data []byte
	switch typeCode {
	case typeBool:
		nBytes := (n + 7) / 8
		data = make([]byte, nBytes)
		for i, v := range vals {
			bit := 0
			switch vv := v.(type) {
			case int:
				bit = vv
			case bool:
				if vv {
					bit = 1
				}
			}
			if bit != 0 {
				data[i/8] |= 1 << (7 - i%8)
			}
		}
	case typeInt8:
		data = make([]byte, n)
		for i, v := range vals {
			data[i] = byte(int8(v.(int)))
		}
	case typeInt16:
		data = make([]byte, n*2)
		for i, v := range vals {
			binary.LittleEndian.PutUint16(data[i*2:], uint16(int16(v.(int))))
		}
	case typeInt32:
		data = make([]byte, n*4)
		for i, v := range vals {
			binary.LittleEndian.PutUint32(data[i*4:], uint32(int32(v.(int))))
		}
	case typeFloat64:
		data = make([]byte, n*8)
		for i, v := range vals {
			var f float64
			switch vv := v.(type) {
			case float64:
				f = vv
			case int:
				f = float64(vv)
			}
			binary.LittleEndian.PutUint64(data[i*8:], math.Float64bits(f))
		}
	case typeComplex:
		data = make([]byte, n*16)
		for i, v := range vals {
			c := v.(complex128)
			binary.LittleEndian.PutUint64(data[i*16:], math.Float64bits(real(c)))
			binary.LittleEndian.PutUint64(data[i*16+8:], math.Float64bits(imag(c)))
		}
	default:
		return fmt.Errorf("amicable: unsupported homogeneous type 0x%02X", typeCode)
	}

	dataWords := (len(data) + w.ptrSize - 1) / w.ptrSize
	if dataWords < 1 {
		dataWords = 1
	}
	size := uint64(3 + 1 + dataWords)
	w.writePtr(size)
	w.writeTypeRank(typeCode, 1, false)
	w.writePtr(uint64(n))
	dataStart := len(w.buf)
	w.buf = append(w.buf, data...)
	w.padDataFrom(dataStart)
	return nil
}

func (w *writer) writeNestedVector(vals []any) error {
	n := len(vals)
	size := uint64(3 + 1 + n) // size + type + shape + n children
	w.writePtr(size)
	w.writeTypeRank(typePointer, 1, true)
	w.writePtr(uint64(n))
	for i, v := range vals {
		if err := w.writeArray(v); err != nil {
			return fmt.Errorf("amicable: writing nested element %d: %w", i, err)
		}
	}
	return nil
}

func (w *writer) writeShapedArray(a *codec.Array) error {
	rank := len(a.Shape)
	totalElements := 1
	for _, d := range a.Shape {
		totalElements *= d
	}

	// Check if all elements are simple and same type
	if totalElements > 0 {
		if typeCode, ok := homogeneousType(a.Data); ok {
			return w.writeHomogeneousShaped(typeCode, a.Data, a.Shape, rank, totalElements)
		}
	}

	// Nested shaped array
	numChildren := totalElements
	if numChildren == 0 {
		numChildren = 1 // prototype
	}
	size := uint64(3 + rank + numChildren)
	w.writePtr(size)
	w.writeTypeRank(typePointer, rank, true)
	for _, d := range a.Shape {
		w.writePtr(uint64(d))
	}
	if totalElements == 0 {
		// Write empty string as prototype
		if err := w.writeArray(""); err != nil {
			return err
		}
	} else {
		for i, v := range a.Data {
			if err := w.writeArray(v); err != nil {
				return fmt.Errorf("amicable: writing shaped element %d: %w", i, err)
			}
		}
	}
	return nil
}

func (w *writer) writeHomogeneousShaped(typeCode byte, vals []any, shape []int, rank, totalElements int) error {
	var data []byte
	switch typeCode {
	case typeBool:
		nBytes := (totalElements + 7) / 8
		data = make([]byte, nBytes)
		for i, v := range vals {
			bit := 0
			switch vv := v.(type) {
			case int:
				bit = vv
			case bool:
				if vv {
					bit = 1
				}
			}
			if bit != 0 {
				data[i/8] |= 1 << (7 - i%8)
			}
		}
	case typeInt8:
		data = make([]byte, totalElements)
		for i, v := range vals {
			data[i] = byte(int8(v.(int)))
		}
	case typeInt16:
		data = make([]byte, totalElements*2)
		for i, v := range vals {
			binary.LittleEndian.PutUint16(data[i*2:], uint16(int16(v.(int))))
		}
	case typeInt32:
		data = make([]byte, totalElements*4)
		for i, v := range vals {
			binary.LittleEndian.PutUint32(data[i*4:], uint32(int32(v.(int))))
		}
	case typeFloat64:
		data = make([]byte, totalElements*8)
		for i, v := range vals {
			var f float64
			switch vv := v.(type) {
			case float64:
				f = vv
			case int:
				f = float64(vv)
			}
			binary.LittleEndian.PutUint64(data[i*8:], math.Float64bits(f))
		}
	case typeComplex:
		data = make([]byte, totalElements*16)
		for i, v := range vals {
			c := v.(complex128)
			binary.LittleEndian.PutUint64(data[i*16:], math.Float64bits(real(c)))
			binary.LittleEndian.PutUint64(data[i*16+8:], math.Float64bits(imag(c)))
		}
	case typeChar8, typeChar16, typeChar32:
		// Character matrices: elements are single-char strings
		elemSize := 1
		if typeCode == typeChar16 {
			elemSize = 2
		} else if typeCode == typeChar32 {
			elemSize = 4
		}
		data = make([]byte, totalElements*elemSize)
		for i, v := range vals {
			s := v.(string)
			r := []rune(s)[0]
			switch typeCode {
			case typeChar8:
				data[i] = byte(r)
			case typeChar16:
				binary.LittleEndian.PutUint16(data[i*2:], uint16(r))
			case typeChar32:
				binary.LittleEndian.PutUint32(data[i*4:], uint32(r))
			}
		}
	default:
		return fmt.Errorf("amicable: unsupported shaped type 0x%02X", typeCode)
	}

	dataWords := (len(data) + w.ptrSize - 1) / w.ptrSize
	if dataWords < 1 {
		dataWords = 1
	}
	size := uint64(3 + rank + dataWords)
	w.writePtr(size)
	w.writeTypeRank(typeCode, rank, false)
	for _, d := range shape {
		w.writePtr(uint64(d))
	}
	dataStart := len(w.buf)
	w.buf = append(w.buf, data...)
	w.padDataFrom(dataStart)
	return nil
}

// homogeneousType returns the binary type code if all elements share a single
// simple type. Returns false for mixed or nested slices.
func homogeneousType(vals []any) (byte, bool) {
	if len(vals) == 0 {
		return 0, false
	}

	// Determine type from first element
	var baseType byte
	switch vals[0].(type) {
	case int:
		baseType = typeInt8 // will be upgraded below
	case float64:
		baseType = typeFloat64
	case complex128:
		baseType = typeComplex
	case bool:
		baseType = typeBool
	case string:
		baseType = typeChar8 // will be upgraded below
	default:
		return 0, false
	}

	// Check all elements match + determine width
	allBool := true
	minInt, maxInt := 0, 0
	maxRune := rune(0)

	for _, v := range vals {
		switch vv := v.(type) {
		case int:
			if baseType != typeInt8 && baseType != typeFloat64 {
				return 0, false
			}
			if vv != 0 && vv != 1 {
				allBool = false
			}
			if vv < minInt {
				minInt = vv
			}
			if vv > maxInt {
				maxInt = vv
			}
		case float64:
			if baseType != typeFloat64 && baseType != typeInt8 {
				return 0, false
			}
			allBool = false
			baseType = typeFloat64
		case complex128:
			if baseType != typeComplex {
				return 0, false
			}
			allBool = false
		case bool:
			if baseType != typeBool && baseType != typeInt8 {
				return 0, false
			}
		case string:
			if baseType != typeChar8 {
				return 0, false
			}
			for _, r := range vv {
				if r > maxRune {
					maxRune = r
				}
			}
		default:
			return 0, false
		}
	}

	// Determine specific type
	switch baseType {
	case typeInt8:
		if allBool {
			return typeBool, true
		}
		switch {
		case minInt >= -128 && maxInt <= 127:
			return typeInt8, true
		case minInt >= -32768 && maxInt <= 32767:
			return typeInt16, true
		case minInt >= -2147483648 && maxInt <= 2147483647:
			return typeInt32, true
		default:
			return typeFloat64, true
		}
	case typeChar8:
		switch {
		case maxRune <= 0xFF:
			return typeChar8, true
		case maxRune <= 0xFFFF:
			return typeChar16, true
		default:
			return typeChar32, true
		}
	}
	return baseType, true
}

