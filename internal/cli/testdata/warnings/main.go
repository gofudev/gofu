package main

type secret struct {
	name string
	age  int
}

//gofu:runnable
func GetSecret() secret {
	return secret{}
}
