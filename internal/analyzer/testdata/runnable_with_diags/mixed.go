package runnable_with_diags

import (
	"fmt"
	"os"
)

var _ = os.Getenv

//gofu:runnable
func Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
