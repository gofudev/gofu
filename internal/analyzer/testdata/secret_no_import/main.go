package secret_no_import

import "fmt"

// No credentials import — no call-site checking should occur.
func Run() {
	fmt.Println("UNDECLARED_KEY")
}
