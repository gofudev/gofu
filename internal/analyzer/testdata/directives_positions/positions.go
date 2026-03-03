//go:build linux

package directives_positions

import "fmt"

//go:noinline
func Hello() {
	fmt.Println("hello")
}
