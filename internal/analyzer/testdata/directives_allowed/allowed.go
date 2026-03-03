package directives_allowed

import "fmt"

// This is a regular comment.
// go:linkname is dangerous — don't use it.
//
//gofu:runnable
func Run() {
	fmt.Println("safe")
}
