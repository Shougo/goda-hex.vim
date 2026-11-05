package hex

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

// Encodings of the strings of a file.
const (
	UTF8    = "utf-8"
	UTF16LE = "utf-16le"
	UTF16BE = "utf-16be"
	ASCII   = "ascii"
	Latin1  = "latin1"
	CP932   = "cp932"
)

// Chars shows the bytes as text in encoding ("utf-8" or "cp932"), as
// ddx.vim: a printable character for each one, "." for each byte that is
// not one (a control code, an invalid sequence).
func Chars(data []byte, encoding string) (string, error) {
	switch strings.ToLower(encoding) {
	case "", UTF8, "utf8":
		return charsUTF8(data), nil
	case CP932:
		return charsCP932(data), nil
	}
	return "", fmt.Errorf("invalid encoding: %q (utf-8, cp932)", encoding)
}

// invisible are the code points shown as ".".
var invisible = map[rune]bool{
	0x180e: true, 0x200b: true, 0x200c: true, 0x200d: true,
	0x200e: true, 0x200f: true, 0xfeff: true,
}

func printableRune(r rune) bool {
	switch {
	case r <= 0x1f, r >= 0x7f && r <= 0x9f, r == utf8.RuneError,
		r >= 0xe0100 && r <= 0xe010f, invisible[r]:
		return false
	}
	return true
}

func charsUTF8(data []byte) string {
	var sb strings.Builder
	for i := 0; i < len(data); {
		c := data[i]
		if c < 0x80 {
			if c >= 0x20 && c <= 0x7e {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('.')
			}
			i++
			continue
		}
		r, n := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError || !printableRune(r) {
			// A sequence: a dot for each of its bytes.
			n = max(n, 1)
			if r == utf8.RuneError {
				n = 1
			}
			sb.WriteString(strings.Repeat(".", n))
			i += n
			continue
		}
		sb.WriteRune(r)
		i += n
	}
	return sb.String()
}

var sjisDecoder = japanese.ShiftJIS.NewDecoder()

func sjisLead(c byte) bool { return 0x81 <= c && c <= 0x9f || 0xe0 <= c && c <= 0xef }

func charsCP932(data []byte) string {
	var sb strings.Builder
	for i := 0; i < len(data); {
		c := data[i]
		if c < 0x80 {
			if c >= 0x20 && c <= 0x7e {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('.')
			}
			i++
			continue
		}
		if i+1 < len(data) && sjisLead(c) && data[i+1] >= 0x40 && data[i+1] <= 0xfc && data[i+1] != 0x7f {
			s, err := sjisDecoder.Bytes(data[i : i+2])
			if err != nil || strings.ContainsRune(string(s), utf8.RuneError) {
				sb.WriteString("..")
			} else {
				sb.Write(s)
			}
			i += 2
			continue
		}
		sb.WriteByte('.')
		i++
	}
	return sb.String()
}

// EncodeString is s in encoding.
func EncodeString(s, encoding string) ([]byte, error) {
	switch strings.ToLower(encoding) {
	case "", UTF8, "utf8":
		return []byte(s), nil
	case CP932:
		b, err := japanese.ShiftJIS.NewEncoder().Bytes([]byte(s))
		if err != nil {
			return nil, fmt.Errorf("%q cannot be cp932: %w", s, err)
		}
		return b, nil
	case UTF16LE, UTF16BE:
		units := utf16.Encode([]rune(s))
		out := make([]byte, 2*len(units))
		for i, u := range units {
			if encoding == UTF16LE {
				binary.LittleEndian.PutUint16(out[2*i:], u)
			} else {
				binary.BigEndian.PutUint16(out[2*i:], u)
			}
		}
		return out, nil
	case ASCII, Latin1:
		var out []byte
		for _, r := range s {
			if encoding == ASCII {
				out = append(out, byte(r)&0x7f)
			} else {
				out = append(out, byte(r))
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("invalid encoding: %q", encoding)
}

// FitString is s in encoding, padded with zeros to size or cut to it (not
// in the middle of a character of UTF-16 or a lead byte of cp932); size 0
// is that of s.
func FitString(s string, size int, encoding string) ([]byte, error) {
	b, err := EncodeString(s, encoding)
	if err != nil || size <= 0 {
		return b, err
	}
	if len(b) > size {
		cut := size
		switch encoding {
		case UTF16LE, UTF16BE:
			cut -= cut % 2
		case CP932:
			// Not a lead byte at the end.
			lead := false
			for i := 0; i < cut; i++ {
				if lead {
					lead = false
				} else if sjisLead(b[i]) {
					lead = true
				}
			}
			if lead {
				cut--
			}
		}
		return b[:cut], nil
	}
	return append(b, make([]byte, size-len(b))...), nil
}

// IntBytes is n as an integer of size bytes, signed (two's complement) or
// not.
func IntBytes(n *big.Int, size int, little, signed bool) ([]byte, error) {
	if size <= 0 || size > 32 {
		return nil, fmt.Errorf("invalid size: %d", size)
	}
	bits := uint(8 * size)
	v := new(big.Int).Set(n)
	if signed {
		lo := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), bits-1))
		hi := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), bits-1), big.NewInt(1))
		if v.Cmp(lo) < 0 || v.Cmp(hi) > 0 {
			return nil, fmt.Errorf("%s is out of a signed integer of %d bytes (%s..%s)", n, size, lo, hi)
		}
		if v.Sign() < 0 {
			v.Add(v, new(big.Int).Lsh(big.NewInt(1), bits))
		}
	} else {
		hi := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), bits), big.NewInt(1))
		if v.Sign() < 0 || v.Cmp(hi) > 0 {
			return nil, fmt.Errorf("%s is out of an unsigned integer of %d bytes (0..%s)", n, size, hi)
		}
	}
	out := v.FillBytes(make([]byte, size)) // big endian
	if little {
		for i, j := 0, size-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, nil
}

