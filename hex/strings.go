package hex

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// ExtractedString is a string found in the buffer (goda_hex#get_strings()).
type ExtractedString struct {
	Text     string `json:"text"`
	Address  int64  `json:"address"`
	Encoding string `json:"encoding"`
}

// Strings are the strings of at least minLen characters in the buffer, in
// encoding (ascii, utf-8, utf-16le, utf-16be), in order.
func (b *Buffer) Strings(encoding string, minLen int) ([]ExtractedString, error) {
	minLen = max(minLen, 1)
	var out []ExtractedString
	var err error
	b.View(func(offset int64, data []byte) {
		switch strings.ToLower(encoding) {
		case ASCII:
			out = stringsASCII(data, offset, minLen)
		case "", UTF8, "utf8":
			out = stringsUTF8(data, offset, minLen)
		case UTF16LE, "utf16le":
			out = stringsUTF16(data, offset, minLen, false)
		case UTF16BE, "utf16be":
			out = stringsUTF16(data, offset, minLen, true)
		default:
			err = fmt.Errorf("unsupported encoding %q (ascii, utf-8, utf-16le, utf-16be)", encoding)
		}
	})
	return out, err
}

func asciiPrintable(c byte) bool {
	return c >= 0x20 && c <= 0x7e || c == '\t' || c == '\n' || c == '\r'
}

func stringsASCII(data []byte, offset int64, minLen int) []ExtractedString {
	var out []ExtractedString
	for i := 0; i < len(data); {
		for i < len(data) && !asciiPrintable(data[i]) {
			i++
		}
		start := i
		for i < len(data) && asciiPrintable(data[i]) {
			i++
		}
		if i-start >= minLen {
			out = append(out, ExtractedString{Text: string(data[start:i]), Address: offset + int64(start), Encoding: UTF8})
		}
	}
	return out
}

// stringsUTF8 are the runs of printable UTF-8 (tabs and newlines in them)
// of at least minLen characters: a control character or an invalid sequence
// ends one.
func stringsUTF8(data []byte, offset int64, minLen int) []ExtractedString {
	var out []ExtractedString
	for i := 0; i < len(data); {
		start, n := i, 0
		for i < len(data) {
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size <= 1 ||
				(r <= 0x1f && r != '\t' && r != '\n' && r != '\r') || r >= 0x7f && r <= 0x9f {
				break
			}
			n++
			i += size
		}
		if n >= minLen {
			out = append(out, ExtractedString{Text: string(data[start:i]), Address: offset + int64(start), Encoding: UTF8})
		}
		if i == start {
			i++
		}
	}
	return out
}

func utf16Printable(r rune) bool { return !unicode.In(r, unicode.C, unicode.Z) && r != utf8.RuneError }

// stringsUTF16 are the runs of printable characters (not of the categories
// C and Z) of UTF-16, at both alignments.
func stringsUTF16(data []byte, offset int64, minLen int, bigEndian bool) []ExtractedString {
	enc := UTF16LE
	order := binary.ByteOrder(binary.LittleEndian)
	if bigEndian {
		enc, order = UTF16BE, binary.BigEndian
	}
	var out []ExtractedString
	seen := map[int]bool{}
	for align := 0; align <= 1; align++ {
		var run []rune
		runStart := -1
		flush := func() {
			if runStart >= 0 && len(run) >= minLen && !seen[runStart] {
				seen[runStart] = true
				out = append(out, ExtractedString{Text: string(run), Address: offset + int64(runStart), Encoding: enc})
			}
			run, runStart = nil, -1
		}
		for i := align; i+1 < len(data); {
			u := order.Uint16(data[i:])
			r, size := rune(u), 2
			if utf16.IsSurrogate(r) {
				if i+3 < len(data) {
					r = utf16.DecodeRune(r, rune(order.Uint16(data[i+2:])))
					size = 4
				} else {
					r = utf8.RuneError
				}
			}
			if utf16Printable(r) {
				if runStart < 0 {
					runStart = i
				}
				run = append(run, r)
				i += size
				continue
			}
			flush()
			i += 2
		}
		flush()
	}
	// In order of their addresses.
	sortStrings(out)
	return out
}

func sortStrings(s []ExtractedString) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Address < s[j-1].Address; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Diff is a difference between two buffers (goda_hex#get_diff()): a byte
// changed, or the bytes one has after the end of the other.
type Diff struct {
	Address int64  `json:"address"`
	Type    string `json:"type"` // "changed", "added", "deleted"
	Old     []int  `json:"oldValue"`
	New     []int  `json:"newValue"`
}

func ints(b []byte) []int {
	out := make([]int, len(b))
	for i, c := range b {
		out[i] = int(c)
	}
	return out
}

// BinaryDiff are the differences of the bytes from offset of mine and of
// another: each byte changed, then those deleted (mine has more) or added
// (another has more).
func BinaryDiff(offset int64, mine, another []byte) []Diff {
	var out []Diff
	n := min(len(mine), len(another))
	for i := range n {
		if mine[i] != another[i] {
			out = append(out, Diff{Address: offset + int64(i), Type: "changed",
				Old: []int{int(mine[i])}, New: []int{int(another[i])}})
		}
	}
	if len(mine) > n {
		out = append(out, Diff{Address: offset + int64(n), Type: "deleted", Old: ints(mine[n:]), New: []int{}})
	}
	if len(another) > n {
		out = append(out, Diff{Address: offset + int64(n), Type: "added", Old: []int{}, New: ints(another[n:])})
	}
	return out
}
