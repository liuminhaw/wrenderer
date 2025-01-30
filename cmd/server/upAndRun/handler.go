package upAndRun

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
	"github.com/spf13/viper"
)

func (app *application) pageRenderWithConfig(config *viper.Viper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get query parameters
		url := r.URL.Query().Get("url")
		app.logger.Debug(fmt.Sprintf("url: %s", url), slog.String("request", r.URL.String()))
		if url == "" {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		// Create boltdb instance
		db, err := bolt.Open(viper.GetString("cache.path"), 0600, nil)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		defer db.Close()

		render, err := wrender.NewWrender(url)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		// exists, err := checkObjectExists(render.Response.Path)
		// caching := wrender.NewCaching(db, render.GetHostPath(), render.Response.Path)
		caching, err := wrender.NewBoltCaching(db, render.CachePath)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		exists, err := caching.IsValid()
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if exists {
			// TODO: Don't need to render page. Get the cached content and return it.
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
			// message := fmt.Sprintf(
			// 	"Cache path: %s/%s/%s\nCache content: %s\n",
			// 	caching.RootBucket,
			// 	caching.HostBucket,
			// 	caching.CachedKey,
			// 	decompressContent,
			// )

			// fmt.Fprintln(w, message)
			// fmt.Fprint(w, string(decompressContent))
			w.Write(decompressContent)
			return
		} else {
			app.logger.Debug(
				"Cache not exists",
				slog.String("RootBucket", caching.RootBucket),
				slog.String("HostBucket", caching.HostBucket),
				slog.String("CachedKey", caching.CachedKey),
			)

			// Render the page
			render := renderer.NewRenderer(renderer.WithLogger(app.logger))
			content, err := render.RenderPage(url, rendererOption(config))
			if err != nil {
				app.serverError(w, r, err)
				return
			}

			// Save the rendered page to cache
			compressedContent, err := internal.Compress(content)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			cacheDuration := config.GetInt("cache.durationInMinutes")
			cacheItem := wrender.NewBoltCached(
				compressedContent,
				time.Duration(cacheDuration)*time.Minute,
			)

			if err := caching.Update(cacheItem); err != nil {
				app.serverError(w, r, err)
				return
			}

			w.Header().Set("Content-Type", "text/html")
			w.Write(content)
		}
	}
}
