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
)

type Wrender struct {
	Target    *url.URL
	UrlKey    string
	CachePath string
}

func NewWrender(urlParam string) (*Wrender, error) {
	if !strings.Contains(urlParam, "://") {
		urlParam = fmt.Sprintf("dummy://%s", urlParam)
	}

	target, err := url.Parse(urlParam)
	if err != nil {
		return nil, fmt.Errorf("new wrender: %w", err)
	}

	if target.Hostname() == "" {
		return nil, fmt.Errorf("new wrender: empty hostname")
	}

	key, err := internal.Sha256Key([]byte(urlParam))
	if err != nil {
		return nil, fmt.Errorf("new wrender: %w", err)
	}

	w := Wrender{
		Target: target,
		UrlKey: key,
	}
	w.genObjectPath()

	return &w, nil
}

func (w *Wrender) GetPrefixPath() string {
	host := w.Target.Hostname()
	port := w.Target.Port()
	if port != "" {
		path := strings.Join([]string{host, port}, "_")
		return filepath.Join(CachedPagePrefix, path)
	}

	return filepath.Join(CachedPagePrefix, host)
}

func (w *Wrender) genObjectPath() {
	hostPath := w.GetPrefixPath()
	w.CachePath = filepath.Join(hostPath, w.UrlKey)
}
