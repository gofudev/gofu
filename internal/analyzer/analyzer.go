package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

type Severity int

const (
	Error Severity = iota
	Warning
)

type Diagnostic struct {
	File     string
	Pos      token.Position
	Msg      string
	Severity Severity
}

type Field struct {
	Name string
	Type string
}

type Runnable struct {
	Name    string
	Params  []Field
	Returns []Field
	File    string
	Pos     token.Position
}

type Result struct {
	Diagnostics []Diagnostic
	Runnables   []Runnable
	Secrets     []string
}

var testingImports = map[string]bool{
	"testing":        true,
	"testing/fstest": true,
	"testing/iotest": true,
	"testing/quick":  true,
}

var allowedStdlib = map[string]bool{
	"fmt":             true,
	"strings":         true,
	"strconv":         true,
	"bytes":           true,
	"unicode":         true,
	"unicode/utf8":    true,
	"math":            true,
	"math/big":        true,
	"math/rand/v2":    true,
	"time":            true,
	"encoding/json":   true,
	"encoding/base64": true,
	"encoding/hex":    true,
	"encoding/csv":    true,
	"encoding/xml":    true,
	"encoding/binary": true,
	"encoding/pem":    true,
	"errors":          true,
	"sort":            true,
	"slices":          true,
	"maps":            true,
	"cmp":             true,
	"iter":            true,
	"regexp":          true,
	"path":            true,
	"io":              true,
	"bufio":           true,
	"context":         true,
	"net/url":         true,
	"crypto/sha256":   true,
	"crypto/sha512":   true,
	"crypto/hmac":     true,
	"crypto/rand":     true,
	"crypto/subtle":   true,
	"hash":            true,
	"hash/crc32":      true,
}

const credentialsImportPath = "gofu.dev/gofu/credentials"

