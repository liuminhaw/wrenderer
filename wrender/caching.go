package wrender

type Caching interface {
	Update(data CacheContent) error
	Read() (CacheContent, error)
	Delete() error
}
