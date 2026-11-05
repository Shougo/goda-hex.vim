// Package hex is the UI "hex" of goda-hex.vim, as ddx-ui-hex: the bytes of
// the file, sixteen a line, with their address and their text.
package hex

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	core "github.com/Shougo/goda-hex.vim/hex"
	"github.com/Shougo/goda.vim/sdk"
)

// Name is the name of the UI.
const Name = "hex"

// width is the number of bytes of a line.
const width = 16

// Params are the params of the UI, as ddx-ui-hex's.
type Params struct {
	// Encoding of the text of the bytes: "utf-8" or "cp932".
	Encoding string `json:"encoding"`
	// FloatingBorder is the border of a floating window (Neovim).
	FloatingBorder any `json:"floatingBorder"`
	// Highlights of the parts (ascii, changed, control, cursorAscii,
	// diffAdd, diffChange, diffDelete, escape, floating, newLine, null,
	// selected, tab).
	Highlights map[string]string `json:"highlights"`
	// OverwriteStatusline sets 'statusline' of the window.
	OverwriteStatusline bool `json:"overwriteStatusline"`
	// OverwriteTitle sets 'titlestring' (as when 'laststatus' is 0).
	OverwriteTitle bool `json:"overwriteTitle"`
	// Split: "horizontal", "vertical", "floating" (Neovim) or "no".
	Split          string `json:"split"`
	SplitDirection string `json:"splitDirection"`
	WinCol         int    `json:"winCol"`
	WinHeight      int    `json:"winHeight"`
	WinRow         int    `json:"winRow"`
	WinWidth       int    `json:"winWidth"`
}

// DefaultParams are ddx-ui-hex's.
func DefaultParams() Params {
	return Params{
		Encoding:            core.UTF8,
		FloatingBorder:      "none",
		Highlights:          map[string]string{},
		OverwriteStatusline: true,
		Split:               "horizontal",
		SplitDirection:      "botright",
		WinHeight:           20,
	}
}

// defaultHighlights are those of the parts not set.
var defaultHighlights = map[string]string{
	"changed": "ErrorMsg", "cursorAscii": "Search", "diffAdd": "DiffAdd",
	"diffChange": "DiffChange", "diffDelete": "DiffDelete", "floating": "NormalFloat",
	"selected": "Visual",
}

func params(a core.Args) (Params, error) {
	p, err := core.DecodeParams(a.Params, DefaultParams())
	if err != nil {
		return p, err
	}
	hl := map[string]string{}
	for k, v := range defaultHighlights {
		hl[k] = v
	}
	for k, v := range p.Highlights {
		hl[k] = v
	}
	p.Highlights = hl
	if p.Encoding == "" {
		p.Encoding = core.UTF8
	}
	return p, nil
}

type ui struct{}

// New returns the UI.
func New() core.UI { return ui{} }

func (ui) Name() string { return Name }

func (ui) New() core.Instance { return &instance{selected: -1} }

// instance is the UI of a session.
type instance struct {
	mu sync.Mutex
	// selected is the address selected (selectAddress), -1 for none.
	selected int64
	// saved are the bytes of copy and remove, for paste.
	saved []byte
	// search is the pattern of the last search.
	search []int
	bufnr  int
	// lines are those drawn: a redraw sends those that changed.
	lines []string
}

func (in *instance) state() (selected int64, saved []byte, search []int, bufnr int) {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.selected, in.saved, in.search, in.bufnr
}

func (in *instance) set(fn func(in *instance)) {
	in.mu.Lock()
	defer in.mu.Unlock()
	fn(in)
}

// render is what goda_hex#ui#hex#_redraw() draws.
type render struct {
	Name  string   `json:"name"`
	Lines []string `json:"lines"`
	// Changes are the lines changed [lnum, line] when Lines is null (the
	// same number of lines as drawn before).
	Changes    [][]any    `json:"changes"`
	Highlights [][]any    `json:"highlights"`
	Params     Params     `json:"params"`
	Offset     int64      `json:"offset"`
	Size       int        `json:"size"`
	Length     int64      `json:"length"`
	Modified   bool       `json:"modified"`
	AddrWidth  int        `json:"addrWidth"`
	PrevBufNr  int        `json:"prevBufnr"`
	PrevWinID  int        `json:"prevWinid"`
	Status     statusInfo `json:"status"`
}