func Analyze(path string) (Result, error) {
	fset := token.NewFileSet()
	var diags []Diagnostic
	var runnables []Runnable
	seenRunnables := map[string]token.Position{}

	// Parsed files for second pass (call-site validation).
	type parsedFile struct {
		filePath string
		f        *ast.File
		isTest   bool
	}
	var files []parsedFile

	// Pass 1: parse all files, collect imports/directives/runnables,
	// and build the secret manifest.
	secrets := []string{}
	seenSecrets := map[string]bool{}

	err := filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		name := d.Name()
		f, parseErr := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if parseErr != nil {
			diags = append(diags, Diagnostic{
				File:     filePath,
				Msg:      "parse error: " + parseErr.Error(),
				Severity: Error,
			})
			return nil
		}

		isTest := strings.HasSuffix(name, "_test.go")
		files = append(files, parsedFile{filePath, f, isTest})

		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			if allowedStdlib[importPath] {
				continue
			}
			if strings.HasPrefix(importPath, "gofu.dev/gofu/") {
				continue
			}
			if isTest && testingImports[importPath] {
				continue
			}

			diags = append(diags, Diagnostic{
				File:     filePath,
				Pos:      fset.Position(imp.Path.Pos()),
				Msg:      "import not allowed: " + importPath,
				Severity: Error,
			})
		}

		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.GoStmt:
				diags = append(diags, Diagnostic{
					File:     filePath,
					Pos:      fset.Position(x.Pos()),
					Msg:      "goroutines are not allowed",
					Severity: Error,
				})
			case *ast.ChanType:
				diags = append(diags, Diagnostic{
					File:     filePath,
					Pos:      fset.Position(x.Pos()),
					Msg:      "channels are not allowed",
					Severity: Error,
				})
			case *ast.SendStmt:
				diags = append(diags, Diagnostic{
					File:     filePath,
					Pos:      fset.Position(x.Pos()),
					Msg:      "channel send is not allowed",
					Severity: Error,
				})
			case *ast.UnaryExpr:
				if x.Op == token.ARROW {
					diags = append(diags, Diagnostic{
						File:     filePath,
						Pos:      fset.Position(x.Pos()),
						Msg:      "channel receive is not allowed",
						Severity: Error,
					})
				}
			case *ast.SelectStmt:
				diags = append(diags, Diagnostic{
					File:     filePath,
					Pos:      fset.Position(x.Pos()),
					Msg:      "select statements are not allowed",
					Severity: Error,
				})
			case *ast.FuncDecl:
				if x.Name.Name == "init" && x.Recv == nil {
					diags = append(diags, Diagnostic{
						File:     filePath,
						Pos:      fset.Position(x.Pos()),
						Msg:      "init functions are not allowed",
						Severity: Error,
					})
				}
			}
			return true
		})

		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if strings.HasPrefix(c.Text, "//go:") {
					diags = append(diags, Diagnostic{
						File:     filePath,
						Pos:      fset.Position(c.Pos()),
						Msg:      "go directive not allowed: " + c.Text,
						Severity: Error,
					})
				}
				if !isTest && strings.HasPrefix(c.Text, "//gofu:secret") {
					rest := strings.TrimPrefix(c.Text, "//gofu:secret")
					// rest == "" → exactly "//gofu:secret" (missing key) → malformed
					// rest[0] != ' ' → e.g. "//gofu:secrets" → not our directive
					if rest != "" && rest[0] != ' ' {
						continue
					}
					key := strings.TrimSpace(rest)
					if key == "" || strings.ContainsAny(key, " \t") {
						diags = append(diags, Diagnostic{
							File:     filePath,
							Pos:      fset.Position(c.Pos()),
							Msg:      "malformed gofu:secret directive",
							Severity: Error,
						})
						continue
					}
					if seenSecrets[key] {
						diags = append(diags, Diagnostic{
							File:     filePath,
							Pos:      fset.Position(c.Pos()),
							Msg:      "duplicate secret declaration: " + key,
							Severity: Warning,
						})
						continue
					}
					seenSecrets[key] = true
					secrets = append(secrets, key)
				}
			}
		}

		fileStructs := collectFileStructs(f)

		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if !hasRunnableDirective(d.Doc) {
					continue
				}
				pos := fset.Position(d.Pos())
				if d.Recv != nil {
					diags = append(diags, Diagnostic{
						File: filePath, Pos: pos,
						Msg: "runnable must not be a method", Severity: Error,
					})
					continue
				}
				if !d.Name.IsExported() {
					diags = append(diags, Diagnostic{
						File: filePath, Pos: pos,
						Msg: "runnable function must be exported", Severity: Error,
					})
					continue
				}
				fnName := d.Name.Name
				if prevPos, ok := seenRunnables[fnName]; ok {
					diags = append(diags, Diagnostic{
						File: filePath, Pos: pos,
						Msg:      fmt.Sprintf("duplicate runnable %q (first defined at %s)", fnName, prevPos),
						Severity: Error,
					})
					continue
				}
				seenRunnables[fnName] = pos
				runnables = append(runnables, Runnable{
					Name:    fnName,
					Params:  extractFields(d.Type.Params),
					Returns: extractFields(d.Type.Results),
					File:    filePath,
					Pos:     pos,
				})
				diags = append(diags, validateRunnableReturns(d.Type.Results, filePath, fset, fileStructs)...)
			case *ast.GenDecl:
				if hasRunnableDirective(d.Doc) {
					diags = append(diags, Diagnostic{
						File: filePath, Pos: fset.Position(d.Pos()),
						Msg: "runnable directive must precede a function declaration", Severity: Error,
					})
				}
			}
		}

		return nil
	})
	if err != nil {
		return Result{}, err
	}

	// Pass 2: validate credentials.Get call sites.
	secretSet := map[string]bool{}
	for _, k := range secrets {
		secretSet[k] = true
	}
	for _, pf := range files {
		alias := credentialsAlias(pf.f)
		if alias == "" {
			continue
		}
		ast.Inspect(pf.f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != alias || sel.Sel.Name != "Get" {
				return true
			}
			if len(call.Args) != 1 {
				return true
			}
			pos := fset.Position(call.Pos())
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				diags = append(diags, Diagnostic{
					File:     pf.filePath,
					Pos:      pos,
					Msg:      "credentials.Get requires a string literal",
					Severity: Error,
				})
				return true
			}
			key := strings.Trim(lit.Value, `"`)
			if !secretSet[key] {
				diags = append(diags, Diagnostic{
					File:     pf.filePath,
					Pos:      pos,
					Msg:      fmt.Sprintf("secret %q not declared via //gofu:secret", key),
					Severity: Error,
				})
			}
			return true
		})
	}

	return Result{Diagnostics: diags, Runnables: runnables, Secrets: secrets}, nil
}

