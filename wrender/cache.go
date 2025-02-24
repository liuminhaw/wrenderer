package wrender

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/boltdb/bolt"
)

type CacheType int

// BoltCached stores the cached content, creation time, and expiration time.
// This is the information that will be stored in the cache file using JSON format.
type BoltCached struct {
	Url     string    `json:"url"`
	Content []byte    `json:"content"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func NewBoltCached(url string, content []byte, ttl time.Duration) BoltCached {
	return BoltCached{
		Url:     url,
		Content: content,
		Created: time.Now().UTC(),
		Expires: time.Now().Add(ttl).UTC(),
	}
}

// BoltCaching is a struct that holds the path to the cached file.
type BoltCaching struct {
	DB         *bolt.DB
	RootBucket string
	HostBucket string
	CachedKey  string
}

// NewBoltCaching creates a Caching struct from the given param (which can be parsed
// into a URL struct). cachePrefix will be use to create the cache path with
// format "{RootBucket}/{HostBucket}/{CachedKey}" -> {cachePrefix}/{host domain}/{hashed param key}.
// The cacheType is for bolt type determination (Bucket or Entry)
func NewBoltCaching(
	db *bolt.DB,
	param string,
	cachedPrefix string,
	bucketCache bool,
) (BoltCaching, error) {
	render, err := NewWrender(param, cachedPrefix)
	if err != nil {
		return BoltCaching{}, err
	}

	parts := strings.Split(render.CachePath, "/")
	if len(parts) != 3 {
		return BoltCaching{}, fmt.Errorf("invalid input path: %s", render.CachePath)
	}

	cachedKey := parts[2]
	if bucketCache {
		cachedKey = ""
	}

	return BoltCaching{
		DB:         db,
		RootBucket: parts[0],
		HostBucket: parts[1],
		CachedKey:  cachedKey,
	}, nil
}

func (c BoltCaching) Update(cached BoltCached) error {
	return c.DB.Update(func(tx *bolt.Tx) error {
		rootBucket, err := tx.CreateBucketIfNotExists([]byte(c.RootBucket))
		if err != nil {
			return err
		}

		hostBucket, err := rootBucket.CreateBucketIfNotExists([]byte(c.HostBucket))
		if err != nil {
			return err
		}

		data, err := json.Marshal(cached)
		if err != nil {
			return err
		}

		return hostBucket.Put([]byte(c.CachedKey), data)
	})
}

func (c BoltCaching) Read() (BoltCached, error) {
	var cached BoltCached

	err := c.DB.View(func(tx *bolt.Tx) error {
		rootBucket := tx.Bucket([]byte(c.RootBucket))
		if rootBucket == nil {
			return fmt.Errorf("root bucket %s not found", c.RootBucket)
		}

		hostBucket := rootBucket.Bucket([]byte(c.HostBucket))
		if hostBucket == nil {
			return fmt.Errorf("host bucket %s not found", c.HostBucket)
		}

		val := hostBucket.Get([]byte(c.CachedKey))
		if val == nil {
			return fmt.Errorf("cached key %s not found", c.CachedKey)
		}
		json.Unmarshal(val, &cached)

		return nil
	})
	if err != nil {
		return BoltCached{}, fmt.Errorf("caching read: %w", err)
	}

	return cached, nil
}

// Cleanup removes all expired cache entries.
func (c BoltCaching) Cleanup() error {
	return c.DB.Update(func(tx *bolt.Tx) error {
		rootBucket := tx.Bucket([]byte(c.RootBucket))
		if rootBucket == nil {
			return nil
		}

		return c.cleanBucket(rootBucket, true)
	})
}

// Delete removes the cache with the given entry. The targetType parameter specifies
// what type of cache to delete. If teh targetType is TargetTypeBucket, it will remove
// all the cache entries in the bucket. If the targetType is TargetTypeEntry, it will
// only remove that cache entry.
func (c BoltCaching) Delete() error {
	return c.DB.Update(func(tx *bolt.Tx) error {
		rootBucket := tx.Bucket([]byte(c.RootBucket))
		if rootBucket == nil {
			return fmt.Errorf("root bucket %s not found", c.RootBucket)
		}

		hostBucket := rootBucket.Bucket([]byte(c.HostBucket))
		if hostBucket == nil {
			return fmt.Errorf("host bucket %s not found", c.HostBucket)
		}

		if c.CachedKey == "" {
			return c.cleanBucket(hostBucket, false)
		} else {
			return c.cleanKey(hostBucket, []byte(c.CachedKey), false)
		}
	})
}

func (c BoltCaching) cleanBucket(bucket *bolt.Bucket, expired bool) error {
	return bucket.ForEach(func(k, v []byte) error {
		if v == nil {
			// This is a nested bucket
			nestedBucket := bucket.Bucket(k)
			if err := c.cleanBucket(nestedBucket, expired); err != nil {
				return err
			}
		} else {
			c.cleanKey(bucket, k, expired)
		}

		return nil
	})
}

// cleanKey remove the cache entry key from the bucket. If the expired is set to true,
// it will check for the expireation time of the cache and only remove the cache if
// it is expired. Otherwise, it will remove the cache entry regardless of the
// expiration time.
func (c BoltCaching) cleanKey(bucket *bolt.Bucket, key []byte, expired bool) error {
	entry := bucket.Get(key)
	if entry == nil {
		return nil
	}

	if expired {
		var cache BoltCached
		if err := json.Unmarshal(entry, &cache); err != nil {
			return err
		}
		if time.Now().After(cache.Expires) {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
	} else {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}

	return nil
}

type BoltCachedInfo struct {
	Path    string    `json:"path"`
	Url     string    `json:"url"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func (c BoltCaching) List() ([]BoltCachedInfo, error) {
	var caches []BoltCachedInfo

	err := c.DB.View(func(tx *bolt.Tx) error {
		rootBucket := tx.Bucket([]byte(c.RootBucket))
		if rootBucket == nil {
			return fmt.Errorf("root bucket %s not found", c.RootBucket)
		}

		hostBucket := rootBucket.Bucket([]byte(c.HostBucket))
		if hostBucket == nil {
			return fmt.Errorf("host bucket %s not found", c.HostBucket)
		}

		return hostBucket.ForEach(func(k, v []byte) error {
			var cached BoltCached
			if err := json.Unmarshal(v, &cached); err != nil {
				return err
			}
			caches = append(caches, BoltCachedInfo{
				Path:    filepath.Join(c.RootBucket, c.HostBucket, string(k)),
				Url:     cached.Url,
				Created: cached.Created,
				Expires: cached.Expires,
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return caches, nil
}

// IsValid checks if the cache key exists and if the cached is regarded as expired.
// Returns true if the cache key exists and the cache is not expired.
// Returns false otherwise.
func (c BoltCaching) IsValid() (bool, error) {
	exists, err := c.exists()
	if err != nil {
		return false, fmt.Errorf("caching validity check: %w", err)
	}
	if !exists {
		return false, nil
	}

	expired, err := c.expired()
	if err != nil {
		return false, fmt.Errorf("caching validity check: %w", err)
	}
	return !expired, nil
}

// exists checks the existence of the cache key.
func (c BoltCaching) exists() (bool, error) {
	var exists bool
	err := c.DB.View(func(tx *bolt.Tx) error {
		rootBucket := tx.Bucket([]byte(c.RootBucket))
		if rootBucket == nil {
			return nil
		}

		bucket := rootBucket.Bucket([]byte(c.HostBucket))
		if bucket == nil {
			return nil
		}

		data := bucket.Get([]byte(c.CachedKey))
		exists = data != nil
		return nil
	})
	if err != nil {
		return exists, fmt.Errorf("caching existence check: %w", err)
	}
	return exists, nil
}

// expired checks if the cached content is expired
// by comparing the current time with the expiration time.
func (c BoltCaching) expired() (bool, error) {
	var expired bool

	err := c.DB.View(func(tx *bolt.Tx) error {
		rootBucket := tx.Bucket([]byte(c.RootBucket))
		if rootBucket == nil {
			return nil
		}

		bucket := rootBucket.Bucket([]byte(c.HostBucket))
		if bucket == nil {
			return nil
		}

		data := bucket.Get([]byte(c.CachedKey))
		if data == nil {
			return nil
		}

		var info BoltCached
		if err := json.Unmarshal(data, &info); err != nil {
			return err
		}

		expired = time.Now().After(info.Expires)
		return nil
	})
	if err != nil {
		return expired, fmt.Errorf("caching expiration check: %w", err)
	}

	return expired, err
}
