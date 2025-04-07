package wrender

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/liuminhaw/wrenderer/internal"
)

// s3 cache: {CachedPagePrefix}/{hostPath}/{objectKey}
// boltdb cache: {CachedPagePrefix}: bucket, {hostPath}: bucket, {objectKey}: key

const (
	CachedPagePrefix = "page"
	CachedJobPrefix  = "jobs"
)

type Wrender struct {
	Target    *url.URL
	UrlKey    string
	CachePath string
	prefix    string
}

// NewWrender creates a new Wrender struct from the given param, the param should
// be a valid URL string that can be parsed into a URL struct. The prefix is used
// for generating the cache object path ({prefix}/{host[_port]}/{hashed key})
func NewWrender(param, prefix string) (*Wrender, error) {
	if !strings.Contains(param, "://") {
		param = fmt.Sprintf("dummy://%s", param)
	}

	target, err := url.Parse(param)
	if err != nil {
		return nil, fmt.Errorf("new wrender: %w", err)
	}

	if target.Hostname() == "" {
		return nil, fmt.Errorf("new wrender: empty hostname")
	}

	key, err := internal.Sha256Key([]byte(param))
	if err != nil {
		return nil, fmt.Errorf("new wrender: %w", err)
	}

	w := Wrender{
		Target: target,
		UrlKey: key,
		prefix: prefix,
	}
	w.genObjectPath()

	return &w, nil
}

func (w *Wrender) GetPrefixPath() string {
	var path string

	host := w.Target.Hostname()
	port := w.Target.Port()
	if port != "" {
		path = strings.Join([]string{host, port}, "_")
	} else {
		path = host
	}

	if w.prefix == "" {
		return path
	}
	return filepath.Join(w.prefix, host)
}

func (w *Wrender) genObjectPath() {
	hostPath := w.GetPrefixPath()
	w.CachePath = filepath.Join(hostPath, w.UrlKey)
}
