package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"gofu.dev/gofu/internal/analyzer"
)

func writeModule(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuildSuccess(t *testing.T) {
	modDir := t.TempDir()
	writeModule(t, modDir, map[string]string{
		"go.mod": "module example.com/test/hello\n\ngo 1.25\n",
		"hello.go": `package hello

//gofu:runnable
func Hello() string { return "hello" }
`,
	})

	outDir := t.TempDir()
	result, err := Build(modDir, outDir)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if len(result.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", result.Diagnostics)
	}
	if result.BinPath == "" {
		t.Fatal("BinPath is empty")
	}
	if _, err := os.Stat(result.BinPath); err != nil {
		t.Fatalf("binary not found: %v", err)
	}
}

func TestBuildAnalysisErrors(t *testing.T) {
	modDir := t.TempDir()
	writeModule(t, modDir, map[string]string{
		"go.mod": "module example.com/test/bad\n\ngo 1.25\n",
		"bad.go": `package bad

import "os/exec"

//gofu:runnable
func Bad() { _ = exec.Command("rm", "-rf", "/") }
`,
	})

	outDir := t.TempDir()
	result, err := Build(modDir, outDir)
	if err != nil {
		t.Fatalf("Build() should not return error for analysis errors, got: %v", err)
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for blocked import")
	}
	hasError := false
	for _, d := range result.Diagnostics {
		if d.Severity == analyzer.Error {
			hasError = true
		}
	}
	if !hasError {
		t.Fatal("expected at least one error-severity diagnostic")
	}
	if result.BinPath != "" {
		t.Fatalf("BinPath should be empty, got %q", result.BinPath)
	}
}

func TestBuildNoRunnables(t *testing.T) {
	modDir := t.TempDir()
	writeModule(t, modDir, map[string]string{
		"go.mod": "module example.com/test/empty\n\ngo 1.25\n",
		"empty.go": `package empty

func NotRunnable() string { return "nope" }
`,
	})

	outDir := t.TempDir()
	result, err := Build(modDir, outDir)
	if err != nil {
		t.Fatalf("Build() should not return error for no runnables, got: %v", err)
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected diagnostic for no runnables")
	}
	if result.BinPath != "" {
		t.Fatalf("BinPath should be empty, got %q", result.BinPath)
	}
}

func TestBuildMissingGoMod(t *testing.T) {
	modDir := t.TempDir()
	writeModule(t, modDir, map[string]string{
		"hello.go": `package hello

//gofu:runnable
func Hello() string { return "hello" }
`,
	})

	outDir := t.TempDir()
	_, err := Build(modDir, outDir)
	if err == nil {
		t.Fatal("expected error for missing go.mod")
	}
}
