package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func testInDir(t *testing.T, dir string, args []string) (stdout, stderr string, code int) {
	t.Helper()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(dir)

	var outBuf, errBuf bytes.Buffer
	code = Test(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

func TestTestPassingModule(t *testing.T) {
	dir := setupTestModule(t, map[string]string{
		"go.mod": "module example.com/test/passing\n\ngo 1.25\n",
		"lib.go": "package passing\n\nfunc Add(a, b int) int { return a + b }\n",
		"lib_test.go": `package passing

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("1+2 != 3")
	}
}
`,
	})

	_, _, code := testInDir(t, dir, nil)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestTestAnalyzerViolation(t *testing.T) {
	dir := setupTestModule(t, map[string]string{
		"go.mod": "module example.com/test/blocked\n\ngo 1.25\n",
		"bad.go": "package blocked\n\nimport \"os/exec\"\n\nfunc Run() { _ = exec.Command(\"ls\") }\n",
		"bad_test.go": `package blocked

import "testing"

func TestRun(t *testing.T) {
	t.Fatal("should not run")
}
`,
	})

	stdout, stderr, code := testInDir(t, dir, nil)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "import not allowed") {
		t.Errorf("expected import diagnostic, got stderr=%q", stderr)
	}
	if strings.Contains(stdout, "FAIL") || strings.Contains(stdout, "PASS") {
		t.Errorf("go test should not have run, got stdout=%q", stdout)
	}
}

func TestTestWarningsStillRun(t *testing.T) {
	dir := setupTestModule(t, map[string]string{
		"go.mod": "module example.com/test/warns\n\ngo 1.25\n",
		"lib.go": `package warns

type secret struct{ name string }

//gofu:runnable
func GetSecret() secret { return secret{} }
`,
		"lib_test.go": `package warns

import "testing"

func TestGetSecret(t *testing.T) {
	s := GetSecret()
	_ = s
}
`,
	})

	_, stderr, code := testInDir(t, dir, nil)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "warning") {
		t.Errorf("expected warning output, got stderr=%q", stderr)
	}
}

func TestTestFlagPassthrough(t *testing.T) {
	dir := setupTestModule(t, map[string]string{
		"go.mod": "module example.com/test/flags\n\ngo 1.25\n",
		"lib.go": "package flags\n\nfunc Greet() string { return \"hi\" }\n",
		"lib_test.go": `package flags

import "testing"

func TestGreet(t *testing.T) {
	if Greet() != "hi" {
		t.Fatal("wrong")
	}
}

func TestOther(t *testing.T) {}
`,
	})

	stdout, _, code := testInDir(t, dir, []string{"-v", "-run", "TestGreet"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "TestGreet") {
		t.Errorf("expected TestGreet in verbose output, got %q", stdout)
	}
	if strings.Contains(stdout, "TestOther") {
		t.Errorf("TestOther should not have run with -run TestGreet, got %q", stdout)
	}
}
