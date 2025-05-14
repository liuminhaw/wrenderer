package upAndRun

import (
	"net/http"

	"github.com/spf13/viper"
)

// The routes() method returns a servemux containing our application routes.
func (app *application) routes(vConfig *viper.Viper) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /render", app.pageRenderWithConfig(vConfig))
	mux.HandleFunc("DELETE /render", app.deleteRenderedCache)
	mux.HandleFunc("PUT /render/sitemap", app.renderSitemapWithConfig(vConfig))
	mux.HandleFunc("GET /render/sitemap/{jobId}/status", app.renderSitemapStatus)

	// admin routes
	adminCheck := authorizedAdmin(vConfig)
	mux.Handle("GET /admin/renders", adminCheck(http.HandlerFunc(app.listRenderedCaches)))
	mux.Handle("GET /admin/jobs", adminCheck(http.HandlerFunc(app.listJobCaches)))
	mux.Handle("GET /admin/config", adminCheck(http.HandlerFunc(app.listConfigWithConfig(vConfig))))

	return authorized(vConfig)(mux)
}
