package hex

import (
	"math/big"
	"reflect"
	"testing"
)

func TestChars(t *testing.T) {
	for _, c := range []struct {
		in   string
		enc  string
		want string
	}{
		{"Hi\n\x00", UTF8, "Hi.."},
		{"\xe3\x81\x82!", UTF8, "あ!"},
		{"\xe3\x81", UTF8, ".."},
		{"\xe2\x80\x8b", UTF8, "..."}, // zero width space
		{"\x82\xa0a", CP932, "あa"},
		{"\x82", CP932, "."},
	} {
		got, err := Chars([]byte(c.in), c.enc)
		if err != nil || got != c.want {
			t.Errorf("Chars(%q, %s) = %q, %v; want %q", c.in, c.enc, got, err, c.want)
		}
	}
	if _, err := Chars(nil, "euc-jp"); err == nil {
		t.Error("an unknown encoding")
	}
}

func TestEncode(t *testing.T) {
	for enc, want := range map[string]string{
		UTF8: "\xe3\x81\x82", CP932: "\x82\xa0", UTF16LE: "\x42\x30", UTF16BE: "\x30\x42",
	} {
		if got, err := EncodeString("あ", enc); err != nil || string(got) != want {
			t.Errorf("%s: %q %v", enc, got, err)
		}
	}
	if got, _ := FitString("あい", 3, CP932); string(got) != "\x82\xa0" {
		t.Errorf("cut cp932 %q", got)
	}
	if got, _ := FitString("ab", 4, UTF8); string(got) != "ab\x00\x00" {
		t.Errorf("pad %q", got)
	}
}

func TestNumbers(t *testing.T) {
	b, err := IntBytes(big.NewInt(-2), 2, true, true)
	if err != nil || !reflect.DeepEqual(b, []byte{0xfe, 0xff}) {
		t.Errorf("-2: %x %v", b, err)
	}
	if _, err := IntBytes(big.NewInt(256), 1, true, false); err == nil {
		t.Error("256 in a byte")
	}
	b, _ = IntBytes(big.NewInt(0x1234), 4, false, false)
	if !reflect.DeepEqual(b, []byte{0, 0, 0x12, 0x34}) {
		t.Errorf("big endian %x", b)
	}
	n, _ := Int([]byte{0xfe, 0xff}, 2, true, true)
	if n.Int64() != -2 {
		t.Errorf("Int %s", n)
	}
	f, _ := FloatBytes(1.5, 4, true)
	if v, _ := Float(f, 4, true); v != 1.5 {
		t.Errorf("Float %v", v)
	}
}

func TestParseHex(t *testing.T) {
	if got := ParseHex("3f ??0A"); !reflect.DeepEqual(got, []int{0x3f, -1, 0x0a}) {
		t.Errorf("%v", got)
	}
	for _, bad := range []string{"", "3", "zz"} {
		if ParseHex(bad) != nil {
			t.Errorf("%q", bad)
		}
	}
}

func TestStrings(t *testing.T) {
	b, _ := open(t, "\x00hello\x01\x02abc\x00\xe3\x81\x82\xe3\x81\x84xyz", 0, 0)
	got, err := b.Strings(UTF8, 4)
	if err != nil || len(got) != 2 || got[0].Text != "hello" || got[0].Address != 1 ||
		got[1].Text != "あいxyz" || got[1].Address != 12 {
		t.Errorf("%+v %v", got, err)
	}
	b, _ = open(t, "h\x00e\x00l\x00l\x00o\x00\x00\x00", 0, 0)
	got, _ = b.Strings(UTF16LE, 4)
	if len(got) == 0 || got[0].Text != "hello" || got[0].Address != 0 {
		t.Errorf("utf-16le %+v", got)
	}
	got, _ = b.Strings(ASCII, 1)
	if len(got) != 5 {
		t.Errorf("ascii %+v", got)
	}
	if Sanitize("a\x00\t\x7f") != `a\0\t\x7F` {
		t.Errorf("Sanitize %q", Sanitize("a\x00\t\x7f"))
	}
}

func TestBinaryDiff(t *testing.T) {
	got := BinaryDiff(10, []byte{1, 2, 3}, []byte{1, 9})
	if len(got) != 2 || got[0].Address != 11 || got[0].Type != "changed" || got[1].Type != "deleted" ||
		got[1].Address != 12 {
		t.Errorf("%+v", got)
	}
}
