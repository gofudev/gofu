package runnable_valid

import "fmt"

//gofu:runnable
func Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
