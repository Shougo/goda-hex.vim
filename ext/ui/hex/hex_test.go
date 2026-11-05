package hex

import (
	"reflect"
	"testing"
)

func TestLines(t *testing.T) {
	data := []byte("Hello, world!\n\x00\x01\x02\xe3\x81\x82")
	lines, hls, err := Lines(data, 0x10, "utf-8", func(addr int64, b byte) string {
		if addr == 0x11 {
			return "Visual"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"00000010: 48 65 6c 6c 6f 2c 20 77 6f 72 6c 64 21 0a 00 01 |   Hello, world!...",
		"00000020: 02 e3 81 82                                     |   .あ",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("lines\n%q\nwant\n%q", lines, want)
	}
	if !reflect.DeepEqual(hls, [][]any{{1, 13, 2, "Visual"}}) {
		t.Errorf("highlights %v", hls)
	}
	if lines, _, _ := Lines(nil, 0, "utf-8", nil); !reflect.DeepEqual(lines, []string{"00000000: "}) {
		t.Errorf("empty %q", lines)
	}
}

func TestValue(t *testing.T) {
	no := false
	for _, c := range []struct {
		raw  string
		tp   TypeParams
		want []int
	}{
		{"41 ??", TypeParams{Type: "hex"}, []int{0x41, -1}},
		{"あ", TypeParams{Type: "string"}, []int{0xe3, 0x81, 0x82}},
		{"258", TypeParams{Type: "number", Size: 2}, []int{2, 1}},
		{"-1", TypeParams{Type: "number", Size: 1, IsSigned: true}, []int{0xff}},
		{"0x0102", TypeParams{Type: "number", Size: 2, IsLittle: &no}, []int{1, 2}},
		{"1.5", TypeParams{Type: "floating", Size: 4}, []int{0, 0, 0xc0, 0x3f}},
	} {
		got, err := Value(c.raw, c.tp, "utf-8")
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("Value(%q, %+v) = %v, %v; want %v", c.raw, c.tp, got, err, c.want)
		}
	}
	if _, err := Value("300", TypeParams{Type: "number", Size: 1}, "utf-8"); err == nil {
		t.Error("300 in a byte")
	}
}

func TestChecksum(t *testing.T) {
	data := []byte("123456789")
	for method, want := range map[string]string{
		"sum": "221", "crc-8": "F4", "crc-16": "29B1", "crc-32": "CBF43926",
		"md5": "25f9e794323b453885f5181f1b624d0b",
	} {
		if got, err := Checksum(data, method); err != nil || got != want {
			t.Errorf("%s = %s, %v; want %s", method, got, err, want)
		}
	}
	if _, err := Checksum(data, "nope"); err == nil {
		t.Error("an unknown method")
	}
}

func TestJumpAddress(t *testing.T) {
	for in, want := range map[string]int64{"0x20": 0x20, "50%": 0x10 + 50, "+3": 13, "-3": 7} {
		if got, err := JumpAddress(in, 10, 0x10, 100); err != nil || got != want {
			t.Errorf("%s = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"12", "0xzz", "200%"} {
		if _, err := JumpAddress(bad, 0, 0, 10); err == nil {
			t.Errorf("%q", bad)
		}
	}
}
