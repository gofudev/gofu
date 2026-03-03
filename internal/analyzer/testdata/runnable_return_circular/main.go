package main

type Node struct {
	Value int
	Next  *Node
}

//gofu:runnable
func GetNode() Node {
	return Node{}
}
