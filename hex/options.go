package hex

import (
	"encoding/json"
	"fmt"
	"maps"
)

// DefaultLength is the default of Options.Length: 1 MiB, as ddx.vim.
const DefaultLength = 1024 * 1024

// Options are those of ddx.vim: of the configuration (Builder.PatchGlobal,
// PatchLocal), of goda_hex#custom#patch_global() and patch_local(), and of
// goda_hex#start(), merged in that order. The params of a UI or an analyzer
// are merged param by param.
type Options struct {
	// Name is the session: a start with the name of another shows it again.
	// Default "default".
	Name string `json:"name,omitempty"`
	// UI is the name of the UI ("hex").
	UI string `json:"ui,omitempty"`
	// UIParams are the params of each UI, by its name.
	UIParams map[string]Params `json:"uiParams,omitempty"`
	// Analyzers are tried in order: the first that detects the file parses
	// it (goda_hex#analyze()).
	Analyzers []string `json:"analyzers,omitempty"`
	// AnalyzerParams are the params of each analyzer, by its name.
	AnalyzerParams map[string]Params `json:"analyzerParams,omitempty"`
	// Path is the file.
	Path string `json:"path,omitempty"`
	// AnotherPath is a file to compare with (the diff of the UI, of
	// goda_hex#get_diff()), from the same Offset for the same Length.
	AnotherPath string `json:"anotherPath,omitempty"`
	// Offset is where the file is read from.
	Offset int64 `json:"offset,omitempty"`
	// Length is how much of the file is read: 0 is all of it. Default
	// DefaultLength. When it is less than the file, the size of the file
	// cannot be changed.
	Length *int64 `json:"length,omitempty"`
}

// length is Options.Length, or its default.
func (o Options) length() int64 {
	if o.Length == nil {
		return DefaultLength
	}
	return *o.Length
}

// Params are the params of a UI or an analyzer: DecodeParams decodes them.
type Params map[string]any

// optionMap is Options as Vim sends them.
type optionMap = map[string]any

// paramKeys are the options merged param by param.
var paramKeys = map[string]bool{"uiParams": true, "analyzerParams": true}

func toMap(o Options) (optionMap, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	m := optionMap{}
	return m, json.Unmarshal(b, &m)
}

// merge is the options of dst with those of src over them.
func merge(dst, src optionMap) optionMap {
	out := maps.Clone(dst)
	if out == nil {
		out = optionMap{}
	}
	for k, v := range src {
		if paramKeys[k] {
			out[k] = mergeParams(asMap(out[k]), asMap(v))
			continue
		}
		out[k] = v
	}
	return out
}

func mergeParams(dst, src map[string]any) map[string]any {
	out := maps.Clone(dst)
	if out == nil {
		out = map[string]any{}
	}
	for name, p := range src {
		m := maps.Clone(asMap(out[name]))
		if m == nil {
			m = map[string]any{}
		}
		maps.Copy(m, asMap(p))
		out[name] = m
	}
	return out
}

func asMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case Params:
		return m
	}
	return nil
}

func fromMap(m optionMap) (Options, error) {
	var o Options
	b, err := json.Marshal(m)
	if err != nil {
		return o, err
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return o, fmt.Errorf("invalid options: %w", err)
	}
	if o.Name == "" {
		o.Name = "default"
	}
	return o, nil
}

// DecodeParams is defaults with params over it: the params of a UI or an
// analyzer, as its struct (json tags are the names of the params).
func DecodeParams[T any](params Params, defaults T) (T, error) {
	b, err := json.Marshal(params)
	if err != nil {
		return defaults, err
	}
	p := defaults
	if err := json.Unmarshal(b, &p); err != nil {
		return defaults, fmt.Errorf("invalid params: %w", err)
	}
	return p, nil
}
