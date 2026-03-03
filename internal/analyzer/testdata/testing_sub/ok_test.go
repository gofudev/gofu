package testing_sub

import (
	"testing/fstest"
	"testing/iotest"
	"testing/quick"
)

var _ = fstest.MapFS{}
var _ = iotest.ErrTimeout
var _ = quick.Check
