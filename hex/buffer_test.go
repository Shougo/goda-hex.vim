package hex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func open(t *testing.T, content string, offset, length int64) (*Buffer, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "f.bin")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	b := NewBuffer()
	if err := b.Open(path, offset, length); err != nil {
		t.Fatal(err)
	}
	return b, path
}

func check(t *testing.T, b *Buffer, want string) {
	t.Helper()
	if got := string(b.Bytes(b.Offset(), b.Size())); got != want {
		t.Fatalf("buffer %q, want %q", got, want)
	}
}

func TestEditUndoRedo(t *testing.T) {
	b, _ := open(t, "abcdef", 0, 0)
	if err := b.Change(1, []byte("XY")); err != nil {
		t.Fatal(err)
	}
	check(t, b, "aXYdef")
	if c := b.Changed(); len(c) != 2 || !c[1] || !c[2] {
		t.Errorf("changed %v", c)
	}
	if err := b.Insert(6, []byte("!")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Remove(0, 1); err != nil {
		t.Fatal(err)
	}
	check(t, b, "XYdef!")
	if !b.Modified() {
		t.Error("not modified")
	}
	b.Undo()
	check(t, b, "aXYdef!")
	b.Undo()
	b.Undo()
	check(t, b, "abcdef")
	if b.Modified() {
		t.Error("modified after undoing all")
	}
	b.Redo()
	check(t, b, "aXYdef")
	if _, err := b.Remove(5, 2); err == nil {
		t.Error("a removal past the end")
	}
	if err := b.Change(5, []byte("FGH")); err != nil {
		t.Fatal(err)
	}
	check(t, b, "aXYdeFGH")
}

func TestSearchSubstitute(t *testing.T) {
	b, _ := open(t, strings.Repeat("-", 100)+"xxabyabzab", 100, 0)
	b2, _ := open(t, "0123456789xxabyabzab", 10, 0)
	if b2.Offset() != 10 || b2.Size() != 10 || string(b2.Bytes(10, 2)) != "xx" {
		t.Fatalf("offset %d size %d", b2.Offset(), b2.Size())
	}
	if at := b.Search(100, []int{'a', -1, 'y'}, true); at != 102 {
		t.Errorf("forward %d", at)
	}
	if at := b.Search(109, []int{'a', 'b'}, false); at != 108 {
		t.Errorf("backward %d", at)
	}
	if at := b.Search(100, []int{'q'}, true); at != -1 {
		t.Errorf("missing %d", at)
	}
	if n := b.Substitute(100, 10, []int{'a', 'b'}, []byte("C")); n != 3 {
		t.Errorf("substituted %d", n)
	}
	check(t, b, "xxCyCzC")
	b.Undo()
	check(t, b, "xxabyabzab")
}

func TestWriteResized(t *testing.T) {
	b, path := open(t, "0123456789", 2, 4)
	check(t, b, "2345")
	if err := b.Insert(4, []byte("ab")); err != nil {
		t.Fatal(err)
	}
	if err := b.Write(""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "0123ab456789" {
		t.Fatalf("file %q", got)
	}
	if _, err := b.Remove(2, 4); err != nil {
		t.Fatal(err)
	}
	if err := b.Write(""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "01456789" {
		t.Fatalf("file %q", got)
	}
	if b.Modified() {
		t.Error("modified after a write")
	}
	b.Resize(4)
	check(t, b, "45\x00\x00")
}

func TestOpenMissing(t *testing.T) {
	b := NewBuffer()
	if err := b.Open(filepath.Join(t.TempDir(), "none"), 0, 0); err != nil || b.Size() != 0 {
		t.Fatalf("%v %d", err, b.Size())
	}
	if err := b.Insert(0, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if err := b.Write(""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(b.Path()); string(got) != "new" {
		t.Fatalf("file %q", got)
	}
}
