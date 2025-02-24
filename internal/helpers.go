package internal

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"

	"github.com/liuminhaw/sitemapHelper"
)

func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, fmt.Errorf("compress: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("compress: %w", err)
	}

	return buf.Bytes(), nil
}

func Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}

	return buf.Bytes(), nil
}

func Sha256Key(input []byte) (string, error) {
	h := sha256.New()
	_, err := h.Write(input)
	if err != nil {
		return "", fmt.Errorf("calc key: %w", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func ValidUrl(str string) bool {
	parsedUrl, err := url.Parse(str)
	if err != nil {
		return false
	}

	if parsedUrl.Scheme == "" || parsedUrl.Host == "" {
		return false
	}
	return true
}

func ParseSitemap(url string) ([]sitemapHelper.UrlEntry, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	entries, err := sitemapHelper.ParseSitemap(resp.Body)
	if err != nil {
		return nil, err
	}

	return entries, nil
}
