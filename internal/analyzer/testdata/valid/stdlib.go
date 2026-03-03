package valid

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"strings"
)

var (
	_ = fmt.Sprint
	_ = strings.Contains
	_ json.Marshaler
	_ = sha256.New
	_ hash.Hash
)
