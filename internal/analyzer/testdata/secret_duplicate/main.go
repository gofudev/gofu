package secret_duplicate

//gofu:secret MY_KEY
//gofu:secret MY_KEY

import "fmt"

func Run() {
	fmt.Println("ok")
}
