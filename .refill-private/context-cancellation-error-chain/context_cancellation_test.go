package contextcancellation_test

import (
	"context"
	"errors"
	"testing"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/repository"
)

func TestCanceledContextRemainsDiscoverableAcrossServiceBoundary(t *testing.T) {
	store, err := repository.Open(context.Background(), "file:context-cancellation-error-chain?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := calibration.NewService(store)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	operations := []struct {
		name string
		run  func() error
	}{
		{name: "ready", run: func() error { return service.Ready(canceled) }},
		{name: "get dossier", run: func() error {
			_, err := service.GetDossier(canceled, "dossier-missing")
			return err
		}},
		{name: "timeline", run: func() error {
			_, err := service.Timeline(canceled, "dossier-missing")
			return err
		}},
		{name: "review preflight", run: func() error {
			_, err := service.ReviewPreflight(canceled, "dossier-missing", "reviewer-1")
			return err
		}},
		{name: "verify permit", run: func() error {
			_, err := service.VerifyPermit(canceled, "permit-missing")
			return err
		}},
		{name: "issue permit", run: func() error {
			_, _, err := service.IssuePermit(canceled, calibration.IssuePermitCommand{DossierID: "dossier-missing", ExpectedVersion: 1, Actor: "deployer-1"})
			return err
		}},
	}

	for _, operation := range operations {
		err := operation.run()
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s lost context cancellation in its error chain: %v", operation.name, err)
		}
	}
}