type statusInfo struct {
	Name   string `json:"name"`
	Offset int64  `json:"offset"`
	Length int64  `json:"length"`
	Size   int    `json:"size"`
}

// addrWidth is the number of hex digits of the addresses of the lines.
func addrWidth(last int64) int {
	return max(8, len(strconv.FormatInt(max(last, 0), 16)))
}

// Lines are the lines of the bytes from offset, and the highlights of the
// bytes [lnum, col (0-based), length, group] (Exported for the tests.)
func Lines(data []byte, offset int64, encoding string, hl func(addr int64, b byte) string) ([]string, [][]any, error) {
	w := addrWidth(offset + int64(len(data)) - 1)
	if len(data) == 0 {
		return []string{fmt.Sprintf("%0*x: ", w, offset)}, nil, nil
	}
	lines := make([]string, 0, (len(data)+width-1)/width)
	var hls [][]any
	var sb strings.Builder
	for i := 0; i < len(data); i += width {
		row := data[i:min(i+width, len(data))]
		addr := offset + int64(i)
		sb.Reset()
		fmt.Fprintf(&sb, "%0*x: ", w, addr)
		for k, b := range row {
			if k > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(hexByte[b])
		}
		sb.WriteString(strings.Repeat("   ", width-len(row)))
		sb.WriteString(" |   ")
		text, err := core.Chars(row, encoding)
		if err != nil {
			return nil, nil, err
		}
		sb.WriteString(text)
		lines = append(lines, sb.String())
		if hl != nil {
			for k, b := range row {
				if g := hl(addr+int64(k), b); g != "" {
					hls = append(hls, []any{len(lines), w + 2 + 3*k, 2, g})
				}
			}
		}
	}
	return lines, hls, nil
}

var hexByte = func() (t [256]string) {
	for i := range t {
		t[i] = fmt.Sprintf("%02x", i)
	}
	return
}()

func (in *instance) Redraw(ctx context.Context, h sdk.Host, a core.Args) error {
	p, err := params(a)
	if err != nil {
		return err
	}
	selected, _, _, _ := in.state()
	changed := a.Buffer.Changed()
	diffChanged, diffDeleted := map[int64]bool{}, map[int64]bool{}
	diffAdded := map[int64]bool{}
	if a.Options.AnotherPath != "" {
		for _, d := range core.Diffs(a.Buffer, a.Another) {
			switch d.Type {
			case "changed":
				diffChanged[d.Address] = true
			case "deleted":
				for k := range d.Old {
					diffDeleted[d.Address+int64(k)] = true
				}
			case "added":
				diffAdded[d.Address] = true
			}
		}
	}
	g := p.Highlights
	hl := func(addr int64, b byte) string {
		switch {
		case addr == selected:
			return g["selected"]
		case changed[addr]:
			return g["changed"]
		case diffAdded[addr]:
			return g["diffAdd"]
		case diffChanged[addr]:
			return g["diffChange"]
		case diffDeleted[addr]:
			return g["diffDelete"]
		case b == 0x00:
			return g["null"]
		case b == 0x09:
			return g["tab"]
		case b == 0x0a:
			return g["newLine"]
		case b == 0x1b:
			return g["escape"]
		case b < 0x20 || b == 0x7f:
			return g["control"]
		case b < 0x7f:
			return g["ascii"]
		}
		return ""
	}
	var lines []string
	var hls [][]any
	var off int64
	var size int
	a.Buffer.View(func(offset int64, data []byte) {
		off, size = offset, len(data)
		lines, hls, err = Lines(data, offset, p.Encoding, hl)
	})
	if err != nil {
		return err
	}
	if hls == nil {
		hls = [][]any{}
	}
	var changes [][]any
	in.mu.Lock()
	prev := in.lines
	in.lines = lines
	in.mu.Unlock()
	if len(prev) == len(lines) {
		changes = [][]any{}
		for i, l := range lines {
			if prev[i] != l {
				changes = append(changes, []any{i + 1, l})
			}
		}
		lines = nil
	}
	length := core.DefaultLength
	if a.Options.Length != nil {
		length = int(*a.Options.Length)
	}
	r := render{
		Name: a.Options.Name, Lines: lines, Changes: changes, Highlights: hls, Params: p,
		Offset: off, Size: size, Length: int64(length), Modified: a.Buffer.Modified(),
		AddrWidth: addrWidth(off + int64(size) - 1),
		PrevBufNr: a.Context.BufNr, PrevWinID: a.Context.WinID,
		Status: statusInfo{Name: a.Options.Name, Offset: off, Length: int64(length), Size: size},
	}
	v, err := h.Call(ctx, "goda_hex#ui#hex#_redraw", a.Options.Name, r)
	if err != nil {
		return err
	}
	var res struct {
		Bufnr int  `json:"bufnr"`
		Full  bool `json:"full"`
	}
	if err := sdk.Decode(v, &res); err != nil {
		return err
	}
	in.set(func(in *instance) { in.bufnr = res.Bufnr })
	if r.Lines == nil && res.Full {
		// A new buffer (or one unloaded): it has all of them.
		in.set(func(in *instance) { in.lines = nil })
		return in.Redraw(ctx, h, a)
	}
	return nil
}

