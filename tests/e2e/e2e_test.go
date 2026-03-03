package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	gofuBin  string
	repoRoot string
)

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	repoRoot, err = filepath.Abs(filepath.Join(wd, "..", ".."))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "gofu-e2e-bin-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	gofuBin = filepath.Join(tmpDir, "gofu")
	cmd := exec.Command("go", "build", "-o", gofuBin, "./cmd/gofu")
	cmd.Dir = repoRoot
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to build gofu:", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

type envelope struct {
	Result     json.RawMessage `json:"result"`
	Stdout     string          `json:"stdout"`
	Stderr     string          `json:"stderr"`
	ExitCode   int             `json:"exit_code"`
	DurationMs int64           `json:"duration_ms"`
}

func runGofu(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	return runGofuEnv(t, dir, nil, "", args...)
}

func runGofuEnv(t *testing.T, dir string, env []string, stdin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(gofuBin, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run gofu: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func setupModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Run go mod tidy if module has dependencies (require directives)
	if gomod, ok := files["go.mod"]; ok && strings.Contains(gomod, "require") {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go mod tidy failed: %v\n%s", err, out)
		}
	}
	return dir
}

func parseEnvelope(t *testing.T, stdout string) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("failed to parse envelope: %v\nstdout: %s", err, stdout)
	}
	return env
}

func goMod(name string) string {
	return fmt.Sprintf("module gofu.dev/gofu/%s\n\ngo 1.25\n", name)
}

func goModWithDeps(name string, deps map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module gofu.dev/gofu/%s\n\ngo 1.25\n", name)
	if len(deps) > 0 {
		b.WriteString("\nrequire (\n")
		for mod := range deps {
			fmt.Fprintf(&b, "\t%s v0.0.0\n", mod)
		}
		b.WriteString(")\n\nreplace (\n")
		for mod, dir := range deps {
			fmt.Fprintf(&b, "\t%s => %s\n", mod, filepath.Join(repoRoot, dir))
		}
		b.WriteString(")\n")
	}
	return b.String()
}

// --- 2.1 Full happy path: init → write → build → run ---

func TestHappyPath(t *testing.T) {
	parent := t.TempDir()

	// gofu init
	_, stderr, code := runGofu(t, parent, "init", "mymod")
	if code != 0 {
		t.Fatalf("init failed (code %d): %s", code, stderr)
	}

	modDir := filepath.Join(parent, "mymod")

	// Overwrite with a runnable that returns a string
	if err := os.WriteFile(filepath.Join(modDir, "mymod.go"), []byte(`package mymod

//gofu:runnable
func Hello() string { return "hello e2e" }
`), 0644); err != nil {
		t.Fatal(err)
	}

	// gofu run
	stdout, stderr, code := runGofu(t, modDir, "run", "Hello")
	if code != 0 {
		t.Fatalf("run failed (code %d): %s", code, stderr)
	}

	env := parseEnvelope(t, stdout)
	if env.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0; stderr: %s", env.ExitCode, env.Stderr)
	}
	var result string
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result != "hello e2e" {
		t.Errorf("result = %q, want %q", result, "hello e2e")
	}
}

// --- 2.2 Runnable with JSON arguments ---

func TestJSONArgs(t *testing.T) {
	dir := setupModule(t, map[string]string{
		"go.mod": goMod("jsonargs"),
		"jsonargs.go": `package jsonargs

//gofu:runnable
func Greet(name string) string { return "Hello, " + name + "!" }
`,
	})

	stdout, stderr, code := runGofu(t, dir, "run", "Greet", `{"name":"world"}`)
	if code != 0 {
		t.Fatalf("run failed (code %d): %s", code, stderr)
	}

	env := parseEnvelope(t, stdout)
	if env.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0", env.ExitCode)
	}
	var result string
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result != "Hello, world!" {
		t.Errorf("result = %q, want %q", result, "Hello, world!")
	}
}

// --- 2.3 Multiple runnables dispatch ---

