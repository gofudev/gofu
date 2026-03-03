package cli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildNoRunnables(t *testing.T) {
	var buf bytes.Buffer
	code := Build([]string{"testdata/valid"}, &buf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "no runnables found") {
		t.Errorf("expected 'no runnables found', got %q", buf.String())
	}
}

func TestBuildInvalid(t *testing.T) {
	var buf bytes.Buffer
	code := Build([]string{"testdata/invalid"}, &buf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	out := buf.String()
	if !strings.Contains(out, "import not allowed: os/exec") {
		t.Errorf("expected import diagnostic, got %q", out)
	}
	if !strings.Contains(out, "error") {
		t.Errorf("expected 'error' severity, got %q", out)
	}
}

func TestBuildWarningsNoBlock(t *testing.T) {
	var buf bytes.Buffer
	code := Build([]string{"testdata/warnings"}, &buf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	out := buf.String()
	if !strings.Contains(out, "warning") {
		t.Errorf("expected warning output, got %q", out)
	}
	if strings.Contains(out, "import not allowed") {
		t.Errorf("warnings should not produce import errors: %q", out)
	}
}

func TestBuildDefaultDir(t *testing.T) {
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir("testdata/valid")

	var buf bytes.Buffer
	code := Build(nil, &buf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "no runnables found") {
		t.Errorf("expected 'no runnables found', got %q", buf.String())
	}
}

func TestBuildBadDir(t *testing.T) {
	var buf bytes.Buffer
	code := Build([]string{"/nonexistent"}, &buf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if buf.Len() == 0 {
		t.Error("expected error output for bad dir")
	}
}

func TestBuildUsageError(t *testing.T) {
	var buf bytes.Buffer
	code := Build([]string{"a", "b"}, &buf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(buf.String(), "usage:") {
		t.Errorf("expected usage message, got %q", buf.String())
	}
}

func writeModule(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuildE2EValid(t *testing.T) {
	modDir := t.TempDir()
	writeModule(t, modDir, map[string]string{
		"go.mod": "module example.com/test/hello\n\ngo 1.25\n",
		"hello.go": `package hello

//gofu:runnable
func Hello() string { return "hello from gofu" }
`,
	})

	var buf bytes.Buffer
	code := Build([]string{modDir}, &buf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, buf.String())
	}

	binPath := filepath.Join(modDir, "bin", "hello")
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("binary not found: %v", err)
	}

	// Run the binary and check dispatch
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	cmd := exec.Command(binPath, "Hello")
	cmd.ExtraFiles = []*os.File{w}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	runErr := cmd.Run()
	_ = w.Close()

	fd3, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("binary failed: %v, stderr: %s", runErr, stderrBuf.String())
	}
	if got := string(fd3); got != "\"hello from gofu\"\n" {
		t.Errorf("fd3 = %q, want %q", got, "\"hello from gofu\"\n")
	}
}

func TestBuildE2ESubdirectoryPackage(t *testing.T) {
	modDir := t.TempDir()
	pkgDir := filepath.Join(modDir, "mylib")
	_ = os.Mkdir(pkgDir, 0755)
	writeModule(t, modDir, map[string]string{
		"go.mod": "module example.com/test/subpkg\n\ngo 1.25\n",
	})
	writeModule(t, pkgDir, map[string]string{
		"lib.go": `package mylib

//gofu:runnable
func Add(a int, b int) int { return a + b }
`,
	})

	var buf bytes.Buffer
	code := Build([]string{modDir}, &buf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, buf.String())
	}

	binPath := filepath.Join(modDir, "bin", "subpkg")
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("binary not found: %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	cmd := exec.Command(binPath, "Add")
	cmd.ExtraFiles = []*os.File{w}
	cmd.Stdin = strings.NewReader(`{"a":2,"b":3}`)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	runErr := cmd.Run()
	_ = w.Close()

	fd3, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("binary failed: %v, stderr: %s", runErr, stderrBuf.String())
	}
	if got := string(fd3); got != "5\n" {
		t.Errorf("fd3 = %q, want %q", got, "5\n")
	}
}

func TestBuildE2EAnalyzerErrors(t *testing.T) {
	modDir := t.TempDir()
	writeModule(t, modDir, map[string]string{
		"go.mod": "module example.com/test/bad\n\ngo 1.25\n",
		"bad.go": `package bad

import "os/exec"

//gofu:runnable
func Run() string { _ = exec.Command("ls"); return "" }
`,
	})

	var buf bytes.Buffer
	code := Build([]string{modDir}, &buf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "import not allowed") {
		t.Errorf("expected import error, got %q", buf.String())
	}
	// No binary should exist
	if _, err := os.Stat(filepath.Join(modDir, "bin", "bad")); err == nil {
		t.Error("binary should not exist after analyzer errors")
	}
}

func TestBuildE2ENoRunnables(t *testing.T) {
	modDir := t.TempDir()
	writeModule(t, modDir, map[string]string{
		"go.mod":   "module example.com/test/empty\n\ngo 1.25\n",
		"empty.go": "package empty\n\nfunc NotRunnable() string { return \"\" }\n",
	})

	var buf bytes.Buffer
	code := Build([]string{modDir}, &buf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "no runnables found") {
		t.Errorf("expected 'no runnables found', got %q", buf.String())
	}
}

func TestBuildNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var buf bytes.Buffer
	code := Build([]string{"testdata/invalid"}, &buf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Errorf("output contains ANSI codes with NO_COLOR set: %q", out)
	}
}
