package wrender

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/boltdb/bolt"
)

type (
	CacheType int
)

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

// Update method updates / creates the cache with the given data.
func (c BoltCaching) Update(data CacheContent) error {
	return c.DB.Update(func(tx *bolt.Tx) error {
		rootBucket, err := tx.CreateBucketIfNotExists([]byte(c.RootBucket))
		if err != nil {
			return err
		}

		hostBucket, err := rootBucket.CreateBucketIfNotExists([]byte(c.HostBucket))
		if err != nil {
			return err
		}

		return hostBucket.Put(CacheContent(c.CachedKey), data)
	})
}

// Read method reads the cached content from the bolt database cache file under
// path {RootBucket}/{HostBucket}/{CachedKey}. It will return the cached content
// if the cache key exists. Otherwise it will return an CacheNotFoundError.
func (c BoltCaching) Read() (CacheContent, error) {
	var data []byte
	err := c.DB.View(func(tx *bolt.Tx) error {
		rootBucket := tx.Bucket([]byte(c.RootBucket))
		if rootBucket == nil {
			return fmt.Errorf("root bucket not found, path: %s", c.path())
		}

		hostBucket := rootBucket.Bucket([]byte(c.HostBucket))
		if hostBucket == nil {
			return fmt.Errorf("host bucket not found, path: %s", c.path())
		}

		data = hostBucket.Get([]byte(c.CachedKey))
		if data == nil {
			return fmt.Errorf("cached key not found, path: %s", c.path())
		}

		return nil
	})
	if err != nil {
		return nil, &CacheNotFoundError{err}
	}

	return CacheContent(data), nil
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

func (c BoltCaching) List() ([]CacheContentInfo, error) {
	var contents []CacheContentInfo

	err := c.DB.View(func(tx *bolt.Tx) error {
		rootBucket := tx.Bucket([]byte(c.RootBucket))
		if rootBucket == nil {
			return &CacheNotFoundError{err: fmt.Errorf("root bucket not found, path: %s", c.path())}
		}

		buckets := []string{}
		if c.HostBucket == "" {
			cursor := rootBucket.Cursor()
			for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
				if bucket := rootBucket.Bucket(k); bucket == nil {
					return fmt.Errorf(
						"boltCache storage error: key %s in root bucket %s is not a bucket",
						string(k),
						c.RootBucket,
					)
				}
				buckets = append(buckets, string(k))
			}
		} else {
			buckets = append(buckets, c.HostBucket)
		}

		var err error
		for _, bucket := range buckets {
			hostBucket := rootBucket.Bucket([]byte(bucket))
			if hostBucket == nil {
				return &CacheNotFoundError{
					err: fmt.Errorf("host bucket not found, path: %s", c.path()),
				}
			}

			err = hostBucket.ForEach(func(k, v []byte) error {
				contents = append(
					contents,
					CacheContentInfo{
						Content: CacheContent(v),
						Path:    filepath.Join(c.RootBucket, bucket, string(k)),
					},
				)
				return nil
			})
			if err != nil {
				break
			}
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	return contents, nil
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
			if err := c.cleanKey(bucket, k, expired); err != nil {
				return err
			}
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
		var cache expiredCache
		if err := json.Unmarshal(entry, &cache); err != nil {
			return err
		}
		if cache.IsExpired() {
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

func (c BoltCaching) path() string {
	return filepath.Join(c.RootBucket, c.HostBucket, c.CachedKey)
}