// FloatBytes is f as a float of size bytes (4 or 8).
func FloatBytes(f float64, size int, little bool) ([]byte, error) {
	order := binary.ByteOrder(binary.BigEndian)
	if little {
		order = binary.LittleEndian
	}
	switch size {
	case 4:
		out := make([]byte, 4)
		order.PutUint32(out, math.Float32bits(float32(f)))
		return out, nil
	case 8:
		out := make([]byte, 8)
		order.PutUint64(out, math.Float64bits(f))
		return out, nil
	}
	return nil, fmt.Errorf("invalid size of a float: %d (4, 8)", size)
}

// Int reads an integer of size bytes (1, 2, 4, 8) at the start of data.
func Int(data []byte, size int, little, signed bool) (*big.Int, error) {
	if size <= 0 || len(data) < size {
		return nil, fmt.Errorf("not %d bytes", size)
	}
	b := make([]byte, size)
	copy(b, data[:size])
	if little {
		for i, j := 0, size-1; i < j; i, j = i+1, j-1 {
			b[i], b[j] = b[j], b[i]
		}
	}
	v := new(big.Int).SetBytes(b)
	if signed && b[0]&0x80 != 0 {
		v.Sub(v, new(big.Int).Lsh(big.NewInt(1), uint(8*size)))
	}
	return v, nil
}

// Float reads a float of size bytes (4, 8) at the start of data.
func Float(data []byte, size int, little bool) (float64, error) {
	if len(data) < size {
		return 0, fmt.Errorf("not %d bytes", size)
	}
	order := binary.ByteOrder(binary.BigEndian)
	if little {
		order = binary.LittleEndian
	}
	switch size {
	case 4:
		return float64(math.Float32frombits(order.Uint32(data))), nil
	case 8:
		return math.Float64frombits(order.Uint64(data)), nil
	}
	return 0, fmt.Errorf("invalid size of a float: %d (4, 8)", size)
}

// ParseHex reads hexadecimal bytes ("3f 0a", "3f0a"), "??" for any byte
// (-1); nil when it is not one.
func ParseHex(s string) []int {
	s = strings.Join(strings.Fields(s), "")
	if s == "" || len(s)%2 != 0 {
		return nil
	}
	out := make([]int, 0, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		if s[i:i+2] == "??" {
			out = append(out, -1)
			continue
		}
		b, err := hex.DecodeString(s[i : i+2])
		if err != nil {
			return nil
		}
		out = append(out, int(b[0]))
	}
	return out
}

// Pattern is the bytes as a pattern of Search.
func Pattern(data []byte) []int {
	out := make([]int, len(data))
	for i, b := range data {
		out[i] = int(b)
	}
	return out
}

// Exact is the bytes of a pattern without "any byte"; false if it has one.
func Exact(pattern []int) ([]byte, bool) {
	out := make([]byte, len(pattern))
	for i, p := range pattern {
		if p < 0 {
			return nil, false
		}
		out[i] = byte(p)
	}
	return out, true
}

// Sanitize shows the control characters of an extracted string as escapes
// (\0, \t, \n, \r, \xNN, \uNNNN), as ddx.vim.
func Sanitize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x20 && r <= 0x7e:
			sb.WriteRune(r)
		case r == 0:
			sb.WriteString(`\0`)
		case r == '\t':
			sb.WriteString(`\t`)
		case r == '\n':
			sb.WriteString(`\n`)
		case r == '\r':
			sb.WriteString(`\r`)
		case !unicode.Is(unicode.C, r) && r != utf8.RuneError:
			sb.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&sb, `\x%02X`, r)
		case r <= 0xffff:
			fmt.Fprintf(&sb, `\u%04X`, r)
		default:
			fmt.Fprintf(&sb, `\u{%X}`, r)
		}
	}
	return sb.String()
}
