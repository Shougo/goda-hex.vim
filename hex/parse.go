package hex

import (
	"fmt"
	"math/big"
	"regexp"
)

// lineRe is a declaration of C: "uint16_t e_type;".
var lineRe = regexp.MustCompile(`^\s*(\S+)\s+(\S+)\s*;\s*$`)

// typeSizes are the types of ParseLine.
var typeSizes = map[string]int{"uint8_t": 1, "uint16_t": 2, "uint32_t": 4, "uint64_t": 8,
	"int8_t": 1, "int16_t": 2, "int32_t": 4, "int64_t": 8}

// ParseLine reads at addr the value of a declaration of C ("uint16_t
// e_type;", an integer type of stdint.h), as ddx.vim's parseLine: the value
// and the address after it.
func ParseLine(b *Buffer, addr int64, line string, little bool) (AnalyzeValue, int64, error) {
	m := lineRe.FindStringSubmatch(line)
	if m == nil {
		return AnalyzeValue{}, addr, fmt.Errorf("parse error in %q", line)
	}
	size, ok := typeSizes[m[1]]
	if !ok {
		return AnalyzeValue{}, addr, fmt.Errorf("unsupported type %q in %q", m[1], line)
	}
	data := b.Bytes(addr, size)
	if len(data) < size {
		return AnalyzeValue{}, addr, fmt.Errorf("no %d bytes at 0x%x for %q", size, addr, line)
	}
	n, err := Int(data, size, little, m[1][0] == 'i')
	if err != nil {
		return AnalyzeValue{}, addr, err
	}
	return IntValue(m[2], n, size, little, addr), addr + int64(size), nil
}

// IntValue is an integer value of an analyzer (as an int64: the bits of
// one that does not fit).
func IntValue(name string, n *big.Int, size int, little bool, addr int64) AnalyzeValue {
	v := n.Int64()
	if !n.IsInt64() {
		v = int64(n.Uint64())
	}
	return AnalyzeValue{Name: name, RawType: "integer", Value: v, Size: size, IsLittle: &little, Address: addr}
}

// Header collects the values of a part of a file: the first error stops it.
type Header struct {
	B      *Buffer
	Result AnalyzeResult
	Addr   int64
	Little bool
	Err    error
}

// NewHeader starts the part name at addr.
func NewHeader(b *Buffer, name string, addr int64, little bool) *Header {
	return &Header{B: b, Result: AnalyzeResult{Name: name, Values: []AnalyzeValue{}}, Addr: addr, Little: little}
}

// Line reads the next value (ParseLine) and returns it (0 after an error).
func (h *Header) Line(line string) int64 {
	if h.Err != nil {
		return 0
	}
	v, next, err := ParseLine(h.B, h.Addr, line, h.Little)
	if err != nil {
		h.Err = err
		return 0
	}
	h.Result.Values = append(h.Result.Values, v)
	h.Addr = next
	return v.Value.(int64)
}

// Bytes reads n bytes, each a value name0, name1, ...
func (h *Header) Bytes(name string, n int) {
	if h.Err != nil {
		return
	}
	data := h.B.Bytes(h.Addr, n)
	if len(data) < n {
		h.Err = fmt.Errorf("no %d bytes at 0x%x for %s", n, h.Addr, name)
		return
	}
	for i, c := range data {
		h.Result.Values = append(h.Result.Values, IntValue(fmt.Sprintf("%s%d", name, i), big.NewInt(int64(c)), 1, true, h.Addr+int64(i)))
	}
	h.Addr += int64(n)
}

// String reads a string of n bytes (in encoding) as the value name.
func (h *Header) String(name string, n int, encoding string) {
	if h.Err != nil || n < 0 {
		return
	}
	data := h.B.Bytes(h.Addr, n)
	s := string(data)
	if encoding == CP932 {
		if t, err := sjisDecoder.Bytes(data); err == nil {
			s = string(t)
		}
	}
	h.Result.Values = append(h.Result.Values, AnalyzeValue{Name: name, RawType: "string", Value: s,
		Size: n, Address: h.Addr, Encoding: encoding})
	h.Addr += int64(n)
}

// Mark adds a value of where something is (its value "?"), and skips n
// bytes.
func (h *Header) Mark(name string, n int64) {
	if h.Err != nil {
		return
	}
	h.Result.Values = append(h.Result.Values, AnalyzeValue{Name: name, RawType: "string", Value: "?", Address: h.Addr})
	h.Addr += n
}
