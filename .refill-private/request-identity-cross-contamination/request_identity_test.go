package requestidentity_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/httpapi"
	"buoy-calibration-gate/internal/repository"
)

type gatedResponseWriter struct {
	*httptest.ResponseRecorder
	blocked bool
	entered chan struct{}
	release chan struct{}
}

func (w *gatedResponseWriter) Header() http.Header {
	if !w.blocked {
		w.blocked = true
		close(w.entered)
		<-w.release
	}
	return w.ResponseRecorder.Header()
}

func createRequest(actor, key string, body []byte) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/calibration-dossiers", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Actor", actor)
	r.Header.Set("X-Role", "engineer")
	r.Header.Set("Idempotency-Key", key)
	return r
}

func TestCreateRequestIdentityIsRequestScoped(t *testing.T) {
	store, err := repository.Open(context.Background(), "file:request-identity-cross-contamination?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := httpapi.New(calibration.NewService(store)).Handler()

	bodyA := []byte(`{"buoyCode":"BUOY-A","targetArea":"东海","plannedDeploymentAt":"2027-01-02T03:04:05Z","owner":"工程师 A"}`)
	responseA := &gatedResponseWriter{ResponseRecorder: httptest.NewRecorder(), entered: make(chan struct{}), release: make(chan struct{})}
	doneA := make(chan struct{})
	go func() {
		defer close(doneA)
		handler.ServeHTTP(responseA, createRequest("actor-a", "create-a", bodyA))
	}()

	<-responseA.entered
	released := false
	defer func() {
		if !released {
			close(responseA.release)
		}
	}()
	bodyB := []byte(`{"buoyCode":"BUOY-B","targetArea":"南海","plannedDeploymentAt":"2027-01-02T03:04:05Z","owner":"工程师 B"}`)
	responseB := httptest.NewRecorder()
	handler.ServeHTTP(responseB, createRequest("actor-b", "create-b", bodyB))
	if responseB.Code != http.StatusCreated {
		t.Fatalf("request B status = %d, body = %s", responseB.Code, responseB.Body.String())
	}

	close(responseA.release)
	released = true
	<-doneA
	if responseA.Code != http.StatusCreated {
		t.Fatalf("request A status = %d, body = %s", responseA.Code, responseA.Body.String())
	}
	var created struct {
		Dossier domain.CalibrationDossier `json:"dossier"`
	}
	if err := json.Unmarshal(responseA.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	timelineResponse := httptest.NewRecorder()
	timelineRequest := httptest.NewRequest(http.MethodGet, "/api/v1/calibration-dossiers/"+created.Dossier.ID+"/timeline", nil)
	handler.ServeHTTP(timelineResponse, timelineRequest)
	if timelineResponse.Code != http.StatusOK {
		t.Fatalf("timeline status = %d, body = %s", timelineResponse.Code, timelineResponse.Body.String())
	}
	var timeline audit.Timeline
	if err := json.Unmarshal(timelineResponse.Body.Bytes(), &timeline); err != nil {
		t.Fatal(err)
	}
	if len(timeline.Events) != 1 {
		t.Fatalf("timeline events = %d, want 1", len(timeline.Events))
	}
	if timeline.Events[0].Actor != "actor-a" {
		t.Fatalf("request A audit actor = %q, want %q", timeline.Events[0].Actor, "actor-a")
	}
}
