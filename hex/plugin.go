// Package hex is goda-hex.vim, the successor of ddx.vim: a configuration
// (main.go) adds the UIs and the analyzers, patches the options, and serves
// the plugin.
//
//	b := hex.New()
//	b.AddUI(hexui.New())
//	b.AddAnalyzer(elf.New())
//	b.PatchGlobal(hex.Options{UI: "hex", Analyzers: []string{"elf"}})
//	b.Main()
package hex

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Shougo/goda.vim/host"
	"github.com/Shougo/goda.vim/sdk"
)

// PluginName is the name of the plugin: goda#request('hex', ...).
const PluginName = "hex"

// ScriptVersion is the version of the Vim script (autoload/goda_hex.vim) this
// binary goes with (sdk.ScriptVersioner): goda.vim refuses a binary of
// another one and says to build it again. Bump it with s:version there when
// the Vim script and the binary no longer work together.
const ScriptVersion = 1

// WatchInterval is how often the files of the sessions are checked for a
// change outside.
var WatchInterval = 2 * time.Second

// Builder collects the UIs, the analyzers and the configuration.
type Builder struct {
	uis       map[string]UI
	analyzers map[string]Analyzer
	global    Options
	local     map[string]Options
	errs      []string
}

// New returns an empty Builder.
func New() *Builder {
	return &Builder{uis: map[string]UI{}, analyzers: map[string]Analyzer{}, local: map[string]Options{}}
}

// AddUI adds a UI under its Name.
func (b *Builder) AddUI(u UI) *Builder {
	if _, dup := b.uis[u.Name()]; dup {
		b.errs = append(b.errs, fmt.Sprintf("ui %q added twice", u.Name()))
	}
	b.uis[u.Name()] = u
	return b
}

// AddAnalyzer adds an analyzer under its Name.
func (b *Builder) AddAnalyzer(a Analyzer) *Builder {
	if _, dup := b.analyzers[a.Name()]; dup {
		b.errs = append(b.errs, fmt.Sprintf("analyzer %q added twice", a.Name()))
	}
	b.analyzers[a.Name()] = a
	return b
}

// PatchGlobal patches the options of every session.
func (b *Builder) PatchGlobal(o Options) *Builder {
	b.global = b.patch(b.global, o)
	return b
}

// PatchLocal patches the options of the session name.
func (b *Builder) PatchLocal(name string, o Options) *Builder {
	b.local[name] = b.patch(b.local[name], o)
	return b
}

func (b *Builder) patch(dst, src Options) Options {
	d, err := toMap(dst)
	if err == nil {
		var s optionMap
		if s, err = toMap(src); err == nil {
			var o Options
			if o, err = fromMap(merge(d, s)); err == nil {
				if dst.Name == "" && src.Name == "" {
					o.Name = ""
				}
				return o
			}
		}
	}
	b.errs = append(b.errs, err.Error())
	return dst
}

