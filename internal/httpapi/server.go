package httpapi

import (
	"net/http"
	"strings"
	"time"

	"buoy-calibration-gate/internal/calibration"
)

type API struct {
	service *calibration.Service
	mux     *http.ServeMux
}

func New(service *calibration.Service) *API {
	a := &API{service: service, mux: http.NewServeMux()}
	a.routes()
	return a
}

func (a *API) Handler() http.Handler {
	return a.requestMiddleware(a.mux)
}

func (a *API) requestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
			if contentType != "application/json" && !strings.HasPrefix(contentType, "application/json;") {
				writeBadRequest(w, "Content-Type 必须为 application/json")
				return
			}
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = start
	})
}
