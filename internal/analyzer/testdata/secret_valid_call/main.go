package secret_valid_call

//gofu:secret API_KEY

import (
	"fmt"
	"gofu.dev/gofu/credentials"
)

func Run() {
	v, err := credentials.Get("API_KEY")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(v)
}