func (in *instance) Jump(ctx context.Context, h sdk.Host, a core.Args, address int64) error {
	_, err := h.Call(ctx, "goda_hex#ui#hex#_jump", a.Options.Name, address, a.Buffer.Offset())
	return err
}

// address is the address under the cursor.
func address(ctx context.Context, h sdk.Host) (int64, error) {
	v, err := h.Call(ctx, "goda_hex#ui#hex#_get_current_address")
	if err != nil {
		return 0, err
	}
	var r []any
	if err := sdk.Decode(v, &r); err != nil || len(r) != 2 {
		return 0, errors.New("invalid address")
	}
	var addr int64
	if err := sdk.Decode(r[1], &addr); err != nil || r[0] == "" {
		return 0, errors.New("invalid address")
	}
	return addr, nil
}

// rng is the range of the selection to addr, or addr alone.
func (in *instance) rng(addr int64) (start int64, n int, isRange bool) {
	selected, _, _, _ := in.state()
	if selected >= 0 && selected != addr {
		return min(selected, addr), int(max(selected, addr)-min(selected, addr)) + 1, true
	}
	return addr, 1, false
}

func (in *instance) unselect() { in.set(func(in *instance) { in.selected = -1 }) }

// TypeParams are the params of the actions that take a value.
type TypeParams struct {
	// Type: "hex" (default), "string", "number", "floating".
	Type string `json:"type"`
	// IsLittle: little endian (default).
	IsLittle *bool `json:"isLittle"`
	IsSigned bool  `json:"isSigned"`
	// Size of a number (default 4).
	Size int `json:"size"`
	// Direction of a search: "forward" (default), "backward".
	Direction string `json:"direction"`
	// Method of checksum.
	Method string `json:"method"`
	// Path of save.
	Path string `json:"path"`
}

func typeParams(ap core.Params) (TypeParams, error) {
	tp, err := core.DecodeParams(ap, TypeParams{Type: "hex", Size: 4, Direction: "forward", Method: "sum"})
	if tp.IsLittle == nil {
		t := true
		tp.IsLittle = &t
	}
	return tp, err
}

func input(ctx context.Context, h sdk.Host, prompt, text string) (string, error) {
	v, err := h.Call(ctx, "goda_hex#util#input", prompt, text)
	if err != nil {
		return "", err
	}
	s, _ := v.(string)
	return s, nil
}

// inputValue asks for a value of the type (prefix: "New", "Search"): its
// bytes, nil when nothing was typed.
func inputValue(ctx context.Context, h sdk.Host, tp TypeParams, enc, prefix string) ([]int, error) {
	prompt := map[string]string{"hex": " value: 0x", "string": " string: ", "number": " number: ",
		"floating": " floating: "}[tp.Type]
	if prompt == "" {
		return nil, fmt.Errorf("invalid type: %q (hex, string, number, floating)", tp.Type)
	}
	raw, err := input(ctx, h, prefix+prompt, "")
	if err != nil || raw == "" {
		return nil, err
	}
	return Value(raw, tp, enc)
}

