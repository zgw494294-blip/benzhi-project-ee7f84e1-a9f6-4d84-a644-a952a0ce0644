package calibration

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func (s *Service) IssuePermit(ctx context.Context, cmd IssuePermitCommand) (domain.DeploymentPermit, domain.CalibrationDossier, error) {
	var permit domain.DeploymentPermit
	var dossier domain.CalibrationDossier
	if err := requireActor(cmd.Actor); err != nil {
		return permit, dossier, err
	}
	err := s.store.Transaction(ctx, func(tx *repository.Tx) error {
		var err error
		dossier, err = tx.GetDossier(cmd.DossierID)
		if err != nil {
			return err
		}
		if err := requireExpected(dossier.Version, cmd.ExpectedVersion); err != nil {
			return err
		}
		if dossier.Status == domain.StatusPermitted {
			return domain.ErrAlreadyIssued
		}
		if dossier.Status != domain.StatusFrozen {
			return domain.ErrInvalidState
		}
		manifest, err := tx.GetManifest(dossier.ID)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		permit = domain.DeploymentPermit{ID: s.id(), PermitNumber: fmt.Sprintf("BCG-%s-%s", now.Format("20060102"), strings.ToUpper(s.id()[:10])), DossierID: dossier.ID, FrozenVersion: manifest.FrozenVersion, ManifestDigest: manifest.Digest, ApprovedBy: cmd.Actor, IssuedAt: now}
		permit.VerificationHash = audit.PermitVerificationHash(permit)
		if err := tx.InsertPermit(permit); err != nil {
			return err
		}
		if err := dossier.Transition(domain.StatusPermitted, now); err != nil {
			return err
		}
		if err := tx.UpdateDossier(&dossier, cmd.ExpectedVersion); err != nil {
			return err
		}
		_, err = audit.Append(tx, dossier.ID, "permit.issued", cmd.Actor, permit, now)
		return err
	})
	if err != nil {
		err = fmt.Errorf("issue permit: %w", err)
	}
	return permit, dossier, err
}

type PermitVerification struct {
	Permit               domain.DeploymentPermit      `json:"permit"`
	Valid                bool                         `json:"valid"`
	Checks               map[string]VerificationCheck `json:"checks"`
	ReasonCodes          []string                     `json:"reasonCodes"`
	VerificationMaterial VerificationMaterial         `json:"verificationMaterial"`
}

type VerificationCheck struct {
	Valid      bool   `json:"valid"`
	ReasonCode string `json:"reasonCode,omitempty"`
}

type VerificationMaterial struct {
	Algorithm        string               `json:"algorithm"`
	Canonicalization string               `json:"canonicalization"`
	FieldOrder       []string             `json:"fieldOrder"`
	Fields           audit.PermitMaterial `json:"fields"`
	VerificationHash string               `json:"verificationHash"`
}

func (s *Service) VerifyPermit(ctx context.Context, number string) (PermitVerification, error) {
	var result PermitVerification
	err := s.store.View(ctx, func(tx *repository.Tx) error {
		var err error
		result.Permit, err = tx.GetPermitByNumber(number)
		if err != nil {
			return err
		}
		result.Checks = make(map[string]VerificationCheck)
		result.VerificationMaterial = VerificationMaterial{Algorithm: "sha256", Canonicalization: "utf8-newline-delimited-v1; issuedAt=RFC3339Nano UTC", FieldOrder: []string{"permitNumber", "dossierId", "frozenVersion", "manifestDigest", "approvedBy", "issuedAt"}, Fields: audit.PermitVerificationMaterial(result.Permit), VerificationHash: result.Permit.VerificationHash}
		setCheck := func(name string, valid bool, reason string) {
			check := VerificationCheck{Valid: valid}
			if !valid {
				check.ReasonCode = reason
				result.ReasonCodes = append(result.ReasonCodes, reason)
			}
			result.Checks[name] = check
		}
		setCheck("permitHash", audit.VerifyPermit(result.Permit), "permit_hash_mismatch")

		dossier, dossierErr := tx.GetDossier(result.Permit.DossierID)
		setCheck("dossierStatus", dossierErr == nil && dossier.Status == domain.StatusPermitted, "dossier_not_permitted")
		linked, linkedErr := tx.GetPermitByDossier(result.Permit.DossierID)
		setCheck("permitAssociation", linkedErr == nil && linked.ID == result.Permit.ID && linked.PermitNumber == result.Permit.PermitNumber, "permit_association_mismatch")

		manifest, manifestErr := tx.GetManifest(result.Permit.DossierID)
		setCheck("frozenVersion", manifestErr == nil && manifest.FrozenVersion == result.Permit.FrozenVersion && dossierErr == nil && dossier.Version == result.Permit.FrozenVersion+1, "frozen_version_mismatch")
		manifestValid := false
		var frozen domain.FrozenManifest
		frozenParsed := false
		if manifestErr == nil {
			if json.Unmarshal(manifest.JSON, &frozen) == nil {
				frozenParsed = true
				recomputed, _, digestErr := audit.ManifestDigest(frozen)
				manifestValid = digestErr == nil && recomputed == manifest.Digest && manifest.Digest == result.Permit.ManifestDigest && frozen.Dossier.ID == result.Permit.DossierID && frozen.Dossier.Version == manifest.FrozenVersion && frozen.Dossier.Status == domain.StatusFrozen
			}
		}
		setCheck("manifestDigest", manifestValid, "manifest_digest_mismatch")

		events, eventsErr := tx.ListAuditEvents(result.Permit.DossierID)
		chainValid := eventsErr == nil && len(events) > 0 && audit.VerifyEventChain(events) == nil
		setCheck("auditChain", chainValid, "audit_chain_invalid")
		eventValid := false
		manifestReviewValid := false
		if eventsErr == nil {
			for _, event := range events {
				if event.EventType == "review.approved_and_frozen" && frozenParsed {
					var payload struct {
						ManifestDigest string `json:"manifestDigest"`
						FrozenVersion  int64  `json:"frozenVersion"`
						Note           string `json:"note"`
					}
					if json.Unmarshal(event.Payload, &payload) == nil && payload.ManifestDigest == result.Permit.ManifestDigest && payload.FrozenVersion == result.Permit.FrozenVersion && payload.Note == frozen.Review.Note && event.Actor == frozen.Review.Reviewer && event.OccurredAt.Equal(frozen.Review.ReviewedAt) {
						manifestReviewValid = true
					}
				}
				if event.EventType != "permit.issued" {
					continue
				}
				var eventPermit domain.DeploymentPermit
				if json.Unmarshal(event.Payload, &eventPermit) == nil && samePermit(eventPermit, result.Permit) {
					eventValid = true
				}
			}
		}
		setCheck("manifestReview", manifestReviewValid, "manifest_review_mismatch")
		setCheck("permitIssuedEvent", eventValid, "permit_event_mismatch")
		sort.Strings(result.ReasonCodes)
		result.Valid = len(result.ReasonCodes) == 0
		return nil
	})
	if err != nil {
		err = fmt.Errorf("verify permit: %w", err)
	}
	return result, err
}

func samePermit(a, b domain.DeploymentPermit) bool {
	return a.ID == b.ID && a.PermitNumber == b.PermitNumber && a.DossierID == b.DossierID && a.FrozenVersion == b.FrozenVersion && a.ManifestDigest == b.ManifestDigest && a.VerificationHash == b.VerificationHash && a.ApprovedBy == b.ApprovedBy && a.IssuedAt.Equal(b.IssuedAt)
}