// Plugin is the plugin of the configuration, or the errors in it.
func (b *Builder) Plugin() (*Plugin, error) {
	if len(b.errs) > 0 {
		return nil, fmt.Errorf("goda-hex: %s", strings.Join(b.errs, "; "))
	}
	p := &Plugin{uis: b.uis, analyzers: b.analyzers, sessions: map[string]*session{}}
	var err error
	if p.global, err = toMap(b.global); err != nil {
		return nil, err
	}
	p.local = map[string]optionMap{}
	for name, o := range b.local {
		if p.local[name], err = toMap(o); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Main serves the plugin, with the other plugins given, as the whole
// program: the last line of main.go. A configuration error does not end
// it: goda_hex#start() shows it, and the other plugins work.
func (b *Builder) Main(others ...sdk.Plugin) {
	host.Main(append([]sdk.Plugin{b.MainPlugin()}, others...)...)
}

// MainPlugin is the plugin, for the Main of another binary: with an invalid
// configuration, one that tells it.
func (b *Builder) MainPlugin() sdk.Plugin {
	p, err := b.Plugin()
	if err != nil {
		log.Print(err)
		return brokenPlugin{err}
	}
	return p
}

type brokenPlugin struct{ err error }

func (brokenPlugin) Name() string { return PluginName }

// ScriptVersion: a broken configuration says why, not that the binary is to
// be built again.
func (brokenPlugin) ScriptVersion() int { return ScriptVersion }

func (p brokenPlugin) Handlers() map[string]sdk.Handler {
	hs := map[string]sdk.Handler{}
	for name := range (&Plugin{}).Handlers() {
		hs[name] = func(context.Context, sdk.Host, []any) (any, error) { return nil, p.err }
	}
	hs["status"] = func(context.Context, sdk.Host, []any) (any, error) { return p.err.Error(), nil }
	return hs
}

// Plugin serves the sessions.
type Plugin struct {
	uis       map[string]UI
	analyzers map[string]Analyzer
	global    optionMap
	local     map[string]optionMap

	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	ui       string
	instance Instance
	args     Args
	// stop ends the watch of its files.
	stop context.CancelFunc
}

func (p *Plugin) Name() string { return PluginName }

// ScriptVersion: sdk.ScriptVersioner.
func (p *Plugin) ScriptVersion() int { return ScriptVersion }

func (p *Plugin) Handlers() map[string]sdk.Handler {
	return map[string]sdk.Handler{
		"status":     func(context.Context, sdk.Host, []any) (any, error) { return "", nil },
		"start":      p.start,
		"restart":    p.restart,
		"redraw":     p.redraw,
		"uiAction":   p.uiAction,
		"analyze":    p.analyze,
		"change":     p.change,
		"jump":       p.jump,
		"getDiff":    p.getDiff,
		"getStrings": p.getStrings,
		"sessions":   p.sessionNames,
		"options":    p.options,
	}
}

// patches are those of goda_hex#custom#*(), which every request carries.
type patches struct {
	Global optionMap            `json:"global"`
	Local  map[string]optionMap `json:"local"`
}

// resolve is the options of a start with the options given.
func (p *Plugin) resolve(user optionMap, pt patches) (Options, error) {
	name, _ := user["name"].(string)
	if name == "" {
		name, _ = pt.Global["name"].(string)
	}
	if name == "" {
		name, _ = p.global["name"].(string)
	}
	if name == "" {
		name = "default"
	}
	m := merge(p.global, p.local[name])
	m = merge(m, pt.Global)
	m = merge(m, pt.Local[name])
	m = merge(m, user)
	m["name"] = name
	return fromMap(m)
}

// start(options, context, patches) opens the file of the options in the
// session of their name, and shows it.
func (p *Plugin) start(ctx context.Context, h sdk.Host, params []any) (any, error) {
	var user optionMap
	var c Context
	var pt patches
	if err := sdk.DecodeArgs(params, &user, sdk.Optional(&c), sdk.Optional(&pt)); err != nil {
		return nil, err
	}
	o, err := p.resolve(user, pt)
	if err != nil {
		return nil, err
	}
	if o.UI == "" {
		return nil, errors.New("goda-hex: no ui (the option \"ui\")")
	}
	u, ok := p.uis[o.UI]
	if !ok {
		return nil, fmt.Errorf("goda-hex: no such ui: %q", o.UI)
	}
	if o.Path == "" {
		return nil, errors.New("goda-hex: no file (the option \"path\")")
	}
	args := Args{Options: o, Params: o.UIParams[o.UI], Context: c, Buffer: NewBuffer(), Another: NewBuffer()}
	if err := p.open(ctx, h, &args); err != nil {
		return nil, err
	}
	p.mu.Lock()
	s := p.sessions[o.Name]
	if s == nil || s.ui != o.UI {
		if s != nil {
			s.stop()
		}
		s = &session{ui: o.UI, instance: u.New(), stop: func() {}}
		p.sessions[o.Name] = s
	}
	s.args = args
	s.stop()
	s.stop = watch(h, args.Buffer, args.Another)
	p.mu.Unlock()
	return nil, p.draw(ctx, h, s.instance, args)
}

// open reads the files of the options (the paths expanded as Vim does, from
// the directory of the start).
func (p *Plugin) open(ctx context.Context, h sdk.Host, a *Args) error {
	expand := func(path string) (string, error) {
		v, err := h.Call(ctx, "expand", path)
		if err != nil {
			return "", err
		}
		s, _ := v.(string)
		if s == "" {
			s = path
		}
		if !filepath.IsAbs(s) && a.Context.Cwd != "" {
			s = filepath.Join(a.Context.Cwd, s)
		}
		return s, nil
	}
	path, err := expand(a.Options.Path)
	if err != nil {
		return err
	}
	if err := a.Buffer.Open(path, a.Options.Offset, a.Options.length()); err != nil {
		return fmt.Errorf("goda-hex: open %s: %w", path, err)
	}
	if a.Options.AnotherPath != "" {
		another, err := expand(a.Options.AnotherPath)
		if err != nil {
			return err
		}
		if err := a.Another.Open(another, a.Options.Offset, a.Options.length()); err != nil {
			return fmt.Errorf("goda-hex: open %s: %w", another, err)
		}
	}
	return nil
}

// watch says, until it is stopped, when the files change outside.
func watch(h sdk.Host, bufs ...*Buffer) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		t := time.NewTicker(WatchInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			for _, b := range bufs {
				if changed, msg := b.ChangedOutside(); changed {
					_ = h.Notify(ctx, "goda_hex#util#print_error", "goda-hex: "+msg)
				}
			}
		}
	}()
	return cancel
}

