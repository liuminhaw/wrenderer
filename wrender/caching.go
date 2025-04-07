package wrender

import "io"

type Caching interface {
	Update(reader io.Reader) error
	UpdateTo(reader io.Reader, suffixPath string) error
	Read() (CacheContent, error)
	Delete() error
}
