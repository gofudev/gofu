package secret_undeclared

import (
	"fmt"
	"gofu.dev/gofu/credentials"
)

func Run() {
	v, err := credentials.Get("UNDECLARED_KEY")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(v)
}