// Value is the bytes of raw, a value of tp ("??" of hex: any byte).
func Value(raw string, tp TypeParams, enc string) ([]int, error) {
	little := tp.IsLittle == nil || *tp.IsLittle
	switch tp.Type {
	case "string":
		b, err := core.EncodeString(raw, enc)
		return core.Pattern(b), err
	case "number":
		n, ok := new(big.Int).SetString(strings.TrimSpace(raw), 0)
		if !ok {
			return nil, fmt.Errorf("not an integer: %q", raw)
		}
		b, err := core.IntBytes(n, tp.Size, little, tp.IsSigned)
		return core.Pattern(b), err
	case "floating":
		f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return nil, fmt.Errorf("not a number: %q", raw)
		}
		size := tp.Size
		if size != 8 {
			size = 4
		}
		b, err := core.FloatBytes(f, size, little)
		return core.Pattern(b), err
	}
	v := core.ParseHex(raw)
	if v == nil {
		return nil, fmt.Errorf("invalid hex value: %q", raw)
	}
	return v, nil
}

func setModified(ctx context.Context, h sdk.Host, bufnr int, b *core.Buffer) error {
	if bufnr <= 0 {
		return nil
	}
	_, err := h.Call(ctx, "setbufvar", bufnr, "&modified", b.Modified())
	return err
}

func echo(ctx context.Context, h sdk.Host, msg string) error {
	_, err := h.Call(ctx, "goda_hex#util#print", msg)
	return err
}

// action is an action at the address under the cursor.
func (in *instance) action(fn func(ctx context.Context, h sdk.Host, a core.Args, tp TypeParams, addr int64) (core.ActionFlags, error)) core.Action {
	return func(ctx context.Context, h sdk.Host, a core.Args, ap core.Params) (core.ActionFlags, error) {
		tp, err := typeParams(ap)
		if err != nil {
			return core.None, err
		}
		addr, err := address(ctx, h)
		if err != nil {
			return core.None, err
		}
		return fn(ctx, h, a, tp, addr)
	}
}

func (in *instance) Actions() map[string]core.Action {
	return map[string]core.Action{
		"change":        in.action(in.change),
		"checksum":      in.action(in.checksum),
		"copy":          in.action(in.copy),
		"insert":        in.action(in.insert),
		"inspect":       in.action(in.inspect),
		"jump":          in.jump,
		"nextDiff":      in.action(in.nextDiff),
		"nextSearch":    in.action(in.nextSearch),
		"paste":         in.action(in.paste),
		"quit":          in.quit,
		"redo":          in.redo,
		"remove":        in.action(in.remove),
		"resize":        in.resize,
		"save":          in.save,
		"search":        in.action(in.searchAction),
		"selectAddress": in.action(in.selectAddress),
		"substitute":    in.action(in.substitute),
		"undo":          in.undo,
		"yank":          in.action(in.yank),
	}
}

func encoding(a core.Args) string {
	p, err := params(a)
	if err != nil {
		return core.UTF8
	}
	return p.Encoding
}

// change overwrites the byte (the selection) with the value typed: a byte
// fills the selection, other values replace it of the same length.
func (in *instance) change(ctx context.Context, h sdk.Host, a core.Args, tp TypeParams, addr int64) (core.ActionFlags, error) {
	v, err := inputValue(ctx, h, tp, encoding(a), "New")
	if err != nil || v == nil {
		return core.None, err
	}
	b, ok := core.Exact(v)
	if !ok {
		return core.None, errors.New("invalid value: no ?? here")
	}
	start, n, isRange := in.rng(addr)
	switch {
	case len(b) == 1:
		b = []byte(strings.Repeat(string(b), n))
	case isRange && len(b) != n:
		return core.None, fmt.Errorf("length mismatch (expected %d bytes)", n)
	}
	if err := a.Buffer.Change(start, b); err != nil {
		return core.None, err
	}
	in.unselect()
	return core.Redraw, nil
}

