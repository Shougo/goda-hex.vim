package analyzer_test

import (
	"archive/zip"
	"bytes"
	"debug/elf"
	"os"
	"path/filepath"
	"testing"

	elfa "github.com/Shougo/goda-hex.vim/ext/analyzer/elf"
	zipa "github.com/Shougo/goda-hex.vim/ext/analyzer/zip"
	"github.com/Shougo/goda-hex.vim/hex"
)

func buffer(t *testing.T, data []byte) *hex.Buffer {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	b := hex.NewBuffer()
	if err := b.Open(path, 0, 0); err != nil {
		t.Fatal(err)
	}
	return b
}

func value(r hex.AnalyzeResult, name string) any {
	for _, v := range r.Values {
		if v.Name == name {
			return v.Value
		}
	}
	return nil
}

// The ELF analyzer reads what debug/elf reads of the test binary.
func TestELF(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	f, err := elf.Open(exe)
	if err != nil {
		t.Skip("not an ELF binary")
	}
	defer f.Close()
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	b := buffer(t, data)
	a := elfa.New()
	if !a.Detect(b, nil) {
		t.Fatal("not detected")
	}
	res, err := a.Parse(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1+len(f.Progs) {
		t.Fatalf("%d results, %d programs", len(res), len(f.Progs))
	}
	if got := value(res[0], "e_entry"); got != int64(f.Entry) {
		t.Errorf("e_entry %v, want %x", got, f.Entry)
	}
	if got := value(res[1], "p_type"); got != int64(f.Progs[0].Type) {
		t.Errorf("p_type %v, want %v", got, f.Progs[0].Type)
	}
	if buffer(t, []byte("PK\x03\x04")) != nil && a.Detect(buffer(t, []byte("PK\x03\x04")), nil) {
		t.Error("a zip archive is no ELF")
	}
}

func TestZip(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range []string{"a.txt", "b.txt"} {
		f, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte("content of " + name))
	}
	w.SetComment("hi")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b := buffer(t, buf.Bytes())
	a := zipa.New()
	if !a.Detect(b, nil) {
		t.Fatal("not detected")
	}
	res, err := a.Parse(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range res {
		names = append(names, r.Name)
	}
	want := "[ZIP_HEADER ZIP_HEADER(PK78) ZIP_HEADER ZIP_HEADER(PK78) ZIP_CENTRAL_HEADER ZIP_CENTRAL_HEADER ZIP_END_HEADER]"
	if got := fmtNames(names); got != want {
		t.Fatalf("headers %s, want %s", got, want)
	}
	if v := value(res[0], "filename"); v != "a.txt" {
		t.Errorf("filename %v", v)
	}
	if v := value(res[len(res)-1], "comment"); v != "hi" {
		t.Errorf("comment %v", v)
	}
}

func fmtNames(s []string) string {
	out := "["
	for i, n := range s {
		if i > 0 {
			out += " "
		}
		out += n
	}
	return out + "]"
}