func (p *Plugin) draw(ctx context.Context, h sdk.Host, in Instance, a Args) error {
	if a.Params == nil {
		a.Params = Params{}
	}
	if err := in.Redraw(ctx, h, a); err != nil {
		return err
	}
	return h.Cmd(ctx, "redraw")
}

func (p *Plugin) session(name string) (*session, Args, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := p.sessions[name]
	if s == nil {
		return nil, Args{}, fmt.Errorf("goda-hex: no such session: %q", name)
	}
	return s, s.args, nil
}

func (p *Plugin) sessionArg(params []any, rest ...any) (*session, Args, error) {
	var name string
	if err := sdk.DecodeArgs(params, append([]any{&name}, rest...)...); err != nil {
		return nil, Args{}, err
	}
	return p.session(name)
}

// restart(name) reads the files again (the changes are lost).
func (p *Plugin) restart(ctx context.Context, h sdk.Host, params []any) (any, error) {
	s, a, err := p.sessionArg(params)
	if err != nil {
		return nil, err
	}
	if err := p.open(ctx, h, &a); err != nil {
		return nil, err
	}
	return nil, p.draw(ctx, h, s.instance, a)
}

// redraw(name) shows the session again.
func (p *Plugin) redraw(ctx context.Context, h sdk.Host, params []any) (any, error) {
	s, a, err := p.sessionArg(params)
	if err != nil {
		return nil, err
	}
	return nil, p.draw(ctx, h, s.instance, a)
}

// uiAction(name, action, params) runs the UI action.
func (p *Plugin) uiAction(ctx context.Context, h sdk.Host, params []any) (any, error) {
	var action string
	var ap Params
	s, a, err := p.sessionArg(params, &action, sdk.Optional(&ap))
	if err != nil {
		return nil, err
	}
	act, ok := s.instance.Actions()[action]
	if !ok {
		names := make([]string, 0)
		for n := range s.instance.Actions() {
			names = append(names, n)
		}
		slices.Sort(names)
		return nil, fmt.Errorf("goda-hex: no such action of the ui %q: %q (%s)",
			s.ui, action, strings.Join(names, ", "))
	}
	if ap == nil {
		ap = Params{}
	}
	if a.Params == nil {
		a.Params = Params{}
	}
	flags, err := act(ctx, h, a, ap)
	if err != nil {
		return nil, err
	}
	if flags&Redraw != 0 {
		return nil, p.draw(ctx, h, s.instance, a)
	}
	return nil, nil
}

// analyze(name) is the structure of the file: that of the first analyzer
// of the option "analyzers" that detects it.
func (p *Plugin) analyze(_ context.Context, _ sdk.Host, params []any) (any, error) {
	_, a, err := p.sessionArg(params)
	if err != nil {
		return []AnalyzeResult{}, nil
	}
	for _, name := range a.Options.Analyzers {
		an, ok := p.analyzers[name]
		if !ok {
			return nil, fmt.Errorf("goda-hex: no such analyzer: %q", name)
		}
		ap := a.Options.AnalyzerParams[name]
		if !an.Detect(a.Buffer, ap) {
			continue
		}
		res, err := an.Parse(a.Buffer, ap)
		if err != nil {
			return nil, fmt.Errorf("goda-hex: analyzer %s: %w", name, err)
		}
		if res == nil {
			res = []AnalyzeResult{}
		}
		return res, nil
	}
	return []AnalyzeResult{}, nil
}

