package wrender

import (
	"crypto/rand"
	"fmt"
)

const (
	letters         = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	JobStatusPrefix = "jobs"
)

func RandomKey(prefixLen, suffixLen int) (string, error) {
	prefix, err := randomString(prefixLen)
	if err != nil {
		return "", err
	}

	suffix, err := randomString(suffixLen)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s", prefix, suffix), nil
}

func randomString(n int) (string, error) {
	// Create a byte slice to hold random bytes.
	b := make([]byte, n)
	// Read n random bytes.
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Map each random byte to a character in the allowed letters.
	for i := range n {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b), nil
}
