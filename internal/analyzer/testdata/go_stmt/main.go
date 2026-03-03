package main

import "fmt"

func doWork() {
	fmt.Println("work")
}

func main() {
	go doWork()
	go func() {
		fmt.Println("anon")
	}()
}
