package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoMod(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/user/mymod\n\ngo 1.25\n"), 0644)

	info, err := parseGoMod(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.ModulePath != "example.com/user/mymod" {
		t.Errorf("module path = %q, want %q", info.ModulePath, "example.com/user/mymod")
	}
	if info.GoVersion != "1.25" {
		t.Errorf("go version = %q, want %q", info.GoVersion, "1.25")
	}
}

func TestParseGoModMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := parseGoMod(dir)
	if err == nil {
		t.Fatal("expected error for missing go.mod")
	}
}

func TestParseGoModMalformed(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("not a go.mod\n"), 0644)

	_, err := parseGoMod(dir)
	if err == nil {
		t.Fatal("expected error for malformed go.mod")
	}
}

func TestParseGoModRequireReplace(t *testing.T) {
	dir := t.TempDir()
	content := `module example.com/mymod

go 1.23

require (
	example.com/dep v1.2.3
	example.com/other v0.0.0
)

replace (
	example.com/dep => ../dep
	example.com/other => /abs/path/other
)
`
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0644)

	info, err := parseGoMod(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Requires) != 2 {
		t.Fatalf("Requires len = %d, want 2", len(info.Requires))
	}
	if info.Requires[0].Path != "example.com/dep" || info.Requires[0].Version != "v1.2.3" {
		t.Errorf("Requires[0] = %+v", info.Requires[0])
	}
	if len(info.Replaces) != 2 {
		t.Fatalf("Replaces len = %d, want 2", len(info.Replaces))
	}
	if info.Replaces[0].OldPath != "example.com/dep" || info.Replaces[0].NewPath != "../dep" {
		t.Errorf("Replaces[0] = %+v", info.Replaces[0])
	}
	if info.Replaces[1].OldPath != "example.com/other" || info.Replaces[1].NewPath != "/abs/path/other" {
		t.Errorf("Replaces[1] = %+v", info.Replaces[1])
	}
}

func TestBuildGoMod(t *testing.T) {
	mod := moduleInfo{
		ModulePath: "example.com/mymod",
		GoVersion:  "1.23",
		Requires:   []requireDirective{{Path: "example.com/dep", Version: "v0.0.0"}},
		Replaces:   []replaceDirective{{OldPath: "example.com/dep", NewPath: "../dep"}},
	}
	got := buildGoMod(mod, "/abs/mymod")
	if !contains(got, "require example.com/dep") && !contains(got, "\texample.com/dep") {
		t.Errorf("missing require for dep:\n%s", got)
	}
	if !contains(got, "example.com/dep => /abs/mymod/../dep") && !contains(got, "example.com/dep => /abs/dep") {
		// filepath.Join resolves the path
		t.Errorf("missing resolved replace for dep:\n%s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
}

func TestBinaryName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"example.com/user/mymod", "mymod"},
		{"mymod", "mymod"},
		{"", "gofu-module"},
	}
	for _, tt := range tests {
		if got := binaryName(tt.input); got != tt.want {
			t.Errorf("binaryName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