func TestMultipleRunnables(t *testing.T) {
	dir := setupModule(t, map[string]string{
		"go.mod": goMod("multi"),
		"multi.go": `package multi

//gofu:runnable
func FuncA() string { return "A" }

//gofu:runnable
func FuncB() string { return "B" }
`,
	})

	stdout, stderr, code := runGofu(t, dir, "run", "FuncA")
	if code != 0 {
		t.Fatalf("run FuncA failed (code %d): %s", code, stderr)
	}
	env := parseEnvelope(t, stdout)
	var result string
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result != "A" {
		t.Errorf("FuncA result = %q, want %q", result, "A")
	}

	stdout, stderr, code = runGofu(t, dir, "run", "FuncB")
	if code != 0 {
		t.Fatalf("run FuncB failed (code %d): %s", code, stderr)
	}
	env = parseEnvelope(t, stdout)
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result != "B" {
		t.Errorf("FuncB result = %q, want %q", result, "B")
	}
}

// --- 2.4 Unknown runnable name ---

func TestUnknownRunnable(t *testing.T) {
	dir := setupModule(t, map[string]string{
		"go.mod": goMod("unknown"),
		"unknown.go": `package unknown

//gofu:runnable
func Exists() string { return "ok" }
`,
	})

	stdout, stderr, code := runGofu(t, dir, "run", "NonExistent")
	if code != 0 {
		t.Fatalf("run failed (code %d): %s", code, stderr)
	}
	env := parseEnvelope(t, stdout)
	if env.ExitCode == 0 {
		t.Error("exit_code should be non-zero for unknown runnable")
	}
	if !strings.Contains(env.Stderr, "unknown function") {
		t.Errorf("stderr = %q, want it to contain 'unknown function'", env.Stderr)
	}
}

// --- 3.1 Error propagation ---

func TestErrorPropagation(t *testing.T) {
	dir := setupModule(t, map[string]string{
		"go.mod": goMod("errprop"),
		"errprop.go": `package errprop

import "fmt"

//gofu:runnable
func Fail() (string, error) { return "", fmt.Errorf("something went wrong") }
`,
	})

	stdout, stderr, code := runGofu(t, dir, "run", "Fail")
	if code != 0 {
		t.Fatalf("run failed (code %d): %s", code, stderr)
	}
	env := parseEnvelope(t, stdout)
	if env.ExitCode != 1 {
		t.Errorf("exit_code = %d, want 1", env.ExitCode)
	}
	if !strings.Contains(env.Stderr, "something went wrong") {
		t.Errorf("stderr = %q, want it to contain error message", env.Stderr)
	}
}

// --- 3.2 Panic propagation ---

func TestPanicPropagation(t *testing.T) {
	dir := setupModule(t, map[string]string{
		"go.mod": goMod("panicker"),
		"panicker.go": `package panicker

//gofu:runnable
func Boom() { panic("boom") }
`,
	})

	stdout, stderr, code := runGofu(t, dir, "run", "Boom")
	if code != 0 {
		t.Fatalf("run failed (code %d): %s", code, stderr)
	}
	env := parseEnvelope(t, stdout)
	if env.ExitCode != 2 {
		t.Errorf("exit_code = %d, want 2", env.ExitCode)
	}
	if !strings.Contains(env.Stderr, "boom") {
		t.Errorf("stderr = %q, want it to contain 'boom'", env.Stderr)
	}
}

// --- 4.1 Blocked import ---

func TestBlockedImport(t *testing.T) {
	dir := setupModule(t, map[string]string{
		"go.mod": goMod("blocked"),
		"blocked.go": `package blocked

import "os/exec"

//gofu:runnable
func Bad() string { _ = exec.Command("ls"); return "" }
`,
	})

	_, stderr, code := runGofu(t, dir, "build")
	if code == 0 {
		t.Fatal("build should fail for blocked import")
	}
	if !strings.Contains(stderr, "import not allowed") {
		t.Errorf("stderr = %q, want it to contain 'import not allowed'", stderr)
	}
}

// --- 4.2 Blocked syntax (go statement) ---

