package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDossierTransitionsRejectSkipping(t *testing.T) {
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	d, err := NewDossier("d-1", "B-01", "东海", "张工", now.Add(24*time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Transition(StatusFrozen, now); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expected invalid state, got %v", err)
	}
	for _, next := range []DossierStatus{StatusCalibrating, StatusReviewPending, StatusFrozen, StatusPermitted} {
		if err := d.Transition(next, now); err != nil {
			t.Fatalf("transition to %s: %v", next, err)
		}
	}
	if err := d.Mutable(); !errors.Is(err, ErrFrozen) {
		t.Fatalf("expected frozen error, got %v", err)
	}
}

func TestRunToleranceAndRetestConsistency(t *testing.T) {
	now := time.Now().UTC()
	sensor := SensorBaseline{ID: "s-1", DossierID: "d-1", SensorType: "temperature", SerialNumber: "T-1", Unit: "C", RangeMin: -5, RangeMax: 45, Tolerance: .1, ConfigurationRevision: "v1", CreatedAt: now}
	failed, err := NewRun("r-1", sensor, RunInitial, 20, 20.11, 22, "sha256:"+strings.Repeat("a", 64), "engineer", now)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Passed {
		t.Fatal("run should fail tolerance")
	}
	deviation, err := NewDeviation("x-1", failed, now)
	if err != nil {
		t.Fatal(err)
	}
	passed, err := NewRun("r-2", sensor, RunRetest, 20, 20.05, 22, "sha256:"+strings.Repeat("b", 64), "engineer", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := deviation.Close("零点漂移", "offset=-0.06", passed, now); err != nil {
		t.Fatal(err)
	}
	if deviation.Status != DeviationClosed || deviation.RetestRunID != passed.ID {
		t.Fatalf("unexpected deviation: %+v", deviation)
	}
}
