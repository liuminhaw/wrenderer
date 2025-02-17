package upAndRun

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
	"github.com/spf13/viper"
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

		caching, err := wrender.NewBoltCaching(app.db, url, wrender.BoltEntry)
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

	caching, err := wrender.NewBoltCaching(app.db, domain, wrender.BoltBucket)
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
	var cacheType wrender.CacheType
	switch {
	case domainParam != "":
		param = domainParam
		cacheType = wrender.BoltBucket
	case urlParam != "":
		param = urlParam
		cacheType = wrender.BoltEntry
	default:
		app.clientError(w, http.StatusBadRequest)
		return
	}

	caching, err := wrender.NewBoltCaching(app.db, param, cacheType)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if err := caching.Delete(); err != nil {
		app.serverError(w, r, err)
		return
	}
}
