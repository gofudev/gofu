package main

func main() {
	var ch chan int
	ch <- 42
	_ = <-ch
}

func recv(c <-chan string) {}

func send() chan<- int { return nil }

type S struct {
	c chan bool
}