func (in *instance) checksum(ctx context.Context, h sdk.Host, a core.Args, tp TypeParams, addr int64) (core.ActionFlags, error) {
	start, n, isRange := in.rng(addr)
	if !isRange {
		return core.None, errors.New("the address is not selected")
	}
	sum, err := Checksum(a.Buffer.Bytes(start, n), tp.Method)
	if err != nil {
		return core.None, err
	}
	return core.None, echo(ctx, h, fmt.Sprintf("Checksum %s: %s", tp.Method, sum))
}

// Checksum of data by method: sum, md5, sha-1, sha-256, crc-8, crc-16,
// crc-32.
func Checksum(data []byte, method string) (string, error) {
	switch method {
	case "", "sum":
		var s byte
		for _, b := range data {
			s += b
		}
		return strconv.Itoa(int(s)), nil
	case "md5":
		s := md5.Sum(data)
		return hex.EncodeToString(s[:]), nil
	case "sha-1":
		s := sha1.Sum(data)
		return hex.EncodeToString(s[:]), nil
	case "sha-256":
		s := sha256.Sum256(data)
		return hex.EncodeToString(s[:]), nil
	case "crc-8":
		var crc byte
		for _, b := range data {
			crc ^= b
			for range 8 {
				if crc&0x80 != 0 {
					crc = crc<<1 ^ 0x07
				} else {
					crc <<= 1
				}
			}
		}
		return fmt.Sprintf("%02X", crc), nil
	case "crc-16":
		crc := uint16(0xffff)
		for _, b := range data {
			crc ^= uint16(b) << 8
			for range 8 {
				if crc&0x8000 != 0 {
					crc = crc<<1 ^ 0x1021
				} else {
					crc <<= 1
				}
			}
		}
		return fmt.Sprintf("%04X", crc), nil
	case "crc-32":
		return fmt.Sprintf("%08X", crc32.ChecksumIEEE(data)), nil
	}
	return "", fmt.Errorf("invalid method: %q (sum, md5, sha-1, sha-256, crc-8, crc-16, crc-32)", method)
}

func (in *instance) copy(_ context.Context, _ sdk.Host, a core.Args, _ TypeParams, addr int64) (core.ActionFlags, error) {
	start, n, _ := in.rng(addr)
	b := a.Buffer.Bytes(start, n)
	in.set(func(in *instance) { in.saved, in.selected = b, -1 })
	return core.Redraw, nil
}

func (in *instance) insert(ctx context.Context, h sdk.Host, a core.Args, tp TypeParams, addr int64) (core.ActionFlags, error) {
	v, err := inputValue(ctx, h, tp, encoding(a), "New")
	if err != nil || v == nil {
		return core.None, err
	}
	b, ok := core.Exact(v)
	if !ok {
		return core.None, errors.New("invalid value: no ?? here")
	}
	return core.Redraw, a.Buffer.Insert(addr, b)
}

func (in *instance) inspect(ctx context.Context, h sdk.Host, a core.Args, tp TypeParams, addr int64) (core.ActionFlags, error) {
	start, n, isRange := in.rng(addr)
	little := *tp.IsLittle
	var msg string
	switch tp.Type {
	case "string":
		s, err := core.Chars(a.Buffer.Bytes(start, n), encoding(a))
		if err != nil {
			return core.None, err
		}
		msg = fmt.Sprintf("Chars: %q", s)
	case "number":
		v, err := core.Int(a.Buffer.Bytes(start, tp.Size), tp.Size, little, tp.IsSigned)
		if err != nil {
			return core.None, err
		}
		msg = fmt.Sprintf("Number: %s (0x%x)", v, v)
	case "floating":
		size := tp.Size
		if size != 8 {
			size = 4
		}
		f, err := core.Float(a.Buffer.Bytes(start, size), size, little)
		if err != nil {
			return core.None, err
		}
		msg = fmt.Sprintf("Float: %v", f)
	default:
		if !isRange {
			n = 1
		}
		msg = fmt.Sprintf("Bytes: %s", hexString(a.Buffer.Bytes(start, n)))
	}
	return core.None, echo(ctx, h, msg)
}

