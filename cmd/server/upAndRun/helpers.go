package upAndRun

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/wrenderer/cmd/worker/upAndRunWorker"
)

type application struct {
	logger           *slog.Logger
	port             int
	db               *bolt.DB
	renderQueue      chan upAndRunWorker.RenderJob
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