// change(name, value, new) changes a value of goda_hex#analyze() (or of
// another source): an integer to the number new, a string to the text new
// (padded or cut to its size).
func (p *Plugin) change(_ context.Context, _ sdk.Host, params []any) (any, error) {
	var v AnalyzeValue
	var newValue any
	_, a, err := p.sessionArg(params, &v, &newValue)
	if err != nil {
		return nil, err
	}
	var data []byte
	if v.RawType == "integer" {
		n, ok := new(big.Int).SetString(strings.TrimSpace(fmt.Sprint(newValue)), 0)
		if !ok {
			return nil, fmt.Errorf("goda-hex: not an integer: %v", newValue)
		}
		little := v.IsLittle == nil || *v.IsLittle
		size := v.Size
		if size <= 0 {
			size = 4
		}
		if data, err = IntBytes(n, size, little, n.Sign() < 0); err != nil {
			return nil, err
		}
	} else {
		s, ok := newValue.(string)
		if !ok {
			return nil, fmt.Errorf("goda-hex: not a string: %v", newValue)
		}
		enc := v.Encoding
		if enc == "" {
			enc = UTF8
		}
		if data, err = FitString(s, v.Size, enc); err != nil {
			return nil, err
		}
	}
	return nil, a.Buffer.Change(v.Address, data)
}

// jump(name, address) moves the cursor of the UI to the address.
func (p *Plugin) jump(ctx context.Context, h sdk.Host, params []any) (any, error) {
	var address int64
	s, a, err := p.sessionArg(params, &address)
	if err != nil {
		return nil, err
	}
	if a.Params == nil {
		a.Params = Params{}
	}
	return nil, s.instance.Jump(ctx, h, a, address)
}

// getDiff(name) are the differences of the file with the option
// "anotherPath".
func (p *Plugin) getDiff(_ context.Context, _ sdk.Host, params []any) (any, error) {
	_, a, err := p.sessionArg(params)
	if err != nil || a.Options.AnotherPath == "" {
		return []Diff{}, nil
	}
	return Diffs(a.Buffer, a.Another), nil
}

// Diffs are the differences of b with another.
func Diffs(b, another *Buffer) []Diff {
	off := b.Offset()
	d := BinaryDiff(off, b.Bytes(off, b.Size()), another.Bytes(another.Offset(), another.Size()))
	if d == nil {
		d = []Diff{}
	}
	return d
}

// getStrings(name, minLength, encoding) are the strings of the file.
func (p *Plugin) getStrings(_ context.Context, _ sdk.Host, params []any) (any, error) {
	minLength := 4
	encoding := UTF8
	_, a, err := p.sessionArg(params, sdk.Optional(&minLength), sdk.Optional(&encoding))
	if err != nil {
		return []ExtractedString{}, nil
	}
	out, err := a.Buffer.Strings(encoding, minLength)
	if out == nil {
		out = []ExtractedString{}
	}
	return out, err
}

func (p *Plugin) sessionNames(context.Context, sdk.Host, []any) (any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	names := make([]string, 0, len(p.sessions))
	for n := range p.sessions {
		names = append(names, n)
	}
	slices.Sort(names)
	return names, nil
}

// options(name, patches) are the options a start of name would have,
// without those of the start (goda_hex#custom#get_current()).
func (p *Plugin) options(_ context.Context, _ sdk.Host, params []any) (any, error) {
	var name string
	var pt patches
	if err := sdk.DecodeArgs(params, &name, sdk.Optional(&pt)); err != nil {
		return nil, err
	}
	if _, args, err := p.session(name); err == nil {
		return toMap(args.Options)
	}
	o, err := p.resolve(optionMap{"name": name}, pt)
	if err != nil {
		return nil, err
	}
	return toMap(o)
}
