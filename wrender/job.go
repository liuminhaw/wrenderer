package wrender

import (
	"crypto/rand"
	"fmt"
)

const (
	letters         = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	JobStatusPrefix = "jobs"
)

// Job path stored in S3 bucket: {JobStatusPrefix}/{Type}/{Key}/{Id}
// For sitemap category, the path will be: jobs/sitemap/{random key}/{queue message id}
type SqsJobCache struct {
	MessageId string
	Key       string
	Category  string
}

func NewSqsJobCache(id, key, category string) *SqsJobCache {
	return &SqsJobCache{
		MessageId: id,
		Key:       key,
		Category:  category,
	}
}

func (j *SqsJobCache) KeyPath() string {
	return fmt.Sprintf("%s/%s/%s", JobStatusPrefix, j.Category, j.Key)
}

func (j *SqsJobCache) QueuedPath() string {
	return fmt.Sprintf("%s/%s/%s/queued/%s", JobStatusPrefix, j.Category, j.Key, j.MessageId)
}

func (j *SqsJobCache) ProcessPath() string {
	return fmt.Sprintf("%s/%s/%s/process/%s", JobStatusPrefix, j.Category, j.Key, j.MessageId)
}

func (j *SqsJobCache) FailurePath() string {
	return fmt.Sprintf("%s/%s/%s/failed/%s", JobStatusPrefix, j.Category, j.Key, j.MessageId)
}

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
