package secret_alias

//gofu:secret API_TOKEN

import (
	"fmt"
	creds "gofu.dev/gofu/credentials"
)

func Run() {
	v, err := creds.Get("API_TOKEN")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(v)
}
