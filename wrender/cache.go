package wrender

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/boltdb/bolt"
)

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
	db         *bolt.DB
	RootBucket string
	HostBucket string
	CachedKey  string
}

// NewBoltCaching creates a Caching struct from the given path.
// The path should have the format of "{RootBucket}/{HostBucket}/{CachedKey}".
// Each part of the path is separated by a slash and will be assigned to the struct fields.
func NewBoltCaching(db *bolt.DB, param string) (BoltCaching, error) {
	render, err := NewWrender(param)
	if err != nil {
		return BoltCaching{}, err
	}

	parts := strings.Split(render.CachePath, "/")
	if len(parts) != 3 {
		return BoltCaching{}, fmt.Errorf("invalid input path: %s", render.CachePath)
	}

	return BoltCaching{
		db:         db,
		RootBucket: parts[0],
		HostBucket: parts[1],
		CachedKey:  parts[2],
	}, nil
}

func (c BoltCaching) Update(cached BoltCached) error {
	return c.db.Update(func(tx *bolt.Tx) error {
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

	err := c.db.View(func(tx *bolt.Tx) error {
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

type BoltCachedInfo struct {
	Path    string    `json:"path"`
	Url     string    `json:"url"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func (c BoltCaching) List() ([]BoltCachedInfo, error) {
	var caches []BoltCachedInfo

	err := c.db.View(func(tx *bolt.Tx) error {
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
	err := c.db.View(func(tx *bolt.Tx) error {
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

	err := c.db.View(func(tx *bolt.Tx) error {
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
