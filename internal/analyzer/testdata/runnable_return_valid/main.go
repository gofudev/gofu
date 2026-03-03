package main

type MyStruct struct {
	Name string
}

//gofu:runnable
func GetInt() int { return 0 }

//gofu:runnable
func GetString() string { return "" }

//gofu:runnable
func GetSlice() []string { return nil }

//gofu:runnable
func GetMap() map[string]int { return nil }

//gofu:runnable
func GetPointer() *MyStruct { return nil }

//gofu:runnable
func GetError() error { return nil }
