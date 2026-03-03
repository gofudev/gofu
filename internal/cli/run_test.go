package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type envelope struct {
	Result     json.RawMessage `json:"result"`
	Stdout     string          `json:"stdout"`
	Stderr     string          `json:"stderr"`
	ExitCode   int             `json:"exit_code"`
	DurationMs int64           `json:"duration_ms"`
}

func setupRunModule(t *testing.T, files map[string]string) string {
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

func runInDir(t *testing.T, dir string, args []string) (envelope, int) {
	t.Helper()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)

	if code != 0 && stdout.Len() == 0 {
		return envelope{}, code
	}

	var env envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse envelope: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return env, code
}

func TestRunGreet(t *testing.T) {
	dir := setupRunModule(t, map[string]string{
		"go.mod": "module example.com/test/greet\n\ngo 1.25\n",
		"greet.go": `package greet

import "fmt"

//gofu:runnable
func Greet(name string) string {
	fmt.Println("hello stdout")
	return "Hello, " + name + "!"
}
`,
	})

	env, code := runInDir(t, dir, []string{"Greet", `{"name":"World"}`})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var result string
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("result = %q, want %q", result, "Hello, World!")
	}
	if env.Stdout != "hello stdout\n" {
		t.Errorf("stdout = %q, want %q", env.Stdout, "hello stdout\n")
	}
	if env.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0", env.ExitCode)
	}
	if env.DurationMs < 0 {
		t.Errorf("duration_ms = %d, want >= 0", env.DurationMs)
	}
}

func TestRunNoArgs(t *testing.T) {
	dir := setupRunModule(t, map[string]string{
		"go.mod": "module example.com/test/noargs\n\ngo 1.25\n",
		"noargs.go": `package noargs

//gofu:runnable
func NoArgs() string { return "ok" }
`,
	})

	env, code := runInDir(t, dir, []string{"NoArgs"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var result string
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
}

func TestRunPanic(t *testing.T) {
	dir := setupRunModule(t, map[string]string{
		"go.mod": "module example.com/test/panicker\n\ngo 1.25\n",
		"panicker.go": `package panicker

//gofu:runnable
func Boom() { panic("boom") }
`,
	})

	env, code := runInDir(t, dir, []string{"Boom"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (envelope always returned)", code)
	}
	if env.ExitCode == 0 {
		t.Error("exit_code in envelope should be non-zero for panic")
	}
	if env.Stderr == "" {
		t.Error("stderr should contain panic message")
	}
}

func TestRunTimeout(t *testing.T) {
	dir := setupRunModule(t, map[string]string{
		"go.mod": "module example.com/test/slow\n\ngo 1.25\n",
		"slow.go": `package slow

import "time"

//gofu:runnable
func Slow() { time.Sleep(10 * time.Second) }
`,
	})

	env, code := runInDir(t, dir, []string{"--timeout", "500ms", "Slow"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (envelope always returned)", code)
	}
	if env.ExitCode == 0 {
		t.Error("exit_code should be non-zero for timeout")
	}
	if env.Stderr == "" {
		t.Error("stderr should indicate timeout")
	}
}

func TestRunVoidFunction(t *testing.T) {
	dir := setupRunModule(t, map[string]string{
		"go.mod": "module example.com/test/voidfn\n\ngo 1.25\n",
		"voidfn.go": `package voidfn

//gofu:runnable
func DoNothing() {}
`,
	})

	env, code := runInDir(t, dir, []string{"DoNothing"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if string(env.Result) != "null" {
		t.Errorf("result = %s, want null", env.Result)
	}
}

func TestRunBuildFailure(t *testing.T) {
	dir := setupRunModule(t, map[string]string{
		"go.mod": "module example.com/test/bad\n\ngo 1.25\n",
		"bad.go": `package bad

//gofu:runnable
func Bad(x chan int) {}
`,
	})

	var stdout, stderr bytes.Buffer
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(dir)

	code := Run([]string{"Bad"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty, got %q", stdout.String())
	}
}

func TestRunMissingFuncName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty, got %q", stdout.String())
	}
}
