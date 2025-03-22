package upAndRun

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/liuminhaw/wrenderer/cmd/shared"
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

		if !internal.ValidUrl(url) {
			app.logger.Info(
				"Invalid url",
				slog.String("url", url),
				slog.String("request", r.URL.String()),
			)
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
		var exists, expired bool
		var cached wrender.PageCached
		cachedData, err := caching.Read()
		if err != nil { // cache not exists
			var werr *wrender.CacheNotFoundError
			if !errors.As(err, &werr) {
				app.serverError(w, r, err)
				return
			}
		} else { // cache exists
			exists = true
			if err := json.Unmarshal([]byte(cachedData), &cached); err != nil {
				app.serverError(w, r, err)
				return
			}
			expired = cached.IsExpired()
		}
		app.logger.Debug("Checking cache done", slog.String("url", url))

		if exists && !expired {
			app.logger.Debug(
				"Cache exists and not expired",
				slog.String("RootBucket", caching.RootBucket),
				slog.String("HostBucket", caching.HostBucket),
				slog.String("CachedKey", caching.CachedKey),
			)

			decompressContent, err := internal.Decompress(cached.Content)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			w.Write(decompressContent)
			return
		} else {
			app.logger.Debug(
				"Cache expired or not exists",
				slog.String("RootBucket", caching.RootBucket),
				slog.String("HostBucket", caching.HostBucket),
				slog.String("CachedKey", caching.CachedKey),
			)

			// Add render job to queue
			var result upAndRunWorker.RenderJobResult
			job := upAndRunWorker.RenderJob{Url: url, Result: make(chan upAndRunWorker.RenderJobResult, 1)}
			select {
			case app.renderQueue <- job:
				// Wait for the job to be processed
				app.logger.Info("Job added to queue", slog.String("url", url))
				result = <-job.Result
				if result.Err != nil {
					app.serverError(w, r, result.Err)
					return
				}
			default:
				app.clientError(w, http.StatusTooManyRequests)
				return
			}

			// Save the rendered page to cache
			pageCache := wrender.NewPageCached(url, nil, config.GetDuration("cache.durationInMinutes")*time.Minute)
			if err := pageCache.Update(caching, result.Content, false); err != nil {
				app.serverError(w, r, err)
				return
			}

			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write(result.Content)
		}
	}
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

func (app *application) renderSitemapWithConfig(config *viper.Viper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			go workerHandler.RenderSitemap(
				config,
				payload.SitemapUrl,
				renderKey,
			)
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
}

func (app *application) renderSitemapStatus(w http.ResponseWriter, r *http.Request) {
	jobId := r.PathValue("jobId")
	if jobId == "" {
		app.clientError(w, http.StatusBadRequest)
		return
	}
	app.logger.Debug("Check job status", slog.String("jobId", jobId))

	param := fmt.Sprintf("%s/%s", internal.SitemapCategory, jobId)
	jobCaching, err := wrender.NewBoltCaching(app.db, param, wrender.CachedJobPrefix, false)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	var statusResp shared.RenderStatusResp
	content, err := jobCaching.Read()
	if err != nil {
		var werr *wrender.CacheNotFoundError
		if errors.As(err, &werr) {
			app.logger.Info(
				"Status of sitemap job not found",
				slog.String("job id", jobId),
				slog.String("error", err.Error()),
			)
			statusResp = shared.RenderStatusResp{
				Status:     internal.JobStatusUnknown,
				StatusCode: http.StatusNotFound,
			}
		} else {
			app.serverError(w, r, err)
			return
		}
	} else {
		jobCache := wrender.SitemapJobCache{}
		if err := json.Unmarshal([]byte(content), &jobCache); err != nil {
			app.serverError(w, r, err)
			return
		}

		if jobCache.Status != internal.JobStatusCompleted && jobCache.IsExpired() {
			jobCache.Status = internal.JobStatusTimeout
		}
		statusResp = shared.RenderStatusResp{
			Status:     jobCache.Status,
			Details:    jobCache.Failed,
			StatusCode: http.StatusOK,
		}
	}
	responseBody, err := json.Marshal(statusResp)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusResp.StatusCode)
	w.Write(responseBody)
}

type renderedCachesResponse struct {
	Caches any `json:"caches"`
}

func (app *application) listRenderedCaches(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	domain := r.URL.Query().Get("domain")
	app.logger.Info(
		fmt.Sprintf("domain: %s", domain),
		slog.String("request", r.URL.String()),
		slog.String("method", r.Method),
	)

	response, err := listCaches(
		app.db,
		wrender.CachedPagePrefix,
		domain,
		wrender.PagesCachesConversion,
	)
	if err != nil {
		var werr *wrender.CacheNotFoundError
		if errors.As(err, &werr) {
			app.logger.Info(
				"Listing rendered caches not found",
				slog.String("request", r.URL.String()),
				slog.String("error", err.Error()),
			)
			app.clientError(w, http.StatusNotFound)
			return
		}
		app.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func (app *application) listJobCaches(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	category := r.URL.Query().Get("category")
	app.logger.Info(
		"List rendered jobs",
		slog.String("request", r.URL.String()),
		slog.String("method", r.Method),
		slog.String("query category", category),
	)

	response, err := listCaches(
		app.db,
		wrender.CachedJobPrefix,
		category,
		wrender.JobsCachesConversion,
	)
	if err != nil {
		var werr *wrender.CacheNotFoundError
		if errors.As(err, &werr) {
			app.logger.Info(
				"Listing job caches not found",
				slog.String("request", r.URL.String()),
				slog.String("error", err.Error()),
			)
			app.clientError(w, http.StatusNotFound)
			return
		}
		app.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}
