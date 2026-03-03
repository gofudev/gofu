package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gofu.dev/gofu/internal/analyzer"
	"gofu.dev/gofu/internal/codegen"
)

func BuildBinary(dir string, stderr io.Writer) (string, error) {
	result, err := analyzer.Analyze(dir)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("analysis errors")
	}

	if len(result.Runnables) == 0 {
		return "", fmt.Errorf("no runnables found")
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	mod, err := parseGoMod(absDir)
	if err != nil {
		return "", err
	}

	importPath, err := pkgImportPath(result.Runnables[0].File, absDir, mod.ModulePath)
	if err != nil {
		return "", err
	}

	src, err := codegen.Generate(result.Runnables, importPath)
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "gofu-build-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	goMod := buildGoMod(mod, absDir)

	for name, content := range map[string][]byte{
		"go.mod":  []byte(goMod),
		"main.go": src,
	} {
		if werr := os.WriteFile(filepath.Join(tmpDir, name), content, 0644); werr != nil {
			return "", werr
		}
	}

	binName := binaryName(mod.ModulePath)
	binDir := filepath.Join(absDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", err
	}
	binPath := filepath.Join(binDir, binName)

	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = tmpDir
	build.Stderr = stderr
	if err := build.Run(); err != nil {
		return "", fmt.Errorf("go build failed")
	}

	return binPath, nil
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

func buildGoMod(mod moduleInfo, absDir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module gofu-build\n\ngo %s\n\n", mod.GoVersion)

	// require: user module + any modules it replaces locally
	fmt.Fprintf(&b, "require (\n\t%s v0.0.0\n", mod.ModulePath)
	for _, rep := range mod.Replaces {
		ver := "v0.0.0"
		for _, req := range mod.Requires {
			if req.Path == rep.OldPath {
				ver = req.Version
				break
			}
		}
		if rep.OldVersion != "" {
			fmt.Fprintf(&b, "\t%s %s\n", rep.OldPath, ver)
		} else {
			fmt.Fprintf(&b, "\t%s %s\n", rep.OldPath, ver)
		}
	}
	b.WriteString(")\n\n")

	// replace: user module + propagate all replace directives, resolving relative paths
	fmt.Fprintf(&b, "replace (\n\t%s => %s\n", mod.ModulePath, absDir)
	for _, rep := range mod.Replaces {
		newPath := rep.NewPath
		if strings.HasPrefix(newPath, "./") || strings.HasPrefix(newPath, "../") {
			newPath = filepath.Join(absDir, newPath)
		}
		old := rep.OldPath
		if rep.OldVersion != "" {
			old += " " + rep.OldVersion
		}
		if rep.NewVersion != "" {
			fmt.Fprintf(&b, "\t%s => %s %s\n", old, newPath, rep.NewVersion)
		} else {
			fmt.Fprintf(&b, "\t%s => %s\n", old, newPath)
		}
	}
	b.WriteString(")\n")
	return b.String()
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
