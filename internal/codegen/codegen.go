package codegen

import (
	"fmt"
	"go/format"
	"strings"
	"text/template"

	"gofu.dev/gofu/internal/analyzer"
)

func Generate(runnables []analyzer.Runnable, modulePath string) ([]byte, error) {
	var callPrefix string
	if modulePath != "" {
		callPrefix = "userpkg."
	}
	data := templateData{
		ModulePath: modulePath,
		Runnables:  make([]runnableData, len(runnables)),
	}
	for i, r := range runnables {
		if len(r.Params) > 0 {
			data.NeedsIO = true
		}
		data.Runnables[i] = runnableData{
			Name:     r.Name,
			CaseBody: buildCaseBody(r, callPrefix),
		}
	}
	var buf strings.Builder
	if err := mainTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execute: %w", err)
	}
	return format.Source([]byte(buf.String()))
}

type templateData struct {
	ModulePath string
	NeedsIO    bool
	Runnables  []runnableData
}

type runnableData struct {
	Name     string
	CaseBody string
}

var mainTmpl = template.Must(template.New("main").Parse(mainTemplate))

const mainTemplate = `package main

import (
	"encoding/json"
	"fmt"
{{if .NeedsIO}}	"io"
{{end}}	"os"
{{if .ModulePath}}
	userpkg "{{.ModulePath}}"
{{end}})

func main() {
	fd3 := os.NewFile(3, "result")

	defer func() {
		if r := recover(); r != nil {
			errJSON, _ := json.Marshal(map[string]string{"error": fmt.Sprint(r)})
			if _, err := fd3.Write(append(errJSON, '\n')); err != nil {
				fmt.Fprintln(os.Stderr, "fd 3 write error:", err)
			}
			fmt.Fprintln(os.Stderr, r)
			os.Exit(2)
		}
	}()

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <function-name>\n", os.Args[0])
		os.Exit(1)
	}

	switch os.Args[1] {
{{range .Runnables}}	case "{{.Name}}":
		{{.CaseBody}}
{{end}}	default:
		fmt.Fprintf(os.Stderr, "unknown function: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "available functions:{{range .Runnables}} {{.Name}}{{end}}")
		os.Exit(1)
	}
}
`

func paramFieldNames(f analyzer.Field, i int) (string, string) {
	if f.Name == "" {
		tag := fmt.Sprintf("p%d", i)
		return strings.ToUpper(tag[:1]) + tag[1:], tag
	}
	return strings.ToUpper(f.Name[:1]) + f.Name[1:], f.Name
}

func buildCaseBody(r analyzer.Runnable, callPrefix string) string {
	returns := r.Returns
	hasError := len(returns) > 0 && returns[len(returns)-1].Type == "error"
	resultCount := len(returns)
	if hasError {
		resultCount--
	}

	var b strings.Builder

	// Parameter deserialization
	var callArgs string
	if len(r.Params) > 0 {
		b.WriteString("type params struct {\n")
		var args []string
		for i, p := range r.Params {
			fieldName, jsonTag := paramFieldNames(p, i)
			fmt.Fprintf(&b, "%s %s `json:\"%s\"`\n", fieldName, p.Type, jsonTag)
			args = append(args, "p."+fieldName)
		}
		b.WriteString("}\n")
		b.WriteString(`var p params
inputData, readErr := io.ReadAll(os.Stdin)
if readErr != nil {
errJSON, _ := json.Marshal(map[string]string{"error": readErr.Error()})
if _, werr := fd3.Write(append(errJSON, '\n')); werr != nil {
fmt.Fprintln(os.Stderr, "fd 3 write error:", werr)
}
fmt.Fprintln(os.Stderr, "stdin read error:", readErr)
os.Exit(1)
}
if unmarshalErr := json.Unmarshal(inputData, &p); unmarshalErr != nil {
errJSON, _ := json.Marshal(map[string]string{"error": unmarshalErr.Error()})
if _, werr := fd3.Write(append(errJSON, '\n')); werr != nil {
fmt.Fprintln(os.Stderr, "fd 3 write error:", werr)
}
fmt.Fprintln(os.Stderr, "json unmarshal error:", unmarshalErr)
os.Exit(1)
}
`)
		callArgs = strings.Join(args, ", ")
	}

	// Function call
	var vars []string
	for i := range resultCount {
		vars = append(vars, fmt.Sprintf("r%d", i))
	}
	if hasError {
		vars = append(vars, "err")
	}
	if len(vars) > 0 {
		b.WriteString(strings.Join(vars, ", "))
		b.WriteString(" := ")
	}
	b.WriteString(callPrefix + r.Name + "(" + callArgs + ")\n")

	// Error check
	if hasError {
		b.WriteString(`if err != nil {
errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
if _, werr := fd3.Write(append(errJSON, '\n')); werr != nil {
fmt.Fprintln(os.Stderr, "fd 3 write error:", werr)
os.Exit(1)
}
fmt.Fprintln(os.Stderr, err)
os.Exit(1)
}
`)
	}

	// Result output
	if resultCount == 0 {
		b.WriteString(`if _, werr := fd3.Write([]byte("null\n")); werr != nil {
fmt.Fprintln(os.Stderr, "fd 3 write error:", werr)
os.Exit(1)
}`)
	} else {
		var resultExpr string
		if resultCount == 1 {
			resultExpr = "r0"
		} else {
			exprs := make([]string, resultCount)
			for i := range exprs {
				exprs[i] = fmt.Sprintf("r%d", i)
			}
			resultExpr = "[]any{" + strings.Join(exprs, ", ") + "}"
		}
		fmt.Fprintf(&b, `resultJSON, marshalErr := json.Marshal(%s)
if marshalErr != nil {
fmt.Fprintln(os.Stderr, "json marshal error:", marshalErr)
os.Exit(1)
}
if _, werr := fd3.Write(append(resultJSON, '\n')); werr != nil {
fmt.Fprintln(os.Stderr, "fd 3 write error:", werr)
os.Exit(1)
}`, resultExpr)
	}

	return b.String()
}
