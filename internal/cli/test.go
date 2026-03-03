package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"gofu.dev/gofu/internal/analyzer"
)

func Test(args []string, stdout, stderr io.Writer) int {
	result, err := analyzer.Analyze(".")
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	noColor := os.Getenv("NO_COLOR") != ""
	hasError := false
	for _, d := range result.Diagnostics {
		if d.Severity == analyzer.Error {
			hasError = true
		}
		_, _ = fmt.Fprintln(stderr, formatDiag(d, noColor))
	}

	if hasError {
		return 1
	}

	goArgs := append([]string{"test", "./..."}, args...)
	cmd := exec.Command("go", goArgs...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
