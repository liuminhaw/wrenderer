package upAndRun

import (
	"net/http"

	"github.com/spf13/viper"
)

// The routes() method returns a servemux containing our application routes.
func (app *application) routes(vConfig *viper.Viper) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /render", app.pageRenderWithConfig(vConfig))
	mux.HandleFunc("DELETE /render", app.deleteRenderedCache)
	mux.HandleFunc("PUT /render/sitemap", app.renderSitemapWithConfig(vConfig))
	mux.HandleFunc("GET /render/sitemap/{jobId}/status", app.renderSitemapStatus)
	// admin routes
	mux.HandleFunc("GET /admin/renders", app.listRenderedCaches)
	mux.HandleFunc("GET /admin/jobs", app.listJobCaches)
	// mux.HandleFunc("GET /admin/config", app.getConfig)

	return mux
}
