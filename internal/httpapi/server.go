package httpapi

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
)

type API struct {
	service    *calibration.Service
	mux        *http.ServeMux
	identityMu sync.RWMutex
	identity   requestIdentity
}

type requestIdentity struct {
	actor string
	role  domain.Role
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
		a.identityMu.Lock()
		a.identity = requestIdentity{
			actor: strings.TrimSpace(r.Header.Get("X-Actor")),
			role:  domain.Role(strings.TrimSpace(r.Header.Get("X-Role"))),
		}
		a.identityMu.Unlock()
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
