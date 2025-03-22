package upAndRunWorker

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/wrender"
	"github.com/spf13/viper"
)

// func (app *application) startWorkers(workersCount int) {
func (h *Handler) StartWorkers(workersCount int) {
	// Render page worker
	for i := range workersCount {
		go h.renderPage(viper.GetViper(), i)
	}

	// Error listening worker
	go h.ErrorListener()
}

func (h *Handler) renderPage(config *viper.Viper, id int) {
	h.Logger.Debug("Worker started", slog.Int("id", id))
	for job := range h.RenderQueue {
		h.Logger.Debug("Worker start rendering", slog.String("url", job.Url), slog.Int("id", id))
		render := renderer.NewRenderer(renderer.WithLogger(h.Logger))
		content, err := render.RenderPage(job.Url, rendererOption(config))
		if err != nil {
			job.Result <- RenderJobResult{Content: nil, Err: err}
		} else {
			job.Result <- RenderJobResult{Content: content, Err: nil}
		}
	}
}

func (h *Handler) StartCacheCleaner(interval int) {
	h.Logger.Debug("Cache cleaner started", slog.Int("interval", interval))
	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		h.Logger.Debug("Cache cleaner triggered")
		if err := h.cleanExpiredCache(); err != nil {
			h.Logger.Error(fmt.Sprintf("Error cleaning cache: %s", err))
		} else {
			h.Logger.Debug("Cache cleaner done")
		}
	}
}

func (h *Handler) cleanExpiredCache() error {
	// Clean expired render caches
	caching := wrender.BoltCaching{DB: h.DB, RootBucket: wrender.CachedPagePrefix}
	if err := caching.Cleanup(); err != nil {
		return err
	}

	// Clean expired job caches
	caching = wrender.BoltCaching{DB: h.DB, RootBucket: wrender.CachedJobPrefix}
	if err := caching.Cleanup(); err != nil {
		return err
	}

	return nil
}

func rendererOption(config *viper.Viper) *renderer.RendererOption {
	config.SetDefault("renderer.windowWidth", 1920)
	config.SetDefault("renderer.windowHeight", 1080)
	config.SetDefault("renderer.container", false)
	config.SetDefault("renderer.headless", true)
	config.SetDefault("renderer.userAgent", "")
	config.SetDefault("renderer.timeout", 30)

	return &renderer.RendererOption{
		BrowserOpts: renderer.BrowserConf{
			IdleType:      "networkIdle",
			Container:     config.GetBool("renderer.container"),
			ChromiumDebug: config.GetBool("chromiumDebug"),
		},
		Headless:     config.GetBool("renderer.headless"),
		WindowWidth:  config.GetInt("renderer.windowWidth"),
		WindowHeight: config.GetInt("renderer.windowHeight"),
		Timeout:      config.GetInt("renderer.timeout"),
		UserAgent:    config.GetString("renderer.userAgent"),
	}
}
