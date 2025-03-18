package upAndRun

import (
	"net/http"

	"github.com/spf13/viper"
)

// The routes() method returns a servemux containing our application routes.
func (app *application) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /render", app.pageRenderWithConfig(viper.GetViper()))
	mux.HandleFunc("DELETE /render", app.deleteRenderedCache)
	mux.HandleFunc("PUT /render/sitemap", app.renderSitemapWithConfig(viper.GetViper()))
	mux.HandleFunc("GET /render/sitemap/{jobId}/status", app.renderSitemapStatus)
	// admin routes
	mux.HandleFunc("GET /admin/renders", app.listRenderedCaches)
	mux.HandleFunc("GET /admin/jobs", app.listJobCaches)

	return mux
}
