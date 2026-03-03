package secret_nonliteral

//gofu:secret MY_KEY

import (
	"fmt"
	"gofu.dev/gofu/credentials"
)

var keyName = "MY_KEY"

func Run() {
	v, err := credentials.Get(keyName)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(v)
}
