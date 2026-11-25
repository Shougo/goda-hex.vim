package hex

import "github.com/Shougo/goda.vim/sdk/custom"

// DefaultLength is the default of Options.Length: 1 MiB, as ddx.vim.
const DefaultLength = 1024 * 1024

// Options are those of ddx.vim: of the configuration (Builder.PatchGlobal,
// PatchLocal) and of goda_hex#start(), merged in that order. The params of a UI or an analyzer
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
type optionMap = custom.Map

// newCustom is the options of the configuration: the params of a UI and of
// an analyzer are merged param by param (goda.vim's sdk/custom).
func newCustom() *custom.Patches[Options] {
	return custom.New[Options](custom.Params("uiParams", "analyzerParams"))
}

// DecodeParams is defaults with params over it: the params of a UI or an
// analyzer, as its struct (json tags are the names of the params).
func DecodeParams[T any](params Params, defaults T) (T, error) {
	return custom.DecodeParams(params, defaults)
}
