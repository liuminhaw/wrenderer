package upAndRun

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/wrender"
	"github.com/spf13/viper"
)

type renderJobResult struct {
	content []byte
	err     error
}

type renderJob struct {
	url    string
	result chan renderJobResult
}

func (app *application) startWorkers(workersCount int) {
	for i := 0; i < workersCount; i++ {
		go app.renderPage(viper.GetViper(), i)
	}
}

func (app *application) renderPage(config *viper.Viper, id int) {
	app.logger.Debug("Worker started", slog.Int("id", id))
	for job := range app.renderQueue {
		app.logger.Debug("Worker start rendering", slog.String("url", job.url), slog.Int("id", id))
		render := renderer.NewRenderer(renderer.WithLogger(app.logger))
		content, err := render.RenderPage(job.url, rendererOption(config))
		if err != nil {
			job.result <- renderJobResult{content: nil, err: err}
		} else {
			job.result <- renderJobResult{content: content, err: nil}
		}
	}
}

func (app *application) startCacheCleaner(interval int) {
	app.logger.Debug("Cache cleaner started", slog.Int("interval", interval))
	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		app.logger.Debug("Cache cleaner triggered")
		app.cleanExpiredCache()
		app.logger.Debug("Cache cleaner done")
	}
}

func (app *application) cleanExpiredCache() {
	caching := wrender.BoltCaching{DB: app.db, RootBucket: wrender.CachedPagePrefix}

	if err := caching.Cleanup(); err != nil {
		app.logger.Error(fmt.Sprintf("Error cleaning cache: %s", err))
	}
}
