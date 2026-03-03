package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

func Run(args []string, stdout, stderr io.Writer) int {
	// Parse --timeout flag
	timeout := 30 * time.Second
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--timeout" {
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "usage: gofu run [--timeout duration] <FuncName> [json-args]")
				return 2
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "invalid timeout: %s\n", args[i+1])
				return 2
			}
			timeout = d
			i++
		} else if val, ok := strings.CutPrefix(args[i], "--timeout="); ok {
			d, err := time.ParseDuration(val)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "invalid timeout: %s\n", val)
				return 2
			}
			timeout = d
		} else {
			rest = append(rest, args[i])
		}
	}

	if len(rest) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: gofu run [--timeout duration] <FuncName> [json-args]")
		return 2
	}

	funcName := rest[0]
	var jsonArgs string
	if len(rest) > 1 {
		jsonArgs = rest[1]
	}

	// Build
	binPath, err := BuildBinary(".", stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	// Set up fd 3 pipe
	pr, pw, err := os.Pipe()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, funcName)
	cmd.ExtraFiles = []*os.File{pw}

	// Capture stdout/stderr
	var childStdout, childStderr bytes.Buffer
	cmd.Stdout = &childStdout
	cmd.Stderr = &childStderr

	// Pipe JSON args to stdin
	if jsonArgs != "" {
		cmd.Stdin = strings.NewReader(jsonArgs)
	} else if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		cmd.Stdin = os.Stdin
	}

	// Read fd 3 in goroutine to prevent pipe buffer deadlock
	fd3Ch := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(pr)
		fd3Ch <- data
	}()

	// Execute and measure duration
	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	// Close write end so reader finishes
	_ = pw.Close()
	fd3Data := <-fd3Ch
	_ = pr.Close()

	// Determine exit code
	exitCode := 0
	if runErr != nil {
		exitCode = 1
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		childStderr.WriteString("timeout: process killed after " + timeout.String() + "\n")
	}

	// Parse fd 3 result
	var result json.RawMessage
	fd3Trimmed := bytes.TrimSpace(fd3Data)
	if len(fd3Trimmed) > 0 {
		result = json.RawMessage(fd3Trimmed)
	}

	// Assemble envelope
	envelope := struct {
		Result     json.RawMessage `json:"result"`
		Stdout     string          `json:"stdout"`
		Stderr     string          `json:"stderr"`
		ExitCode   int             `json:"exit_code"`
		DurationMs int64           `json:"duration_ms"`
	}{
		Result:     result,
		Stdout:     childStdout.String(),
		Stderr:     childStderr.String(),
		ExitCode:   exitCode,
		DurationMs: duration.Milliseconds(),
	}

	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(envelope); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	return 0
}
