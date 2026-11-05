package hex

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/Shougo/goda.vim/sdk"
)

var _ = []sdk.ScriptVersioner{(*Plugin)(nil), brokenPlugin{}}

// The Vim script says the version the binary does (sdk.ScriptVersioner).
func TestScriptVersion(t *testing.T) {
	b, err := os.ReadFile("../autoload/goda_hex.vim")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^let s:version = #\{version: (\d+),`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no s:version in autoload/goda_hex.vim")
	}
	if v, _ := strconv.Atoi(string(m[1])); v != ScriptVersion {
		t.Errorf("s:version is %d, ScriptVersion is %d", v, ScriptVersion)
	}
}

func TestResolve(t *testing.T) {
	n := int64(16)
	b := New().PatchGlobal(Options{UI: "hex", UIParams: map[string]Params{"hex": {"split": "no", "winHeight": 5}}}).
		PatchLocal("x", Options{Length: &n})
	p, err := b.Plugin()
	if err != nil {
		t.Fatal(err)
	}
	o, err := p.resolve(optionMap{"name": "x", "uiParams": map[string]any{"hex": map[string]any{"winHeight": 9}}},
		patches{Global: optionMap{"path": "a.bin"}})
	if err != nil {
		t.Fatal(err)
	}
	if o.Name != "x" || o.Path != "a.bin" || o.length() != 16 || o.UIParams["hex"]["split"] != "no" ||
		o.UIParams["hex"]["winHeight"] != float64(9) {
		t.Errorf("%+v", o)
	}
	if o, _ := p.resolve(optionMap{}, patches{}); o.length() != DefaultLength || o.Name != "default" {
		t.Errorf("defaults %+v", o)
	}
	if _, err := New().AddUI(nil2{}).AddUI(nil2{}).Plugin(); err == nil {
		t.Error("a UI added twice")
	}
}

type nil2 struct{}

func (nil2) Name() string  { return "n" }
func (nil2) New() Instance { return nil }
