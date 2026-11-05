package hex

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Buffer is a part of a file in memory (from an offset, of a length), with
// its changes, which can be undone, until it is written. Addresses are those
// of the file: from the offset to the offset plus the size. It is safe for
// concurrent use.
type Buffer struct {
	mu       sync.Mutex
	path     string
	offset   int64
	data     []byte
	origSize int
	mtime    time.Time
	modified bool
	// changed are the addresses changed (the highlight of the UI).
	changed map[int64]bool
	// history and future are the edits to undo and to redo, a step each.
	history [][]edit
	future  [][]edit
}

// edit replaces Old at Addr with New (an insertion has no Old, a removal no
// New); its undo is the reverse.
type edit struct {
	Addr     int64
	Old, New []byte
}

// NewBuffer returns an empty buffer.
func NewBuffer() *Buffer { return &Buffer{changed: map[int64]bool{}} }

// Open reads length bytes of path from offset (0: to the end; less when the
// file ends before). A file that does not exist is empty.
func (b *Buffer) Open(path string, offset, length int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	b.path, b.offset, b.data, b.origSize = abs, offset, nil, 0
	b.mtime, b.modified = time.Time{}, false
	b.changed = map[int64]bool{}
	b.history, b.future = nil, nil
	f, err := os.Open(abs)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("%s is a directory", abs)
	}
	b.mtime = st.ModTime()
	size := st.Size()
	if length <= 0 || offset+length > size {
		length = size - offset
	}
	if length <= 0 {
		return nil
	}
	buf := make([]byte, length)
	n, err := f.ReadAt(buf, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	b.data = buf[:n]
	b.origSize = n
	return nil
}

// Path is the absolute path of the file.
func (b *Buffer) Path() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.path
}

// Offset is the address of the first byte.
func (b *Buffer) Offset() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.offset
}

// Size is the number of bytes.
func (b *Buffer) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.data)
}

// Modified says whether it has changes not written.
func (b *Buffer) Modified() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.modified
}

// Changed are the addresses changed.
func (b *Buffer) Changed() map[int64]bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return maps.Clone(b.changed)
}

// Byte is the byte at addr.
func (b *Buffer) Byte(addr int64) (byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := addr - b.offset
	if i < 0 || i >= int64(len(b.data)) {
		return 0, false
	}
	return b.data[i], true
}

// Bytes are (a copy of) at most n bytes from addr.
func (b *Buffer) Bytes(addr int64, n int) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.slice(addr, n))
}

// slice is the bytes from addr, at most n, not copied.
func (b *Buffer) slice(addr int64, n int) []byte {
	i := addr - b.offset
	if i < 0 || n <= 0 || i >= int64(len(b.data)) {
		return nil
	}
	return b.data[i:min(int64(len(b.data)), i+int64(n))]
}

// View calls fn with the bytes (not to be kept nor changed).
func (b *Buffer) View(fn func(offset int64, data []byte)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	fn(b.offset, b.data)
}

func (b *Buffer) checkAddr(addr int64, end bool) error {
	i := addr - b.offset
	if i < 0 || i > int64(len(b.data)) || !end && i == int64(len(b.data)) {
		return fmt.Errorf("address 0x%x is out of the buffer", addr)
	}
	return nil
}

// do applies the edits as one step to undo.
func (b *Buffer) do(edits []edit) {
	for _, e := range edits {
		b.apply(e)
	}
	b.history = append(b.history, edits)
	b.future = nil
	b.modified = true
}

// apply replaces e.Old (its length) at e.Addr with e.New, and marks the
// addresses changed: those of e.New, and none after it when the size
// changed (they moved).
func (b *Buffer) apply(e edit) {
	i := int(e.Addr - b.offset)
	end := min(i+len(e.Old), len(b.data))
	if len(e.New) != end-i {
		for a := range b.changed {
			if a >= e.Addr {
				delete(b.changed, a)
			}
		}
	}
	b.data = slices.Concat(b.data[:i], e.New, b.data[end:])
	for k := range e.New {
		b.changed[e.Addr+int64(k)] = true
	}
}

// Insert inserts data before addr (or at the end).
func (b *Buffer) Insert(addr int64, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.checkAddr(addr, true); err != nil {
		return err
	}
	if len(data) > 0 {
		b.do([]edit{{Addr: addr, New: slices.Clone(data)}})
	}
	return nil
}

// Change overwrites the bytes from addr with data (the buffer grows when
// it goes past the end).
func (b *Buffer) Change(addr int64, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.checkAddr(addr, true); err != nil {
		return err
	}
	if len(data) > 0 {
		old := slices.Clone(b.slice(addr, len(data)))
		b.do([]edit{{Addr: addr, Old: old, New: slices.Clone(data)}})
	}
	return nil
}

// Remove removes n bytes from addr and returns them.
func (b *Buffer) Remove(addr int64, n int) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.checkAddr(addr, false); err != nil {
		return nil, err
	}
	old := slices.Clone(b.slice(addr, n))
	if len(old) != n {
		return nil, fmt.Errorf("0x%x bytes from 0x%x are out of the buffer", n, addr)
	}
	b.do([]edit{{Addr: addr, Old: old}})
	return old, nil
}

