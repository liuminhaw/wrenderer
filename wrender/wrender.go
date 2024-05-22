package wrender

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
)

const (
	cachedPagePrefix = "page"
)

type WrenderResp struct {
	Path string `json:"path"`
}

type Wrender struct {
	Target           *url.URL
	Response         *WrenderResp
	ObjectKey        string
	CachedPrefixPath string
}

func NewWrender(urlParam string) (*Wrender, error) {
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
		CachedPrefixPath: cachedPagePrefix,
		Response:         &WrenderResp{},
	}
	w.genObjectPath()

	return &w, nil
}

func (w *Wrender) GetHostPath() string {
	host := w.Target.Hostname()
	return strings.Join([]string{w.CachedPrefixPath, host}, "/")
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
	var objectPath string
	host := w.Target.Hostname()
	port := w.Target.Port()
	if port != "" {
		objectPath = strings.Join([]string{host, port}, "_")
		objectPath = strings.Join([]string{w.CachedPrefixPath, objectPath, w.ObjectKey}, "/")
	} else {
		objectPath = strings.Join([]string{w.CachedPrefixPath, host, w.ObjectKey}, "/")
	}

	w.Response.Path = objectPath
}
