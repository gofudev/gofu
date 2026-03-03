package directives_test_file

import "testing"

//go:linkname foo runtime.foo
func TestHello(t *testing.T) {}
