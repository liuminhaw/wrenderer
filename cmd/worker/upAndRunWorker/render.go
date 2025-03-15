package upAndRunWorker

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
	"github.com/spf13/viper"
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

func (h *Handler) RenderSitemap(config *viper.Viper, url, jobKey string) {
	defer func() { <-h.Semaphore }() // release semaphore slot

	entries, err := internal.ParseSitemap(url)
	if err != nil {
		h.Logger.Info("Error parsing sitemap", slog.String("url", url))
		err := HandlerError{source: "worker renderSitemap", err: err}
		h.ErrorChan <- &err
		return
	}

	param := fmt.Sprintf("%s/%s", internal.SitemapCategory, jobKey)
	h.Logger.Debug(fmt.Sprintf("Sitemap Job Caching param: %s", param))
	jobCaching, err := wrender.NewBoltCaching(h.DB, param, wrender.CachedJobPrefix, false)
	if err != nil {
		err := HandlerError{source: "worker renderSitemap", err: err}
		h.ErrorChan <- &err
		return
	}

	h.Logger.Info(
		"Sitemap Job Cache",
		slog.String("RootBucket", jobCaching.RootBucket),
		slog.String("HostBucket", jobCaching.HostBucket),
		slog.String("CachedKey", jobCaching.CachedKey),
	)

	ttl := config.GetDuration("semaphore.jobTimeoutInMinutes") * time.Minute
	jobCache := wrender.NewSitemapJobCache(internal.JobStatusProcessing, ttl)
	if err := jobCache.Update(jobCaching, internal.JobStatusProcessing); err != nil {
		err := HandlerError{source: "renderSitemap worker", err: err}
		h.ErrorChan <- &err
		return
	}

	h.Logger.Debug(
		"Sitemap Job Cache updated",
		slog.String(
			"path",
			fmt.Sprintf(
				"%s/%s/%s",
				jobCaching.RootBucket,
				jobCaching.HostBucket,
				jobCaching.CachedKey,
			),
		),
		slog.String("status", internal.JobStatusProcessing),
	)

	// Render each url from the sitemap
	render := renderer.NewRenderer(renderer.WithLogger(h.Logger))
	for _, entry := range entries {
		h.Logger.Debug(fmt.Sprintf("Sitemap rendering: %s start", entry.Loc))

		caching, err := wrender.NewBoltCaching(h.DB, entry.Loc, wrender.CachedPagePrefix, false)
		if err != nil {
			jobCache.Failed = append(jobCache.Failed, entry.Loc)
			err := HandlerError{source: "renderSitemap worker", err: err}
			h.ErrorChan <- &err
			continue
		}

		content, err := render.RenderPage(entry.Loc, rendererOption(config))
		if err != nil {
			jobCache.Failed = append(jobCache.Failed, entry.Loc)
			err := HandlerError{source: "renderSitemap worker", err: err}
			h.ErrorChan <- &err
			continue
		}

		pageCache := wrender.NewPageCached(
			entry.Loc,
			nil,
			config.GetDuration("cache.durationInMinutes")*time.Minute,
		)
		if err := pageCache.Update(caching, content, false); err != nil {
			jobCache.Failed = append(jobCache.Failed, entry.Loc)
			err := HandlerError{source: "renderSitemap worker", err: err}
			h.ErrorChan <- &err
			continue
		}
		h.Logger.Debug(fmt.Sprintf("Sitemap rendering: %s done", entry.Loc))
	}

	jobStatus := internal.JobStatusCompleted
	if len(jobCache.Failed) != 0 {
		jobStatus = internal.JobStatusFailed
	}
	if err := jobCache.Update(jobCaching, jobStatus); err != nil {
		err := HandlerError{source: "renderSitemap worker", err: err}
		h.ErrorChan <- &err
		return
	}

	h.Logger.Debug(
		"Sitemap Job Cache updated",
		slog.String(
			"path",
			fmt.Sprintf(
				"%s/%s/%s",
				jobCaching.RootBucket,
				jobCaching.HostBucket,
				jobCaching.CachedKey,
			),
		),
		slog.String("status", internal.JobStatusCompleted),
	)
}
