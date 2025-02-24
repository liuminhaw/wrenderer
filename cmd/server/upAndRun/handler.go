package upAndRun

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/liuminhaw/wrenderer/cmd/server/shared"
	"github.com/liuminhaw/wrenderer/cmd/worker/upAndRunWorker"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
	"github.com/spf13/viper"
)

const (
	statusKeyLength = 6
)

func (app *application) pageRenderWithConfig(config *viper.Viper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get query parameters
		url := r.URL.Query().Get("url")
		app.logger.Info(
			fmt.Sprintf("url: %s", url),
			slog.String("request", r.URL.String()),
			slog.String("method", r.Method),
		)
		if url == "" {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		caching, err := wrender.NewBoltCaching(
			app.db,
			url,
			wrender.CachedPagePrefix,
			false,
		)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		app.logger.Debug("Checking cache", slog.String("url", url))
		exists, err := caching.IsValid()
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		app.logger.Debug("Checking cache done", slog.String("url", url))
		if exists {
			app.logger.Debug(
				"Cache exists",
				slog.String("RootBucket", caching.RootBucket),
				slog.String("HostBucket", caching.HostBucket),
				slog.String("CachedKey", caching.CachedKey),
			)
			cached, err := caching.Read()
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			decompressContent, err := internal.Decompress(cached.Content)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			w.Write(decompressContent)
			return
		} else {
			app.logger.Debug(
				"Cache not exists",
				slog.String("RootBucket", caching.RootBucket),
				slog.String("HostBucket", caching.HostBucket),
				slog.String("CachedKey", caching.CachedKey),
			)

			// Add render job to queue
			var result renderJobResult
			job := renderJob{url: url, result: make(chan renderJobResult, 1)}
			select {
			case app.renderQueue <- job:
				// Wait for the job to be processed
				app.logger.Info("Job added to queue", slog.String("url", url))
				result = <-job.result
				if result.err != nil {
					app.serverError(w, r, result.err)
					return
				}
			default:
				app.clientError(w, http.StatusTooManyRequests)
				return
			}

			// Save the rendered page to cache
			compressedContent, err := internal.Compress(result.content)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			cacheDuration := config.GetInt("cache.durationInMinutes")
			cacheItem := wrender.NewBoltCached(
				url,
				compressedContent,
				time.Duration(cacheDuration)*time.Minute,
			)

			if err := caching.Update(cacheItem); err != nil {
				app.serverError(w, r, err)
				return
			}

			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write(result.content)
		}
	}
}

type renderedCachesResponse struct {
	Caches []wrender.BoltCachedInfo `json:"caches"`
}

func (app *application) listRenderedCaches(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	domain := r.URL.Query().Get("domain")
	app.logger.Info(
		fmt.Sprintf("domain: %s", domain),
		slog.String("request", r.URL.String()),
		slog.String("method", r.Method),
	)
	if domain == "" {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	caching, err := wrender.NewBoltCaching(
		app.db,
		domain,
		wrender.CachedPagePrefix,
		true,
	)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	caches, err := caching.List()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	cachesResponse := renderedCachesResponse{}
	for _, cache := range caches {
		cachesResponse.Caches = append(cachesResponse.Caches, cache)
	}

	response, err := json.Marshal(cachesResponse)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func (app *application) deleteRenderedCache(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	app.logger.Info(
		"delete rendered cache",
		slog.String("request", r.URL.String()),
		slog.String("method", r.Method),
	)
	urlParam := r.URL.Query().Get("url")
	domainParam := r.URL.Query().Get("domain")
	app.logger.Debug(
		"Delete rendered cache",
		slog.String("url param", urlParam),
		slog.String("domain param", domainParam),
	)

	if urlParam == "" && domainParam == "" {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	var caching wrender.BoltCaching
	var param string
	var targetBucket bool
	switch {
	case domainParam != "":
		param = domainParam
		targetBucket = true
	case urlParam != "":
		param = urlParam
		targetBucket = false
	default:
		app.clientError(w, http.StatusBadRequest)
		return
	}

	caching, err := wrender.NewBoltCaching(app.db, param, wrender.CachedPagePrefix, targetBucket)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if err := caching.Delete(); err != nil {
		app.serverError(w, r, err)
		return
	}
}

type renderSitemapStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

func (app *application) renderSitemap(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	defer r.Body.Close()

	var payload shared.RenderSitemapPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		app.logger.Info(
			"Failed to unmarhsal request body",
			slog.String("request body", string(body)),
		)
		app.clientError(w, http.StatusBadRequest)
		return
	}

	if !internal.ValidUrl(payload.SitemapUrl) {
		app.logger.Info("Invalid sitemap url", slog.String("sitemap url", payload.SitemapUrl))
		app.clientError(w, http.StatusBadRequest)
		return
	}

	var renderKey string
	select {
	case app.sitemapSemaphore <- struct{}{}:
		var err error
		renderKey, err = wrender.RandomKey(statusKeyLength, statusKeyLength)
		if err != nil {
			<-app.sitemapSemaphore // release semaphore slot
			app.serverError(w, r, err)
			return
		}

		workerHandler := upAndRunWorker.Handler{
			Logger:    app.logger,
			DB:        app.db,
			Semaphore: app.sitemapSemaphore,
			ErrorChan: app.errorChan,
		}
		go workerHandler.RenderSitemap(payload.SitemapUrl, renderKey)
	default:
		app.clientError(w, http.StatusTooManyRequests)
		return
	}

	msg := fmt.Sprintf(
		"{\"message\": \"Sitemap rendering accepted\", \"location\": \"/render/sitemap/%s/status\"}",
		renderKey,
	)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/render/sitemap/%s/status", renderKey))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(msg))
}
