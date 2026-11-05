package hex

import (
	"context"

	"github.com/Shougo/goda.vim/sdk"
)

// UI is a kind of UI ("hex"): it makes the UI of each session.
type UI interface {
	Name() string
	// New is the UI of a new session.
	New() Instance
}

// Instance is the UI of a session.
//
// The editor may call back while a method waits for it (an input(), the
// autocommands of a buffer): an Instance does not hold its locks across
// calls of the editor.
type Instance interface {
	// Redraw shows the buffer of the session (goda_hex#start(),
	// goda_hex#redraw(), an action that changed it).
	Redraw(ctx context.Context, h sdk.Host, a Args) error
	// Jump moves the cursor to the address (goda_hex#jump()).
	Jump(ctx context.Context, h sdk.Host, a Args, address int64) error
	// Actions are the UI actions, by name.
	Actions() map[string]Action
}

// Action is a UI action: goda_hex#ui_action().
type Action func(ctx context.Context, h sdk.Host, a Args, params Params) (ActionFlags, error)

// ActionFlags say what follows an action.
type ActionFlags int

const (
	// None: nothing.
	None ActionFlags = 0
	// Redraw: the UI is drawn again.
	Redraw ActionFlags = 1 << iota
)

// Args are what the methods of an Instance get.
type Args struct {
	Options Options
	// Params are the params of the UI (Options.UIParams[Options.UI]).
	Params  Params
	Context Context
	// Buffer is the file of Options.Path.
	Buffer *Buffer
	// Another is the file of Options.AnotherPath (empty without it).
	Another *Buffer
}

// Context is where goda_hex#start() was called.
type Context struct {
	Cwd   string `json:"cwd"`
	BufNr int    `json:"bufNr"`
	WinID int    `json:"winId"`
}

// Analyzer reads the structure of a kind of file (an ELF binary, a zip
// archive): goda_hex#analyze().
type Analyzer interface {
	Name() string
	// Detect says whether the buffer is of its kind.
	Detect(b *Buffer, params Params) bool
	// Parse reads its structure.
	Parse(b *Buffer, params Params) ([]AnalyzeResult, error)
}

// AnalyzeResult is a part of the file (a header): its values.
type AnalyzeResult struct {
	Name   string         `json:"name"`
	Values []AnalyzeValue `json:"values"`
}

// AnalyzeValue is a value of the file, where it is: an integer of Size
// bytes (RawType "integer", Value int64) or a string (RawType "string",
// Value string).
type AnalyzeValue struct {
	Name    string `json:"name"`
	RawType string `json:"rawType"`
	Value   any    `json:"value"`
	// Size in bytes (0: unknown).
	Size     int   `json:"size,omitempty"`
	IsLittle *bool `json:"isLittle,omitempty"`
	Address  int64 `json:"address"`
	// Encoding of a string ("utf-8" by default).
	Encoding string `json:"encoding,omitempty"`
}
