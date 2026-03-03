package credentials

import (
	"fmt"
	"os"
)

func Get(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("secret not set: %s", key)
	}
	return v, nil
}
