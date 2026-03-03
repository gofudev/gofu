//go:build linux && amd64

package directives_blocked

import "fmt"

//go:linkname localname runtime.localname
func linked() {}

//go:noescape
func escaped()

//go:generate stringer -type=Foo
type Foo int

//go:embed template.html
var tmpl string

func main() {
	fmt.Println("hello")
}
