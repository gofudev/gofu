package main

import "fmt"

type T struct{}

func (t *T) init() {
	fmt.Println("method init")
}

func main() {}
