package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gofu.dev/gofu/internal/analyzer"
	"gofu.dev/gofu/internal/pipeline"
)

func BuildBinary(dir string, stderr io.Writer) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	outDir := filepath.Join(absDir, "bin")
	result, err := pipeline.Build(dir, outDir)
	if err != nil {
		return "", err
	}

	noColor := os.Getenv("NO_COLOR") != ""
	for _, d := range result.Diagnostics {
		_, _ = fmt.Fprintln(stderr, formatDiag(d, noColor))
	}

	if len(result.Diagnostics) > 0 {
		return "", fmt.Errorf("analysis errors")
	}

	return result.BinPath, nil
}

func Build(args []string, stderr io.Writer) int {
	if len(args) > 1 {
		_, _ = fmt.Fprintln(stderr, "usage: gofu build [dir]")
		return 2
	}

	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}

	binPath, err := BuildBinary(dir, stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	_, _ = fmt.Fprintln(stderr, binPath)
	return 0
}

func formatDiag(d analyzer.Diagnostic, noColor bool) string {
	sev := "error"
	if d.Severity == analyzer.Warning {
		sev = "warning"
	}

	if !noColor {
		if d.Severity == analyzer.Error {
			sev = "\033[31m" + sev + "\033[0m"
		} else {
			sev = "\033[33m" + sev + "\033[0m"
		}
	}

	return fmt.Sprintf("%s:%d:%d: %s: %s", d.File, d.Pos.Line, d.Pos.Column, sev, d.Msg)
}
