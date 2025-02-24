package upAndRunWorker

import (
	"fmt"
	"log/slog"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
)

type Handler struct {
	Logger    *slog.Logger
	DB        *bolt.DB
	Semaphore chan struct{}
	ErrorChan chan error
}

func (h *Handler) ErrorListener() {
    h.Logger.Debug("Error Listener started")
	for err := range h.ErrorChan {
		h.Logger.Error(err.Error())
	}
}

func (h *Handler) RenderSitemap(url, jobKey string) {
	defer func() { <-h.Semaphore }()

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

	for _, entry := range entries {
		h.Logger.Info(fmt.Sprintf("Entry: %s", entry.Loc))
	}
}
