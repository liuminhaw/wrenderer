package upAndRun

import (
	"log/slog"

	"github.com/liuminhaw/renderer"
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
