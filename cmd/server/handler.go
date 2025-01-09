package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
	"github.com/spf13/viper"
)

func pageRenderWithConfig(config *viper.Viper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get query parameters
		url := r.URL.Query().Get("url")
		log.Printf("url: %s", url)
		if url == "" {
			log.Printf("Missing url parameter %+v", r.URL.String())
			http.Error(w, "Missing url parameter", http.StatusBadRequest)
			return
		}

		// Create boltdb instance
		db, err := bolt.Open(viper.GetString("cache.path"), 0600, nil)
		if err != nil {
			log.Printf("Failed to open boltdb: %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer db.Close()

		render, err := wrender.NewWrender(url)
		if err != nil {
			log.Printf("Failed to create renderer: %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// exists, err := checkObjectExists(render.Response.Path)
		// caching := wrender.NewCaching(db, render.GetHostPath(), render.Response.Path)
		caching, err := wrender.NewCaching(db, render.Response.Path)
		if err != nil {
			log.Printf("Failed to create caching: %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		exists, err := caching.IsValid()
		if err != nil {
			log.Printf("Caching check failed: %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if exists {
			// TODO: Don't need to render page. Get the cached content and return it.
			log.Println("Cache exists")
			cached, err := caching.Read()
			if err != nil {
				log.Printf("Caching read failed: %s", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
            decompressContent, err := internal.Decompress(cached.Content)
            if err != nil {
                log.Printf("Decompress content failed: %s", err)
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
                return
            }
			message := fmt.Sprintf(
				"Cache path: %s/%s/%s\nCache content: %s\n",
				caching.RootBucket,
				caching.HostBucket,
				caching.CachedKey,
                decompressContent,
			)

			fmt.Fprintln(w, message)
			return
		} else {
			log.Println("Cache not exists")
		}

		// Render the page
		browserContext := renderer.BrowserContext{
			DebugMode: config.GetBool("renderer.debugMode"),
			Container: config.GetBool("renderer.container"),
		}
		rendererContext := renderer.RendererContext{
			Headless:       config.GetBool("renderer.headless"),
			WindowWidth:    config.GetInt("renderer.windowWidth"),
			WindowHeight:   config.GetInt("renderer.windowHeight"),
			Timeout:        config.GetInt("renderer.timeout"),
			ImageLoad:      false,
			SkipFrameCount: 0,
		}
		ctx := context.Background()
		ctx = renderer.WithBrowserContext(ctx, &browserContext)
		ctx = renderer.WithRendererContext(ctx, &rendererContext)

		content, err := renderer.RenderPage(ctx, url)
		if err != nil {
			log.Printf("Render Failed: %s", err)
			http.Error(w, "Render Failed", http.StatusInternalServerError)
		}

		// Save the rendered page to cache
		compressedContent, err := internal.Compress(content)
        if err != nil {
            log.Printf("Compress content failed: %s", err)
            http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            return
        }
		cacheDuration := config.GetInt("cache.durationInMinutes")
		cacheItem := wrender.NewBoltCached(
			compressedContent,
			time.Duration(cacheDuration)*time.Minute,
		)
		caching.Update(cacheItem)

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(
			w,
			"write cache path: %s",
			fmt.Sprintf("%s/%s/%s\n", caching.RootBucket, caching.HostBucket, caching.CachedKey),
		)
		// w.Write(context)
	}
}
