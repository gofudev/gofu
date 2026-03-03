package codegen

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gofu.dev/gofu/internal/analyzer"
)

var testRunnables = []analyzer.Runnable{
	{Name: "Hello", Returns: []analyzer.Field{{Type: "string"}}},
	{Name: "Fail", Returns: []analyzer.Field{{Type: "error"}}},
	{Name: "Panicker", Returns: []analyzer.Field{{Type: "string"}}},
	{Name: "NoReturn"},
	{Name: "Succeed", Returns: []analyzer.Field{{Type: "string"}, {Type: "error"}}},
	{Name: "Greet", Params: []analyzer.Field{{Name: "name", Type: "string"}}, Returns: []analyzer.Field{{Type: "string"}}},
	{Name: "Add", Params: []analyzer.Field{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}}, Returns: []analyzer.Field{{Type: "int"}}},
	{Name: "Unnamed", Params: []analyzer.Field{{Type: "string"}}, Returns: []analyzer.Field{{Type: "string"}}},
	{Name: "GreetOrFail", Params: []analyzer.Field{{Name: "name", Type: "string"}}, Returns: []analyzer.Field{{Type: "string"}, {Type: "error"}}},
}

const testStubs = `package main

import "errors"

func Hello() string              { return "hello" }
func Fail() error                { return errors.New("intentional error") }
func Panicker() string           { panic("test panic") }
func NoReturn()                  {}
func Succeed() (string, error)   { return "ok", nil }
func Greet(name string) string   { return "hello " + name }
func Add(a, b int) int           { return a + b }
func Unnamed(p0 string) string   { return p0 }
func GreetOrFail(name string) (string, error) {
	if name == "" { return "", errors.New("empty name") }
	return "hello " + name, nil
}
`

func buildBinary(t *testing.T, src []byte) string {
	t.Helper()
	dir := t.TempDir()

	for name, content := range map[string][]byte{
		"main.go":  src,
		"stubs.go": []byte(testStubs),
		"go.mod":   []byte("module test\n\ngo 1.25\n"),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}

	bin := filepath.Join(dir, "prog")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}
	return bin
}

func runBinary(t *testing.T, bin string, args ...string) (fd3Out, stderrOut []byte, exitCode int) {
	t.Helper()
	return runBinaryWithInput(t, bin, nil, args...)
}

func runBinaryWithInput(t *testing.T, bin string, stdin io.Reader, args ...string) (fd3Out, stderrOut []byte, exitCode int) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	cmd := exec.Command(bin, args...)
	cmd.ExtraFiles = []*os.File{w}
	cmd.Stdin = stdin
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	_ = w.Close()

	fd3Bytes, _ := io.ReadAll(r)

	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			return fd3Bytes, stderrBuf.Bytes(), ee.ExitCode()
		}
		t.Fatal(runErr)
	}
	return fd3Bytes, stderrBuf.Bytes(), 0
}

func TestGenerateCompilable(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(src)
	if err != nil {
		t.Fatal("not valid Go:", err)
	}
	if !bytes.Equal(src, formatted) {
		t.Error("source is not gofmt-clean")
	}
	buildBinary(t, src)
}

func TestGenerateDispatch(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	fd3, stderr, code := runBinary(t, bin, "Hello")
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if got := string(fd3); got != "\"hello\"\n" {
		t.Errorf("fd3 = %q, want %q", got, "\"hello\"\n")
	}
}

func TestGenerateUnknownFunction(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	_, stderr, code := runBinary(t, bin, "DoesNotExist")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	se := string(stderr)
	for _, name := range []string{"Hello", "Fail", "Panicker", "NoReturn", "Succeed", "Greet", "Add", "Unnamed", "GreetOrFail"} {
		if !strings.Contains(se, name) {
			t.Errorf("stderr should list %q: %s", name, se)
		}
	}
}

func TestGenerateNoArgs(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	_, stderr, code := runBinary(t, bin)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(string(stderr), "usage") {
		t.Errorf("stderr should contain usage: %s", stderr)
	}
}

