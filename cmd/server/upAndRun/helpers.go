package upAndRun

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/renderer"
	"github.com/spf13/viper"
)

type application struct {
	logger           *slog.Logger
	port             int
	db               *bolt.DB
	renderQueue      chan renderJob
	sitemapSemaphore chan struct{}
	errorChan        chan error
}

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
