package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var validName = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

func Init(args []string, stderr io.Writer) int {
	owner := "gofu"
	var name string

	for i := 0; i < len(args); i++ {
		if args[i] == "--owner" {
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "usage: gofu init <name> [--owner <owner>]")
				return 2
			}
			owner = args[i+1]
			i++
		} else {
			if name != "" {
				_, _ = fmt.Fprintln(stderr, "usage: gofu init <name> [--owner <owner>]")
				return 2
			}
			name = args[i]
		}
	}

	if name == "" {
		_, _ = fmt.Fprintln(stderr, "usage: gofu init <name> [--owner <owner>]")
		return 2
	}

	if !validName.MatchString(name) {
		_, _ = fmt.Fprintf(stderr, "gofu init: invalid module name %q (must match [a-z][a-z0-9]*)\n", name)
		return 1
	}

	if err := os.Mkdir(name, 0755); err != nil {
		if os.IsExist(err) {
			_, _ = fmt.Fprintf(stderr, "gofu init: directory %q already exists\n", name)
			return 1
		}
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	goMod := fmt.Sprintf("module gofu.dev/%s/%s\n\ngo 1.23\n", owner, name)
	if err := os.WriteFile(filepath.Join(name, "go.mod"), []byte(goMod), 0644); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	goFile := fmt.Sprintf(`package %s

import "fmt"

//gofu:runnable
func Hello() {
	fmt.Println("hello from %s")
}
`, name, name)
	if err := os.WriteFile(filepath.Join(name, name+".go"), []byte(goFile), 0644); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	return 0
}
