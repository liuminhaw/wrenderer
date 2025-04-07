package wrender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/liuminhaw/wrenderer/internal"
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
	cache := PageCached{
		Url:     url,
		Created: time.Now().UTC(),
		Expires: time.Now().Add(ttl).UTC(),
	}

	if content != nil {
		cache.Content = content
	}

	return cache
}

// IsExpired checks if the cache has expired.
func (p PageCached) IsExpired() bool {
	return time.Now().UTC().After(p.Expires)
}

func (p *PageCached) Update(caching Caching, content []byte, compressed bool) error {
	if compressed {
		p.Content = content
	} else {
		content, err := internal.Compress(content)
		if err != nil {
			return err
		}
		p.Content = content
	}

	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	return caching.Update(bytes.NewReader(data))
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

type SitemapJobCache struct {
	Status  string    `json:"status"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
	Failed  []string  `json:"failed,omitempty"`
}

func NewSitemapJobCache(status string, ttl time.Duration) SitemapJobCache {
	return SitemapJobCache{
		Status:  status,
		Created: time.Now().UTC(),
		Expires: time.Now().Add(ttl).UTC(),
		Failed:  []string{},
	}
}

func (c SitemapJobCache) IsExpired() bool {
	return time.Now().UTC().After(c.Expires)
}

func (c *SitemapJobCache) Update(caching Caching, status string) error {
	c.Status = status

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return caching.Update(bytes.NewReader(data))
}

type SqsJobPayload struct {
	TargetUrl string `json:"targetUrl"`
	RandomKey string `json:"randomKey"`
}

type SqsJobCache struct {
	MessageId string
	Key       string
	Category  string
	Prefix    string
}

func NewSqsJobCache(key, category, cachedPrefix string) *SqsJobCache {
	return &SqsJobCache{
		Key:      key,
		Category: category,
		Prefix:   cachedPrefix,
	}
}

func (c *SqsJobCache) KeyPath() string {
	return fmt.Sprintf("%s/%s/%s", c.Prefix, c.Category, c.Key)
}

type JobCachedInfo struct {
	Path    string    `json:"path"`
	Status  string    `json:"status"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func JobsCachesConversion(cachesInfo []CacheContentInfo) ([]JobCachedInfo, error) {
	var jCachesInfo []JobCachedInfo

	for _, info := range cachesInfo {
		var jCache SitemapJobCache
		if err := json.Unmarshal(info.Content, &jCache); err != nil {
			return nil, err
		}
		jCachesInfo = append(jCachesInfo, JobCachedInfo{
			Path:    info.Path,
			Status:  jCache.Status,
			Created: jCache.Created,
			Expires: jCache.Expires,
		})
	}

	return jCachesInfo, nil
}

type expiredCache struct {
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func (c expiredCache) IsExpired() bool {
	return time.Now().UTC().After(c.Expires)
}
