package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitValid(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	code := Init([]string{"mymod"}, &buf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, buf.String())
	}

	gomod, err := os.ReadFile(filepath.Join(dir, "mymod", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gomod), "module gofu.dev/gofu/mymod") {
		t.Errorf("go.mod = %q, want module gofu.dev/gofu/mymod", gomod)
	}
	if !strings.Contains(string(gomod), "go 1.23") {
		t.Errorf("go.mod missing go 1.23: %q", gomod)
	}

	gofile, err := os.ReadFile(filepath.Join(dir, "mymod", "mymod.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(gofile)
	if !strings.Contains(src, "package mymod") {
		t.Errorf("missing package declaration: %q", src)
	}
	if !strings.Contains(src, "//gofu:runnable") {
		t.Errorf("missing runnable directive: %q", src)
	}
	if !strings.Contains(src, `"fmt"`) {
		t.Errorf("missing fmt import: %q", src)
	}
}

func TestInitInvalidNames(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(dir)

	for _, name := range []string{"MyMod", "my-mod", "1mod", "my_mod"} {
		var buf bytes.Buffer
		code := Init([]string{name}, &buf)
		if code != 1 {
			t.Errorf("Init(%q) = %d, want 1", name, code)
		}
		if !strings.Contains(buf.String(), "invalid module name") {
			t.Errorf("Init(%q) stderr = %q, want 'invalid module name'", name, buf.String())
		}
	}
}

func TestInitNoName(t *testing.T) {
	var buf bytes.Buffer
	code := Init(nil, &buf)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(buf.String(), "usage:") {
		t.Errorf("expected usage message, got %q", buf.String())
	}
}

func TestInitDuplicateDirectory(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(dir)

	_ = os.Mkdir("mymod", 0755)

	var buf bytes.Buffer
	code := Init([]string{"mymod"}, &buf)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "already exists") {
		t.Errorf("expected 'already exists', got %q", buf.String())
	}
}

func TestInitCustomOwner(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	code := Init([]string{"mymod", "--owner", "alice"}, &buf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, buf.String())
	}

	gomod, err := os.ReadFile(filepath.Join(dir, "mymod", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gomod), "module gofu.dev/alice/mymod") {
		t.Errorf("go.mod = %q, want module gofu.dev/alice/mymod", gomod)
	}
}
