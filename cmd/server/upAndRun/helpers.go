package upAndRun

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/boltdb/bolt"
	"github.com/liuminhaw/wrenderer/cmd/shared"
	"github.com/liuminhaw/wrenderer/cmd/worker/upAndRunWorker"
	"github.com/liuminhaw/wrenderer/wrender"
)

type application struct {
	logger           *slog.Logger
	addr             string
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
func (app *application) clientError(
	w http.ResponseWriter,
	status int,
	message *shared.RespErrorMessage,
) {
	if message == nil {
		message = &shared.RespErrorMessage{Message: http.StatusText(status)}
	}
	respMsg, err := json.Marshal(message)
	if err != nil {
		app.logger.Error("failed to marshal response message", slog.Any("message", message))
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	w.Write([]byte(respMsg))
}

func listCaches[T any](
	db *bolt.DB,
	cachePrefix string,
	queryString string,
	conversion func([]wrender.CacheContentInfo) ([]T, error),
) ([]byte, error) {
	var caching wrender.BoltCaching
	if queryString == "" {
		caching = wrender.BoltCaching{
			DB:         db,
			RootBucket: cachePrefix,
		}
	} else {
		var err error
		caching, err = wrender.NewBoltCaching(
			db,
			queryString,
			cachePrefix,
			true,
		)
		if err != nil {
			return nil, err
		}
	}

	cachesInfo, err := caching.List()
	if err != nil {
		return nil, err
	}
	caches, err := conversion(cachesInfo)
	if err != nil {
		return nil, err
	}

	cachesResponse := renderedCachesResponse{Caches: caches}
	response, err := json.Marshal(cachesResponse)
	if err != nil {
		return nil, err
	}

	return response, nil
}
