// Package e2e runs the UI in a real Vim and Neovim: the scenario
// testdata/test.vim, with the binary of testbin.
package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// editor is the executable of Vim or Neovim: $GODA_VIM or $GODA_NVIM, or
// the one in PATH.
func editor(t *testing.T, env, name string) string {
	t.Helper()
	if p := os.Getenv(env); p != "" {
		return p
	}
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("no %s", name)
	}
	return p
}

// setup builds the binary and the files, and returns the environment of
// the scenario.
func setup(t *testing.T) []string {
	t.Helper()
	if testing.Short() {
		t.Skip("-short")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "goda-hex")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, "./testbin").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/Shougo/goda.vim").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	files := filepath.Join(dir, "files")
	if err := os.Mkdir(files, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(files, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.bin", []byte("Hello, world!\n\x00\x01\x02\xe3\x81\x82\xff"))
	write("b.bin", []byte("Hello, World!\n\x00\x01\x02\xe3\x81\x82"))
	var zb bytes.Buffer
	w := zip.NewWriter(&zb)
	f, _ := w.CreateHeader(&zip.FileHeader{Name: "a.txt", Method: zip.Store})
	f.Write([]byte("text"))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	write("a.zip", zb.Bytes())
	return append(os.Environ(),
		"GODA_VIM_DIR="+strings.TrimSpace(string(out)),
		"GODA_HEX_DIR="+root,
		"GODA_HEX_BINARY="+bin,
		"GODA_HEX_FILES="+files,
		"GODA_HEX_RESULT="+filepath.Join(dir, "result"),
	)
}

func run(t *testing.T, env []string, name string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	var result string
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "GODA_HEX_RESULT="); ok {
			result = v
		}
	}
	b, rerr := os.ReadFile(result)
	if rerr != nil {
		t.Fatalf("no result (%v): %v\n%s", err, rerr, out)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	for _, l := range lines {
		if l == "ok" {
			return
		}
	}
	t.Errorf("%s", strings.Join(lines, "\n"))
}

func script(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("testdata/test.vim")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVim(t *testing.T) {
	vim := editor(t, "GODA_VIM", "vim")
	env := setup(t)
	run(t, env, vim, "-N", "-u", "NONE", "-i", "NONE", "-n", "-es", "-S", script(t))
}

func TestNeovim(t *testing.T) {
	nvim := editor(t, "GODA_NVIM", "nvim")
	env := setup(t)
	run(t, env, nvim, "--headless", "--clean", "-n", "-S", script(t))
}