func TestBlockedSyntax(t *testing.T) {
	dir := setupModule(t, map[string]string{
		"go.mod": goMod("blockedsyntax"),
		"blockedsyntax.go": `package blockedsyntax

//gofu:runnable
func Bad() {
	go func() {}()
}
`,
	})

	_, stderr, code := runGofu(t, dir, "build")
	if code == 0 {
		t.Fatal("build should fail for blocked syntax")
	}
	if !strings.Contains(stderr, "goroutines are not allowed") {
		t.Errorf("stderr = %q, want it to contain 'goroutines are not allowed'", stderr)
	}
}

// --- 5.1 Credentials module: read secret ---

func TestCredentialsRead(t *testing.T) {
	dir := setupModule(t, map[string]string{
		"go.mod": goModWithDeps("testcreds", map[string]string{"gofu.dev/gofu/credentials": "modules/credentials"}),
		"testcreds.go": `package testcreds

import "gofu.dev/gofu/credentials"

//gofu:secret TOKEN

//gofu:runnable
func ReadToken() (string, error) { return credentials.Get("TOKEN") }
`,
	})

	stdout, stderr, code := runGofuEnv(t, dir, []string{"TOKEN=abc123"}, "", "run", "ReadToken")
	if code != 0 {
		t.Fatalf("run failed (code %d): %s", code, stderr)
	}
	env := parseEnvelope(t, stdout)
	if env.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0; stderr: %s", env.ExitCode, env.Stderr)
	}
	var result string
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result != "abc123" {
		t.Errorf("result = %q, want %q", result, "abc123")
	}
}

// --- 5.2 Credentials module: missing secret ---

func TestCredentialsMissing(t *testing.T) {
	dir := setupModule(t, map[string]string{
		"go.mod": goModWithDeps("testcredsmissing", map[string]string{"gofu.dev/gofu/credentials": "modules/credentials"}),
		"testcredsmissing.go": `package testcredsmissing

import "gofu.dev/gofu/credentials"

//gofu:secret MISSING

//gofu:runnable
func ReadMissing() (string, error) { return credentials.Get("MISSING") }
`,
	})

	// Run without setting the MISSING env var
	stdout, stderr, code := runGofuEnv(t, dir, []string{"MISSING="}, "", "run", "ReadMissing")
	if code != 0 {
		t.Fatalf("run failed (code %d): %s", code, stderr)
	}
	env := parseEnvelope(t, stdout)
	if env.ExitCode != 1 {
		t.Errorf("exit_code = %d, want 1", env.ExitCode)
	}
	if !strings.Contains(env.Stderr, "secret not set") {
		t.Errorf("stderr = %q, want it to contain 'secret not set'", env.Stderr)
	}
}

// --- 5.3 HTTP module: GET request ---

func TestHTTPGet(t *testing.T) {
	// Start a local HTTP server on a non-loopback address.
	// The http module has SSRF protection that blocks 127.0.0.0/8,
	// so we bind to 0.0.0.0 and connect via that address.
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port
	serverURL := fmt.Sprintf("http://0.0.0.0:%d/", port)

	dir := setupModule(t, map[string]string{
		"go.mod": goModWithDeps("testhttp", map[string]string{"gofu.dev/gofu/http": "modules/http"}),
		"testhttp.go": fmt.Sprintf(`package testhttp

import gohttp "gofu.dev/gofu/http"

//gofu:runnable
func Fetch() (string, error) {
	resp, err := gohttp.Get("%s")
	if err != nil {
		return "", err
	}
	return resp.Body, nil
}
`, serverURL),
	})

	stdout, stderr, code := runGofu(t, dir, "run", "Fetch")
	if code != 0 {
		t.Fatalf("run failed (code %d): %s", code, stderr)
	}
	env := parseEnvelope(t, stdout)
	if env.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0; stderr: %s", env.ExitCode, env.Stderr)
	}
	var result string
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result != `{"ok":true}` {
		t.Errorf("result = %q, want %q", result, `{"ok":true}`)
	}
}
