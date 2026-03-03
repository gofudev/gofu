package runnable_malformed

// gofu:runnable
func Spaced() string {
	return "ignored"
}

// gofu:Runnable
func WrongCase() string {
	return "ignored"
}

// GOFU:runnable
func AllCaps() string {
	return "ignored"
}
