package main

func main() {
	ch := make(chan int)
	select {
	case v := <-ch:
		_ = v
	default:
	}
}
