package pipeline

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gofu.dev/gofu/internal/analyzer"
	"gofu.dev/gofu/internal/codegen"
)

type BuildResult struct {
	BinPath     string
	Diagnostics []analyzer.Diagnostic
}

func Build(dir, outDir string) (BuildResult, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return BuildResult{}, err
	}

	result, err := analyzer.Analyze(absDir)
	if err != nil {
		return BuildResult{}, err
	}

	hasError := false
	for _, d := range result.Diagnostics {
		if d.Severity == analyzer.Error {
			hasError = true
			break
		}
	}
	if hasError {
		return BuildResult{Diagnostics: result.Diagnostics}, nil
	}

	if len(result.Runnables) == 0 {
		return BuildResult{
			Diagnostics: []analyzer.Diagnostic{{
				Msg:      "no runnables found",
				Severity: analyzer.Error,
			}},
		}, nil
	}

	mod, err := parseGoMod(absDir)
	if err != nil {
		return BuildResult{}, err
	}

	importPath, err := pkgImportPath(result.Runnables[0].File, absDir, mod.ModulePath)
	if err != nil {
		return BuildResult{}, err
	}

	src, err := codegen.Generate(result.Runnables, importPath)
	if err != nil {
		return BuildResult{}, err
	}

	tmpDir, err := os.MkdirTemp("", "gofu-build-*")
	if err != nil {
		return BuildResult{}, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	goMod := buildGoMod(mod, absDir)

	for name, content := range map[string][]byte{
		"go.mod":  []byte(goMod),
		"main.go": src,
	} {
		if werr := os.WriteFile(filepath.Join(tmpDir, name), content, 0644); werr != nil {
			return BuildResult{}, werr
		}
	}

	binName := binaryName(mod.ModulePath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return BuildResult{}, err
	}
	binPath := filepath.Join(outDir, binName)

	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = tmpDir
	var buildStderr bytes.Buffer
	build.Stderr = &buildStderr
	if err := build.Run(); err != nil {
		if buildStderr.Len() > 0 {
			return BuildResult{}, fmt.Errorf("go build failed:\n%s", buildStderr.String())
		}
		return BuildResult{}, fmt.Errorf("go build failed: %w", err)
	}

	return BuildResult{BinPath: binPath}, nil
}
