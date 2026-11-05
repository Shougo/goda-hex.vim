package hexfi

import (
	"context"
	"fmt"
	"testing"

	"github.com/Shougo/goda-fi.vim/fi"
	"github.com/Shougo/goda.vim/sdk"
)

// host answers as the Vim script of goda-hex, and records the calls.
type host struct {
	sdk.Host
	calls []string
	input string
}

func (h *host) Call(_ context.Context, fn string, args ...any) (any, error) {
	h.calls = append(h.calls, fmt.Sprint(fn, args))
	switch fn {
	case "getbufvar":
		if args[1] == "goda_hex_name" {
			return "s", nil
		}
		return "cp932", nil
	case "goda_hex#analyze":
		return []any{map[string]any{"name": "H", "values": []any{
			map[string]any{"name": "n", "rawType": "integer", "value": int64(42), "size": int64(2), "address": int64(4)},
			map[string]any{"name": "s", "rawType": "string", "value": "a\x00", "address": int64(8)},
		}}}, nil
	case "goda_hex#get_diff":
		return []any{map[string]any{"address": int64(16), "type": "changed", "oldValue": []any{int64(1)}, "newValue": []any{int64(2)}}}, nil
	case "goda_hex#get_strings":
		return []any{map[string]any{"text": "hello", "address": int64(3), "encoding": "utf-8"}}, nil
	case "goda_hex#util#input":
		return h.input, nil
	}
	return nil, nil
}

func gather(t *testing.T, s fi.Source, h *host) []fi.Item {
	t.Helper()
	var out []fi.Item
	err := s.Gather(context.Background(), h, fi.GatherArgs{Context: &fi.Context{BufNr: 3}}, func(items []fi.Item) error {
		out = append(out, items...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSources(t *testing.T) {
	h := &host{}
	items := gather(t, NewAnalyzeSource(), h)
	if len(items) != 3 || items[0].Word != "H" || items[1].Word != "  n: 42 (0x2a)" || items[2].Word != `  s: a\0` {
		t.Errorf("analyze %+v", items)
	}
	items = gather(t, NewDiffSource(), h)
	if len(items) != 1 || items[0].Word != "0x10: changed 0x01 -> 0x02" {
		t.Errorf("diff %+v", items)
	}
	h.calls = nil
	items = gather(t, NewStringSource(StringParams{}), h)
	if len(items) != 1 || items[0].Word != "hello" {
		t.Errorf("string %+v", items)
	}
	// cp932 is not an encoding of the strings: utf-8.
	if last := h.calls[len(h.calls)-1]; last != "goda_hex#get_strings[s 5 utf-8]" {
		t.Errorf("call %s", last)
	}
}

func TestKind(t *testing.T) {
	h := &host{input: "7"}
	items := gather(t, NewAnalyzeSource(), h)
	h.calls = nil
	acts := NewKind().Actions()
	res, err := acts["edit"].Run(context.Background(), h, fi.ActionArgs{Items: items[1:2]})
	if err != nil || res.Flags&fi.RefreshItems == 0 {
		t.Fatalf("%v %v", res, err)
	}
	if fmt.Sprint(h.calls[1:]) != "[goda_hex#change[s {n integer 42 2 <nil> 4 } 7] goda_hex#redraw[s]]" {
		t.Errorf("calls %v", h.calls)
	}
	h.calls = nil
	if _, err := acts["open"].Run(context.Background(), h, fi.ActionArgs{Items: items[2:3]}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(h.calls) != "[goda_hex#jump[s 8]]" {
		t.Errorf("calls %v", h.calls)
	}
}
