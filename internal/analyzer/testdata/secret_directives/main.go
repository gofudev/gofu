package secret_directives

//gofu:secret SLACK_TOKEN
//gofu:secret DATABASE_URL

import "fmt"

func Run() {
	fmt.Println("ok")
}
