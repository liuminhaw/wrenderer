package upAndRun

import (
	"net/http"
	"slices"

	"github.com/spf13/viper"
)

func authorized(config *viper.Viper) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keys := []string{
				config.GetString("app.key"),
				config.GetString("app.adminKey"),
			}

			requestKey := r.Header.Get("x-api-key")

			if !slices.Contains(keys, requestKey) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func authorizedAdmin(config *viper.Viper) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestKey := r.Header.Get("x-api-key")

			if requestKey != config.GetString("app.adminKey") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