// Substitute replaces each pattern (-1: any byte) from addr, before addr+n,
// with repl: one step to undo. It returns how many were replaced.
func (b *Buffer) Substitute(addr int64, n int, pattern []int, repl []byte) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(pattern) == 0 {
		return 0
	}
	var edits []edit
	pos, end := addr, addr+int64(n)
	for pos < end {
		at := b.search(pos, pattern, true)
		if at < 0 || at+int64(len(pattern)) > end {
			break
		}
		e := edit{Addr: at, Old: slices.Clone(b.slice(at, len(pattern))), New: slices.Clone(repl)}
		b.apply(e)
		edits = append(edits, e)
		end += int64(len(repl) - len(pattern))
		pos = at + int64(max(len(repl), 1))
	}
	if len(edits) > 0 {
		b.history = append(b.history, edits)
		b.future = nil
		b.modified = true
	}
	return len(edits)
}

// Resize makes the buffer n bytes long (zeros after the end): it cannot be
// undone.
func (b *Buffer) Resize(n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n < len(b.data) {
		b.data = b.data[:n:n]
	} else {
		b.data = append(b.data, make([]byte, n-len(b.data))...)
	}
	b.history, b.future = nil, nil
	clear(b.changed)
	b.modified = true
}

func reverse(edits []edit) []edit {
	out := make([]edit, len(edits))
	for i, e := range edits {
		out[len(edits)-1-i] = edit{Addr: e.Addr, Old: e.New, New: e.Old}
	}
	return out
}

// Undo undoes the last step and says how many are left.
func (b *Buffer) Undo() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.history) == 0 {
		return 0
	}
	step := b.history[len(b.history)-1]
	b.history = b.history[:len(b.history)-1]
	b.unmark(step)
	for _, e := range reverse(step) {
		b.apply(e)
	}
	b.unmark(reverse(step))
	b.future = append(b.future, step)
	b.modified = len(b.history) > 0
	return len(b.history)
}

// Redo does again the last step undone and says how many are left.
func (b *Buffer) Redo() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.future) == 0 {
		return 0
	}
	step := b.future[len(b.future)-1]
	b.future = b.future[:len(b.future)-1]
	for _, e := range step {
		b.apply(e)
	}
	b.history = append(b.history, step)
	b.modified = true
	return len(b.future)
}

func (b *Buffer) unmark(edits []edit) {
	for _, e := range edits {
		for k := range e.New {
			delete(b.changed, e.Addr+int64(k))
		}
	}
}

// Search is the address of the first pattern (-1: any byte) from addr
// forward, or from addr backward; -1 if there is none.
func (b *Buffer) Search(addr int64, pattern []int, forward bool) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.search(addr, pattern, forward)
}

func (b *Buffer) search(addr int64, pattern []int, forward bool) int64 {
	n := len(pattern)
	if n == 0 || n > len(b.data) {
		return -1
	}
	match := func(i int) bool {
		for k, p := range pattern {
			if p >= 0 && b.data[i+k] != byte(p) {
				return false
			}
		}
		return true
	}
	i := int(addr - b.offset)
	if forward {
		for i = max(i, 0); i+n <= len(b.data); i++ {
			if match(i) {
				return b.offset + int64(i)
			}
		}
		return -1
	}
	for i = min(i, len(b.data)-n); i >= 0; i-- {
		if match(i) {
			return b.offset + int64(i)
		}
	}
	return -1
}

// Write writes the buffer to path (that of the file when it is ""), at its
// offset: the rest of the file follows it when the size changed.
func (b *Buffer) Write(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if path == "" {
		path = b.path
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	data := b.data
	if len(b.data) != b.origSize {
		// The rest of the file moves.
		from := b.offset + int64(b.origSize)
		if from < st.Size() {
			rest := make([]byte, st.Size()-from)
			if _, err := f.ReadAt(rest, from); err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			data = slices.Concat(b.data, rest)
		}
		if err := f.Truncate(b.offset + int64(len(data))); err != nil {
			return err
		}
	}
	if _, err := f.WriteAt(data, b.offset); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if st, err := f.Stat(); err == nil && path == b.path {
		b.mtime = st.ModTime()
	}
	b.origSize = len(b.data)
	b.modified = false
	return nil
}

// ChangedOutside says whether the file changed since it was read or
// written (a newer time, or it was removed).
func (b *Buffer) ChangedOutside() (bool, string) {
	b.mu.Lock()
	path, mtime := b.path, b.mtime
	b.mu.Unlock()
	if path == "" || mtime.IsZero() {
		return false, ""
	}
	st, err := os.Stat(path)
	if err != nil {
		return true, path + " was removed"
	}
	if st.ModTime().After(mtime) {
		b.mu.Lock()
		b.mtime = st.ModTime()
		b.mu.Unlock()
		return true, fmt.Sprintf("%s was changed outside at %s", path, st.ModTime().Format(time.DateTime))
	}
	return false, ""
}

// Equal says whether the buffer has these bytes (for the tests).
func (b *Buffer) Equal(data []byte) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Equal(b.data, data)
}