func hexString(b []byte) string {
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = hexByte[c]
	}
	return strings.Join(parts, " ")
}

// JumpAddress is the address of the input of jump: "0x{address}",
// "{percentage}%" of the size, "+{offset}" or "-{offset}" from cur.
func JumpAddress(input string, cur int64, offset int64, size int) (int64, error) {
	switch {
	case strings.HasPrefix(input, "0x") || strings.HasPrefix(input, "0X"):
		v, err := strconv.ParseInt(input[2:], 16, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid address: %q", input)
		}
		return v, nil
	case strings.HasSuffix(input, "%"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(input, "%"), 64)
		if err != nil || f < 0 || f > 100 {
			return 0, fmt.Errorf("invalid percentage: %q", input)
		}
		return offset + int64(float64(size)*f/100), nil
	case strings.HasPrefix(input, "+") || strings.HasPrefix(input, "-"):
		v, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid offset: %q", input)
		}
		return cur + v, nil
	}
	return 0, fmt.Errorf("invalid input: %q (0x{address}, {percentage}%%, +{offset}, -{offset})", input)
}

func (in *instance) jump(ctx context.Context, h sdk.Host, a core.Args, _ core.Params) (core.ActionFlags, error) {
	s, err := input(ctx, h, "Jump to the address: ", "0x")
	if err != nil || s == "" || s == "0x" {
		return core.None, err
	}
	cur, _ := address(ctx, h)
	addr, err := JumpAddress(s, cur, a.Buffer.Offset(), a.Buffer.Size())
	if err != nil {
		return core.None, err
	}
	return core.None, in.Jump(ctx, h, a, addr)
}

func (in *instance) nextDiff(ctx context.Context, h sdk.Host, a core.Args, _ TypeParams, addr int64) (core.ActionFlags, error) {
	if a.Options.AnotherPath == "" {
		return core.None, errors.New("the option \"anotherPath\" is needed")
	}
	for _, d := range core.Diffs(a.Buffer, a.Another) {
		if d.Address > addr {
			return core.None, in.Jump(ctx, h, a, d.Address)
		}
	}
	return core.None, errors.New("no more differences")
}

func (in *instance) find(ctx context.Context, h sdk.Host, a core.Args, pattern []int, direction string, addr int64) error {
	forward := direction != "backward"
	from := addr + 1
	if !forward {
		from = addr - 1
	}
	at := a.Buffer.Search(from, pattern, forward)
	if at < 0 {
		return errors.New("not found")
	}
	return in.Jump(ctx, h, a, at)
}

func (in *instance) nextSearch(ctx context.Context, h sdk.Host, a core.Args, tp TypeParams, addr int64) (core.ActionFlags, error) {
	_, _, pattern, _ := in.state()
	if pattern == nil {
		return core.None, nil
	}
	return core.None, in.find(ctx, h, a, pattern, tp.Direction, addr)
}

func (in *instance) paste(_ context.Context, _ sdk.Host, a core.Args, _ TypeParams, addr int64) (core.ActionFlags, error) {
	_, saved, _, _ := in.state()
	return core.Redraw, a.Buffer.Insert(addr, saved)
}

func (in *instance) quit(ctx context.Context, h sdk.Host, a core.Args, _ core.Params) (core.ActionFlags, error) {
	p, err := params(a)
	if err != nil {
		return core.None, err
	}
	_, err = h.Call(ctx, "goda_hex#ui#hex#_quit", a.Options.Name, p.Split, a.Context.BufNr, a.Context.WinID)
	return core.None, err
}

func (in *instance) redo(ctx context.Context, h sdk.Host, a core.Args, _ core.Params) (core.ActionFlags, error) {
	a.Buffer.Redo()
	return core.Redraw, nil
}

func (in *instance) undo(ctx context.Context, h sdk.Host, a core.Args, _ core.Params) (core.ActionFlags, error) {
	a.Buffer.Undo()
	return core.Redraw, nil
}

