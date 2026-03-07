package pipeline

import (
	"os"
	"path/filepath"

	"gofu.dev/gofu/internal/analyzer"
)

type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Runnable struct {
	Name    string  `json:"name"`
	Params  []Field `json:"params"`
	Returns []Field `json:"returns"`
}

type AnalyzeResult struct {
	Runnables []Runnable
}

func Analyze(dir string) (AnalyzeResult, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return AnalyzeResult{}, err
	}

	result, err := analyzer.Analyze(absDir)
	if err != nil {
		return AnalyzeResult{}, err
	}

	runnables := make([]Runnable, len(result.Runnables))
	for i, r := range result.Runnables {
		params := make([]Field, len(r.Params))
		for j, p := range r.Params {
			params[j] = Field{Name: p.Name, Type: p.Type}
		}
		returns := make([]Field, len(r.Returns))
		for j, ret := range r.Returns {
			returns[j] = Field{Name: ret.Name, Type: ret.Type}
		}
		runnables[i] = Runnable{Name: r.Name, Params: params, Returns: returns}
	}

	return AnalyzeResult{Runnables: runnables}, nil
}

func AnalyzeSource(source string) (AnalyzeResult, error) {
	dir, err := os.MkdirTemp("", "gofu-analyze-*")
	if err != nil {
		return AnalyzeResult{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	if err := os.WriteFile(filepath.Join(dir, "source.go"), []byte(source), 0644); err != nil {
		return AnalyzeResult{}, err
	}

	return Analyze(dir)
}
