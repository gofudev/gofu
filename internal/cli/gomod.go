package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type requireDirective struct {
	Path    string
	Version string
}

type replaceDirective struct {
	OldPath    string
	OldVersion string
	NewPath    string
	NewVersion string
}

type moduleInfo struct {
	ModulePath string
	GoVersion  string
	Requires   []requireDirective
	Replaces   []replaceDirective
}

func parseGoMod(dir string) (moduleInfo, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return moduleInfo{}, fmt.Errorf("not a Go module: %w", err)
	}
	var info moduleInfo
	inRequire := false
	inReplace := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == ")" {
			inRequire = false
			inReplace = false
			continue
		}
		if mod, ok := strings.CutPrefix(line, "module "); ok {
			info.ModulePath = strings.TrimSpace(mod)
			continue
		}
		if ver, ok := strings.CutPrefix(line, "go "); ok && info.GoVersion == "" {
			info.GoVersion = strings.TrimSpace(ver)
			continue
		}
		if line == "require (" {
			inRequire = true
			continue
		}
		if line == "replace (" {
			inReplace = true
			continue
		}
		if rest, ok := strings.CutPrefix(line, "require "); ok {
			if r := parseRequireLine(rest); r.Path != "" {
				info.Requires = append(info.Requires, r)
			}
			continue
		}
		if rest, ok := strings.CutPrefix(line, "replace "); ok {
			if r := parseReplaceLine(rest); r.OldPath != "" {
				info.Replaces = append(info.Replaces, r)
			}
			continue
		}
		if inRequire && line != "" {
			if r := parseRequireLine(line); r.Path != "" {
				info.Requires = append(info.Requires, r)
			}
		}
		if inReplace && line != "" {
			if r := parseReplaceLine(line); r.OldPath != "" {
				info.Replaces = append(info.Replaces, r)
			}
		}
	}
	if info.ModulePath == "" {
		return moduleInfo{}, fmt.Errorf("go.mod missing module declaration")
	}
	return info, nil
}

func parseRequireLine(s string) requireDirective {
	if i := strings.Index(s, "//"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		return requireDirective{Path: parts[0], Version: parts[1]}
	}
	return requireDirective{}
}

func parseReplaceLine(s string) replaceDirective {
	if i := strings.Index(s, "//"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	idx := strings.Index(s, " => ")
	if idx < 0 {
		return replaceDirective{}
	}
	oldPart := strings.Fields(s[:idx])
	newPart := strings.Fields(s[idx+4:])
	var r replaceDirective
	switch len(oldPart) {
	case 1:
		r.OldPath = oldPart[0]
	case 2:
		r.OldPath, r.OldVersion = oldPart[0], oldPart[1]
	default:
		return replaceDirective{}
	}
	switch len(newPart) {
	case 1:
		r.NewPath = newPart[0]
	case 2:
		r.NewPath, r.NewVersion = newPart[0], newPart[1]
	default:
		return replaceDirective{}
	}
	return r
}

func pkgImportPath(runnableFile, moduleRoot, modulePath string) (string, error) {
	absFile, err := filepath.Abs(runnableFile)
	if err != nil {
		return "", err
	}
	pkgDir := filepath.Dir(absFile)
	rel, err := filepath.Rel(moduleRoot, pkgDir)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return modulePath, nil
	}
	return modulePath + "/" + filepath.ToSlash(rel), nil
}

func binaryName(modulePath string) string {
	if modulePath == "" {
		return "gofu-module"
	}
	if i := strings.LastIndex(modulePath, "/"); i >= 0 {
		return modulePath[i+1:]
	}
	return modulePath
}