// credentialsAlias returns the local identifier for gofu.dev/gofu/credentials in f,
// or "" if the file does not import it.
func credentialsAlias(f *ast.File) string {
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != credentialsImportPath {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		// default: last path segment
		parts := strings.Split(credentialsImportPath, "/")
		return parts[len(parts)-1]
	}
	return ""
}

func hasRunnableDirective(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, c := range doc.List {
		if c.Text == "//gofu:runnable" {
			return true
		}
	}
	return false
}

func collectFileStructs(f *ast.File) map[string]*ast.StructType {
	structs := map[string]*ast.StructType{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			structs[ts.Name.Name] = st
		}
	}
	return structs
}

func validateRunnableReturns(results *ast.FieldList, filePath string, fset *token.FileSet, fileStructs map[string]*ast.StructType) []Diagnostic {
	if results == nil {
		return nil
	}
	var diags []Diagnostic
	for _, field := range results.List {
		expr := field.Type
		pos := fset.Position(expr.Pos())
		switch t := expr.(type) {
		case *ast.FuncType:
			diags = append(diags, Diagnostic{
				File: filePath, Pos: pos,
				Msg:      "runnable return type not allowed: " + types.ExprString(expr),
				Severity: Error,
			})
		case *ast.ChanType:
			diags = append(diags, Diagnostic{
				File: filePath, Pos: pos,
				Msg:      "runnable return type not allowed: " + types.ExprString(expr),
				Severity: Error,
			})
		case *ast.Ident:
			if t.Name == "complex64" || t.Name == "complex128" {
				diags = append(diags, Diagnostic{
					File: filePath, Pos: pos,
					Msg:      "runnable return type not allowed: " + t.Name,
					Severity: Error,
				})
				continue
			}
			st, ok := fileStructs[t.Name]
			if !ok {
				continue
			}
			if allUnexported(st) {
				diags = append(diags, Diagnostic{
					File: filePath, Pos: pos,
					Msg:      "runnable return type has no exported fields: " + t.Name,
					Severity: Warning,
				})
			}
			if isSelfReferential(st, t.Name) {
				diags = append(diags, Diagnostic{
					File: filePath, Pos: pos,
					Msg:      "runnable return type is circular: " + t.Name,
					Severity: Warning,
				})
			}
		}
	}
	return diags
}

func allUnexported(st *ast.StructType) bool {
	if st.Fields == nil || len(st.Fields.List) == 0 {
		return false
	}
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			if ident, ok := f.Type.(*ast.Ident); ok && ident.IsExported() {
				return false
			}
			if star, ok := f.Type.(*ast.StarExpr); ok {
				if ident, ok := star.X.(*ast.Ident); ok && ident.IsExported() {
					return false
				}
			}
			continue
		}
		for _, name := range f.Names {
			if name.IsExported() {
				return false
			}
		}
	}
	return true
}

func isSelfReferential(st *ast.StructType, typeName string) bool {
	if st.Fields == nil {
		return false
	}
	for _, f := range st.Fields.List {
		if star, ok := f.Type.(*ast.StarExpr); ok {
			if ident, ok := star.X.(*ast.Ident); ok && ident.Name == typeName {
				return true
			}
		}
	}
	return false
}

func extractFields(fl *ast.FieldList) []Field {
	if fl == nil {
		return nil
	}
	var fields []Field
	for _, f := range fl.List {
		typeStr := types.ExprString(f.Type)
		if len(f.Names) == 0 {
			fields = append(fields, Field{Type: typeStr})
		} else {
			for _, name := range f.Names {
				fields = append(fields, Field{Name: name.Name, Type: typeStr})
			}
		}
	}
	return fields
}
