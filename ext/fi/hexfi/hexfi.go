// Package hexfi is the sources and the kind of goda-fi.vim
// (https://github.com/Shougo/goda-fi.vim) of goda-hex.vim, as ddx.vim's ddu
// sources and kind: the sources "hex_analyze" (the values of the analyzer),
// "hex_diff" (the differences with the option "anotherPath") and
// "hex_string" (the strings of the file) of the session of the buffer the
// finder is started from, and the kind "hex" of their items (open: go to
// the address, edit: change the value).
//
// Add them in your configuration of goda-fi.vim:
//
//	import "github.com/Shougo/goda-hex.vim/ext/fi/hexfi"
//
//	b.AddSource(hexfi.NewAnalyzeSource())
//	b.AddSource(hexfi.NewDiffSource())
//	b.AddSource(hexfi.NewStringSource(hexfi.StringParams{}))
//	b.AddKind(hexfi.NewKind())
package hexfi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Shougo/goda-fi.vim/fi"
	"github.com/Shougo/goda-hex.vim/hex"
	"github.com/Shougo/goda.vim/sdk"
)

// KindName is the name of the kind of the items.
const KindName = "hex"

// ActionData is the action of an item: the session and the value.
type ActionData struct {
	Name  string           `json:"name"`
	Value hex.AnalyzeValue `json:"value"`
}

// sessionName is the session of goda-hex of the buffer the finder is
// started from.
func sessionName(ctx context.Context, h sdk.Host, a fi.GatherArgs) (string, error) {
	if a.Context == nil {
		return "", nil
	}
	v, err := h.Call(ctx, "getbufvar", a.Context.BufNr, "goda_hex_name", "")
	if err != nil {
		return "", err
	}
	s, _ := v.(string)
	return s, nil
}

func call[T any](ctx context.Context, h sdk.Host, out *T, fn string, args ...any) error {
	v, err := h.Call(ctx, fn, args...)
	if err != nil {
		return err
	}
	return sdk.Decode(v, out)
}

type analyzeSource struct{}

// NewAnalyzeSource returns the source "hex_analyze".
func NewAnalyzeSource() fi.Source { return analyzeSource{} }

func (analyzeSource) Name() string { return "hex_analyze" }
func (analyzeSource) Kind() string { return KindName }

func (analyzeSource) Gather(ctx context.Context, h sdk.Host, a fi.GatherArgs, emit func([]fi.Item) error) error {
	name, err := sessionName(ctx, h, a)
	if err != nil || name == "" {
		return err
	}
	var results []hex.AnalyzeResult
	if err := call(ctx, h, &results, "goda_hex#analyze", name); err != nil {
		return err
	}
	var items []fi.Item
	for _, r := range results {
		items = append(items, fi.Item{Word: r.Name})
		for _, v := range r.Values {
			items = append(items, fi.Item{Word: "  " + v.Name + ": " + FormatValue(v),
				Action: ActionData{Name: name, Value: v}})
		}
	}
	return emit(items)
}

// FormatValue is the value as the sources show it: "42 (0x2a)".
func FormatValue(v hex.AnalyzeValue) string {
	if v.RawType == "integer" {
		var n int64
		if sdk.Decode(v.Value, &n) == nil {
			if n < 0 {
				return fmt.Sprintf("%d (0x%x)", n, uint64(n))
			}
			return fmt.Sprintf("%d (0x%x)", n, n)
		}
	}
	return hex.Sanitize(fmt.Sprint(v.Value))
}

type diffSource struct{}

// NewDiffSource returns the source "hex_diff".
func NewDiffSource() fi.Source { return diffSource{} }

func (diffSource) Name() string { return "hex_diff" }
func (diffSource) Kind() string { return KindName }

func formatBytes(b []int) string {
	if len(b) == 0 {
		return "(none)"
	}
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = fmt.Sprintf("0x%02x", c)
	}
	return strings.Join(parts, " ")
}

func (diffSource) Gather(ctx context.Context, h sdk.Host, a fi.GatherArgs, emit func([]fi.Item) error) error {
	name, err := sessionName(ctx, h, a)
	if err != nil || name == "" {
		return err
	}
	var diffs []hex.Diff
	if err := call(ctx, h, &diffs, "goda_hex#get_diff", name); err != nil {
		return err
	}
	items := make([]fi.Item, 0, len(diffs))
	for _, d := range diffs {
		text := fmt.Sprintf("0x%x: %s %s -> %s", d.Address, d.Type, formatBytes(d.Old), formatBytes(d.New))
		little := true
		v := hex.AnalyzeValue{Name: text, RawType: "integer", Size: 1, IsLittle: &little, Address: d.Address}
		if len(d.Old) > 0 {
			v.Value = int64(d.Old[0])
		} else {
			v.Value = int64(0)
		}
		items = append(items, fi.Item{Word: text, Action: ActionData{Name: name, Value: v}})
	}
	return emit(items)
}

