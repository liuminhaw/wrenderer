package wrender

type CacheNotFoundError struct {
	err error
}

func (e *CacheNotFoundError) Error() string {
	return e.err.Error()
}
