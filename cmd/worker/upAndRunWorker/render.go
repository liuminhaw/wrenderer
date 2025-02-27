package upAndRunWorker

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
)

type RenderJobResult struct {
	Content []byte
	Err     error
}

type RenderJob struct {
	Url    string
	Result chan RenderJobResult
}

type Handler struct {
	Logger      *slog.Logger
	DB          *bolt.DB
	RenderQueue chan RenderJob
	Semaphore   chan struct{}
	ErrorChan   chan error
}

func (h *Handler) ErrorListener() {
	h.Logger.Debug("Error Listener started")
	for err := range h.ErrorChan {
		h.Logger.Error(err.Error())
	}
}

type renderSitemapStatus struct {
	Status  string    `json:"status"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func (h *Handler) RenderSitemap(url, jobKey string, ttl time.Duration) {
	defer func() { <-h.Semaphore }() // release semaphore slot

	entries, err := internal.ParseSitemap(url)
	if err != nil {
		h.Logger.Info("Error parsing sitemap", slog.String("url", url))
		err := HandlerError{source: "worker renderSitemap", err: err}
		h.ErrorChan <- &err
		return
	}

	param := fmt.Sprintf("%s/%s", url, jobKey)
	jobCache, err := wrender.NewBoltCaching(h.DB, param, wrender.CachedJobPrefix, false)
	if err != nil {
		err := HandlerError{source: "worker renderSitemap", err: err}
		h.ErrorChan <- &err
		return
	}

	// RootBucket string
	// HostBucket string
	// CachedKey  string
	h.Logger.Info(
		"Sitemap Job Cache",
		slog.String("RootBucket", jobCache.RootBucket),
		slog.String("HostBucket", jobCache.HostBucket),
		slog.String("CachedKey", jobCache.CachedKey),
	)

	// data, err := json.Marshal(renderSitemapStatus{
	// 	Status:  internal.JobStatusProcessing,
	// 	Created: time.Now().UTC(),
	// 	Expires: time.Now().Add(ttl).UTC(),
	// })
	//    if err != nil {
	//        err := HandlerError{source: "worker renderSitemap", err: err}
	//        h.ErrorChan <- &err
	//        return
	//    }

	for _, entry := range entries {
		h.Logger.Info(fmt.Sprintf("Entry: %s", entry.Loc))
	}
}
