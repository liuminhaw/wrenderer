package wrender

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// s3 cache: {CachedPagePrefix}/{hostPath}/{objectKey}
// boltdb cache: {CachedPagePrefix}: bucket, {hostPath}: bucket, {objectKey}: key

const (
	CachedPagePrefix = "page"
)

type WrenderResp struct {
	// Path is in the format of "{CahedPagePrefix}/{host}/{objectKey}"
	Path string `json:"path"`
}

type Wrender struct {
	Target           *url.URL
	Response         *WrenderResp
	ObjectKey        string
	CachedPrefixPath string
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

	key, err := calcKey([]byte(urlParam))
	if err != nil {
		return nil, fmt.Errorf("new wrender: %w", err)
	}

	w := Wrender{
		Target:           target,
		ObjectKey:        key,
		CachedPrefixPath: CachedPagePrefix,
		Response:         &WrenderResp{},
	}
	w.genObjectPath()

	return &w, nil
}

func (w *Wrender) GetHostPath() string {
	host := w.Target.Hostname()
	port := w.Target.Port()
	if port != "" {
		path := strings.Join([]string{host, port}, "_")
		return filepath.Join(w.CachedPrefixPath, path)
	}

	return filepath.Join(w.CachedPrefixPath, host)
}

func calcKey(input []byte) (string, error) {
	h := sha256.New()
	_, err := h.Write(input)
	if err != nil {
		return "", fmt.Errorf("calc key: %w", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (w *Wrender) genObjectPath() {
    hostPath := w.GetHostPath()
    objectPath := filepath.Join(hostPath, w.ObjectKey)

	w.Response.Path = objectPath
}