func (in *instance) remove(_ context.Context, _ sdk.Host, a core.Args, _ TypeParams, addr int64) (core.ActionFlags, error) {
	start, n, _ := in.rng(addr)
	b, err := a.Buffer.Remove(start, n)
	if err != nil {
		return core.None, err
	}
	in.set(func(in *instance) { in.saved, in.selected = b, -1 })
	return core.Redraw, nil
}

func (in *instance) resize(ctx context.Context, h sdk.Host, a core.Args, _ core.Params) (core.ActionFlags, error) {
	s, err := input(ctx, h, "Resize length: ", strconv.Itoa(a.Buffer.Size()))
	if err != nil || s == "" {
		return core.None, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 0, 64)
	if err != nil || n <= 0 {
		return core.None, fmt.Errorf("invalid length: %q", s)
	}
	a.Buffer.Resize(int(n))
	return core.Redraw, nil
}

func (in *instance) save(ctx context.Context, h sdk.Host, a core.Args, ap core.Params) (core.ActionFlags, error) {
	tp, err := typeParams(ap)
	if err != nil {
		return core.None, err
	}
	path := tp.Path
	if path == "" {
		path = a.Buffer.Path()
	} else if !filepath.IsAbs(path) {
		v, err := h.Call(ctx, "getcwd")
		if err != nil {
			return core.None, err
		}
		cwd, _ := v.(string)
		path = filepath.Join(cwd, path)
	}
	if err := a.Buffer.Write(path); err != nil {
		return core.None, fmt.Errorf("save %s: %w", path, err)
	}
	_, _, _, bufnr := in.state()
	if err := setModified(ctx, h, bufnr, a.Buffer); err != nil {
		return core.None, err
	}
	return core.None, echo(ctx, h, fmt.Sprintf("Saved to %q", path))
}

func (in *instance) searchAction(ctx context.Context, h sdk.Host, a core.Args, tp TypeParams, addr int64) (core.ActionFlags, error) {
	v, err := inputValue(ctx, h, tp, encoding(a), "Search")
	if err != nil || v == nil {
		return core.None, err
	}
	in.set(func(in *instance) { in.search = v })
	return core.None, in.find(ctx, h, a, v, tp.Direction, addr)
}

func (in *instance) selectAddress(_ context.Context, _ sdk.Host, _ core.Args, _ TypeParams, addr int64) (core.ActionFlags, error) {
	in.set(func(in *instance) {
		if in.selected >= 0 && in.selected == addr {
			in.selected = -1
		} else {
			in.selected = addr
		}
	})
	return core.Redraw, nil
}

// substitute replaces each value in the selection (all from the address
// without one) with another.
func (in *instance) substitute(ctx context.Context, h sdk.Host, a core.Args, tp TypeParams, addr int64) (core.ActionFlags, error) {
	old, err := inputValue(ctx, h, tp, encoding(a), "Search")
	if err != nil || old == nil {
		return core.None, err
	}
	nv, err := inputValue(ctx, h, tp, encoding(a), "New")
	if err != nil || nv == nil {
		return core.None, err
	}
	repl, ok := core.Exact(nv)
	if !ok {
		return core.None, errors.New("invalid value: no ?? here")
	}
	start, n, isRange := in.rng(addr)
	if !isRange {
		n = int(a.Buffer.Offset() + int64(a.Buffer.Size()) - addr)
	}
	in.unselect()
	if a.Buffer.Substitute(start, n, old, repl) == 0 {
		return core.None, errors.New("not found")
	}
	return core.Redraw, nil
}

func (in *instance) yank(ctx context.Context, h sdk.Host, a core.Args, tp TypeParams, addr int64) (core.ActionFlags, error) {
	start, n, _ := in.rng(addr)
	b := a.Buffer.Bytes(start, n)
	text := hexString(b)
	if tp.Type != "hex" {
		s, err := core.Chars(b, encoding(a))
		if err != nil {
			return core.None, err
		}
		text = s
	}
	if _, err := h.Call(ctx, "goda_hex#ui#hex#_yank", text); err != nil {
		return core.None, err
	}
	in.unselect()
	return core.Redraw, echo(ctx, h, fmt.Sprintf("Yanked %q", text))
}