func TestGenerateEmptyRunnables(t *testing.T) {
	src, err := Generate(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, parseErr := parser.ParseFile(token.NewFileSet(), "main.go", src, 0); parseErr != nil {
		t.Fatal("not valid Go:", parseErr)
	}
	formatted, err := format.Source(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src, formatted) {
		t.Error("source is not gofmt-clean")
	}
}

func TestGenerateErrorReturn(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	fd3, stderr, code := runBinary(t, bin, "Fail")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	fd3Str := string(fd3)
	if !strings.Contains(fd3Str, `"error"`) || !strings.Contains(fd3Str, "intentional error") {
		t.Errorf("fd3 should contain error JSON: %s", fd3Str)
	}
	if !strings.Contains(string(stderr), "intentional error") {
		t.Errorf("stderr should contain error message: %s", stderr)
	}
}

func TestGenerateNilErrorReturn(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	fd3, stderr, code := runBinary(t, bin, "Succeed")
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if got := string(fd3); got != "\"ok\"\n" {
		t.Errorf("fd3 = %q, want %q", got, "\"ok\"\n")
	}
}

func TestGeneratePanicRecovery(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	fd3, stderr, code := runBinary(t, bin, "Panicker")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	fd3Str := string(fd3)
	if !strings.Contains(fd3Str, `"error"`) || !strings.Contains(fd3Str, "test panic") {
		t.Errorf("fd3 should contain error JSON with panic message: %s", fd3Str)
	}
	if !strings.Contains(string(stderr), "test panic") {
		t.Errorf("stderr should contain panic message: %s", stderr)
	}
}

func TestGenerateSingleParam(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	fd3, stderr, code := runBinaryWithInput(t, bin, strings.NewReader(`{"name":"World"}`), "Greet")
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if got := string(fd3); got != "\"hello World\"\n" {
		t.Errorf("fd3 = %q, want %q", got, "\"hello World\"\n")
	}
}

func TestGenerateMultipleParams(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	fd3, stderr, code := runBinaryWithInput(t, bin, strings.NewReader(`{"a":2,"b":3}`), "Add")
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if got := string(fd3); got != "5\n" {
		t.Errorf("fd3 = %q, want %q", got, "5\n")
	}
}

func TestGenerateUnnamedParams(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	fd3, stderr, code := runBinaryWithInput(t, bin, strings.NewReader(`{"p0":"echo"}`), "Unnamed")
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if got := string(fd3); got != "\"echo\"\n" {
		t.Errorf("fd3 = %q, want %q", got, "\"echo\"\n")
	}
}

func TestGenerateMalformedJSON(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	fd3, stderr, code := runBinaryWithInput(t, bin, strings.NewReader(`not json`), "Greet")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(string(fd3), `"error"`) {
		t.Errorf("fd3 should contain error JSON: %s", fd3)
	}
	if !strings.Contains(string(stderr), "unmarshal") {
		t.Errorf("stderr should mention unmarshal error: %s", stderr)
	}
}

func TestGenerateWithModulePath(t *testing.T) {
	src, err := Generate(testRunnables, "example.com/user/mymod")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, `userpkg "example.com/user/mymod"`) {
		t.Error("expected userpkg import")
	}
	if !strings.Contains(s, "userpkg.Hello(") {
		t.Error("expected qualified call userpkg.Hello")
	}
	if !strings.Contains(s, "userpkg.Greet(") {
		t.Error("expected qualified call userpkg.Greet")
	}
	// Verify it's gofmt-clean
	formatted, err := format.Source(src)
	if err != nil {
		t.Fatal("not valid Go:", err)
	}
	if !bytes.Equal(src, formatted) {
		t.Error("source is not gofmt-clean")
	}
}

func TestGenerateWithoutModulePathNoImport(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if strings.Contains(s, "userpkg") {
		t.Error("empty module path should not produce userpkg import")
	}
}

func TestGenerateParamWithErrorReturn(t *testing.T) {
	src, err := Generate(testRunnables, "")
	if err != nil {
		t.Fatal(err)
	}
	bin := buildBinary(t, src)

	// Error case
	fd3, stderr, code := runBinaryWithInput(t, bin, strings.NewReader(`{"name":""}`), "GreetOrFail")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(string(fd3), "empty name") {
		t.Errorf("fd3 should contain error: %s", fd3)
	}
	if !strings.Contains(string(stderr), "empty name") {
		t.Errorf("stderr should contain error: %s", stderr)
	}

	// Success case
	fd3, stderr, code = runBinaryWithInput(t, bin, strings.NewReader(`{"name":"World"}`), "GreetOrFail")
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if got := string(fd3); got != "\"hello World\"\n" {
		t.Errorf("fd3 = %q, want %q", got, "\"hello World\"\n")
	}
}
