package upAndRun

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/liuminhaw/renderer"
	"github.com/spf13/viper"
)

// The serverError helper writes a log entry at Error level (including the request
// method and URI as attributes), then sends a generic 500 Internal Server Error
// response to the user.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
		trace  = string(debug.Stack())
	)

	app.logger.Error(err.Error(), slog.String("method", method), slog.String("uri", uri))
	app.logger.Debug(err.Error(), slog.String("trace", trace))
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// The clientError helper sends a specific status code and corresponding description
// to the user.
func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func rendererContext(config *viper.Viper) context.Context {
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

    return ctx
}
