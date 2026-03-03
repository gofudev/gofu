package runnable_multi_params

//gofu:runnable
func Version() string {
	return "1.0"
}

//gofu:runnable
func Divide(a, b float64) (result float64, err error) {
	if b == 0 {
		return 0, nil
	}
	return a / b, nil
}