// StringParams are the params of the source "hex_string".
type StringParams struct {
	// MinLength of the strings. Default 5.
	MinLength int `json:"minLength"`
	// Encoding of the strings (ascii, utf-8, utf-16le, utf-16be). Default
	// that of the UI when it is one of them, or utf-8.
	Encoding string `json:"encoding"`
}

type stringSource struct{ p StringParams }

// NewStringSource returns the source "hex_string" with p as the default
// params.
func NewStringSource(p StringParams) fi.Source {
	if p.MinLength <= 0 {
		p.MinLength = 5
	}
	return stringSource{p}
}

func (stringSource) Name() string { return "hex_string" }
func (stringSource) Kind() string { return KindName }

func (s stringSource) Gather(ctx context.Context, h sdk.Host, a fi.GatherArgs, emit func([]fi.Item) error) error {
	p := s.p
	if err := fi.Decode(a.SourceParams, &p); err != nil {
		return err
	}
	name, err := sessionName(ctx, h, a)
	if err != nil || name == "" {
		return err
	}
	if p.Encoding == "" {
		var enc string
		if err := call(ctx, h, &enc, "getbufvar", a.Context.BufNr, "goda_hex_encoding", ""); err != nil {
			return err
		}
		switch enc {
		case hex.ASCII, hex.UTF16LE, hex.UTF16BE:
			p.Encoding = enc
		default:
			p.Encoding = hex.UTF8
		}
	}
	var strs []hex.ExtractedString
	if err := call(ctx, h, &strs, "goda_hex#get_strings", name, p.MinLength, p.Encoding); err != nil {
		return err
	}
	items := make([]fi.Item, 0, len(strs))
	for _, st := range strs {
		v := hex.AnalyzeValue{Name: hex.Sanitize(st.Text), RawType: "string", Value: st.Text,
			Size: len(mustEncode(st.Text, st.Encoding)), Address: st.Address, Encoding: st.Encoding}
		items = append(items, fi.Item{Word: v.Name, Action: ActionData{Name: name, Value: v}})
	}
	return emit(items)
}

func mustEncode(s, enc string) []byte {
	b, err := hex.EncodeString(s, enc)
	if err != nil {
		return []byte(s)
	}
	return b
}

type kind struct{}

// NewKind returns the kind "hex".
func NewKind() fi.Kind { return kind{} }

func (kind) Name() string { return KindName }

func data(items []fi.Item) []ActionData {
	var out []ActionData
	for _, it := range items {
		var d ActionData
		if it.Action != nil && sdk.Decode(it.Action, &d) == nil && d.Name != "" {
			out = append(out, d)
		}
	}
	return out
}

func (kind) Actions() map[string]fi.Action {
	return map[string]fi.Action{
		"open": {
			Description: "Go to the address.",
			Run: func(ctx context.Context, h sdk.Host, a fi.ActionArgs) (fi.ActionResult, error) {
				for _, d := range data(a.Items) {
					if _, err := h.Call(ctx, "goda_hex#jump", d.Name, d.Value.Address); err != nil {
						return fi.ActionResult{}, err
					}
				}
				return fi.ActionResult{}, nil
			},
		},
		"edit": {
			Description: "Change the value.",
			Run: func(ctx context.Context, h sdk.Host, a fi.ActionArgs) (fi.ActionResult, error) {
				names := map[string]bool{}
				for _, d := range data(a.Items) {
					cur := fmt.Sprint(d.Value.Value)
					var in string
					if err := call(ctx, h, &in, "goda_hex#util#input",
						fmt.Sprintf("New value: %s -> ", cur), cur); err != nil {
						return fi.ActionResult{}, err
					}
					if strings.TrimSpace(in) == "" {
						return fi.ActionResult{Flags: fi.Persist}, nil
					}
					var nv any = in
					if d.Value.RawType == "integer" {
						n, err := strconv.ParseInt(strings.TrimSpace(in), 0, 64)
						if err != nil {
							return fi.ActionResult{Flags: fi.Persist}, errors.New("invalid value: " + in)
						}
						nv = n
					}
					if _, err := h.Call(ctx, "goda_hex#change", d.Name, d.Value, nv); err != nil {
						return fi.ActionResult{}, err
					}
					names[d.Name] = true
				}
				for name := range names {
					if _, err := h.Call(ctx, "goda_hex#redraw", name); err != nil {
						return fi.ActionResult{}, err
					}
				}
				return fi.ActionResult{Flags: fi.Persist | fi.RefreshItems}, nil
			},
		},
	}
}
