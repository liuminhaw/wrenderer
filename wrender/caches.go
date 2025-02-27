package wrender

import (
	"encoding/json"
	"time"
)

type CacheContent []byte

type CacheContentInfo struct {
	Content CacheContent
	Path    string
}

type Caches interface {
	IsExpired() bool
}

// PageCached stores the source url, rendered content, creation time,
// and expiration time of the generated page cache.
type PageCached struct {
	Url     string    `json:"url"`
	Content []byte    `json:"content"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func NewPageCached(url string, content []byte, ttl time.Duration) PageCached {
	return PageCached{
		Url:     url,
		Content: content,
		Created: time.Now().UTC(),
		Expires: time.Now().Add(ttl).UTC(),
	}
}

// IsExpired checks if the cache has expired.
func (p *PageCached) IsExpired() bool {
	return time.Now().UTC().After(p.Expires)
}

type PageCachedInfo struct {
	Path    string    `json:"path"`
	Url     string    `json:"url"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func PagesCachesConversion(cachesInfo []CacheContentInfo) ([]PageCachedInfo, error) {
	var pCachesInfo []PageCachedInfo

	for _, info := range cachesInfo {
		var pCache PageCached
		if err := json.Unmarshal(info.Content, &pCache); err != nil {
			return nil, err
		}
		pCachesInfo = append(pCachesInfo, PageCachedInfo{
			Path:    info.Path,
			Url:     pCache.Url,
			Created: pCache.Created,
			Expires: pCache.Expires,
		})
	}

	return pCachesInfo, nil
}

type expiredCache struct {
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func (c expiredCache) IsExpired() bool {
	return time.Now().UTC().After(c.Expires)
}
