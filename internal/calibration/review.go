package calibration

import (
	"context"
	"strings"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func buildPreflight(dossier domain.CalibrationDossier, sensors []domain.SensorBaseline, runs []domain.CalibrationRun, deviations []domain.DeviationCase, reviewer string) (domain.ReviewPreflight, domain.FrozenManifest, string, error) {
	result := domain.ReviewPreflight{DossierVersion: dossier.Version, SensorCount: len(sensors), RunCount: len(runs), DeviationCount: len(deviations), Blockers: domain.ReviewBlockers(sensors, runs, deviations)}
	if len(result.Blockers) != 0 {
		return result, domain.FrozenManifest{}, "", nil
	}
	previewDossier := dossier
	previewDossier.Status = domain.StatusFrozen
	previewDossier.Version = dossier.Version + 1
	manifest := domain.StableManifest(previewDossier, append([]domain.SensorBaseline(nil), sensors...), append([]domain.CalibrationRun(nil), runs...), append([]domain.DeviationCase(nil), deviations...), domain.ReviewDecision{Reviewer: reviewer, Conclusion: "approved"})
	digest, _, err := audit.ManifestDigest(manifest)
	if err != nil {
		return result, manifest, "", err
	}
	result.PreviewDigest = digest
	return result, manifest, digest, nil
}

func (s *Service) ReviewPreflight(ctx context.Context, dossierID, actor string) (domain.ReviewPreflight, error) {
	var result domain.ReviewPreflight
	if err := requireActor(actor); err != nil {
		return result, err
	}
	err := s.store.View(ctx, func(tx *repository.Tx) error {
		snapshot, err := tx.Snapshot(dossierID)
		if err != nil {
			return err
		}
		if snapshot.Dossier.Status != domain.StatusReviewPending {
			return domain.ErrInvalidState
		}
		if cached, ok := s.cachedPreflight(dossierID, snapshot.Dossier.Version); ok {
			result = cached
			return nil
		}
		result, _, _, err = buildPreflight(snapshot.Dossier, snapshot.Sensors, snapshot.Runs, snapshot.Deviations, actor)
		if err == nil {
			s.cachePreflight(dossierID, snapshot.Dossier.Version, result)
		}
		return err
	})
	return result, err
}

func (s *Service) ReturnDeviation(ctx context.Context, cmd ReviewCommand) (domain.DeviationCase, domain.CalibrationDossier, error) {
	var deviation domain.DeviationCase
	var dossier domain.CalibrationDossier
	if err := requireActor(cmd.Actor); err != nil {
		return deviation, dossier, err
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
		if dossier.Status != domain.StatusReviewPending {
			return domain.ErrInvalidState
		}
		deviation, err = tx.GetDeviation(cmd.DeviationID)
		if err != nil {
			return err
		}
		if deviation.DossierID != dossier.ID {
			return &domain.RuleError{Field: "deviationId", Message: "不属于当前档案"}
		}
		if err := deviation.Return(strings.TrimSpace(cmd.Note)); err != nil {
			return err
		}
		if err := tx.UpdateDeviation(deviation); err != nil {
			return err
		}
		now := s.now().UTC()
		if err := dossier.Transition(domain.StatusRemediationRequired, now); err != nil {
			return err
		}
		if err := tx.PutReview(dossier.ID, domain.ReviewDecision{Reviewer: cmd.Actor, Conclusion: "returned", Note: cmd.Note, ReviewedAt: now}); err != nil {
			return err
		}
		if err := tx.UpdateDossier(&dossier, cmd.ExpectedVersion); err != nil {
			return err
		}
		_, err = audit.Append(tx, dossier.ID, "review.deviation_returned", cmd.Actor, map[string]any{"deviationId": deviation.ID, "note": cmd.Note, "version": dossier.Version}, now)
		return err
	})
	return deviation, dossier, err
}

func (s *Service) Approve(ctx context.Context, cmd ReviewCommand) (domain.FrozenManifest, string, domain.CalibrationDossier, error) {
	var manifest domain.FrozenManifest
	var digest string
	var dossier domain.CalibrationDossier
	if err := requireActor(cmd.Actor); err != nil {
		return manifest, digest, dossier, err
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
		if dossier.Status != domain.StatusReviewPending {
			return domain.ErrInvalidState
		}
		sensors, err := tx.ListSensors(dossier.ID)
		if err != nil {
			return err
		}
		runs, err := tx.ListRuns(dossier.ID)
		if err != nil {
			return err
		}
		deviations, err := tx.ListDeviations(dossier.ID)
		if err != nil {
			return err
		}
		preflight, _, previewDigest, err := buildPreflight(dossier, sensors, runs, deviations, cmd.Actor)
		if err != nil {
			return err
		}
		if len(preflight.Blockers) > 0 {
			return &domain.ValidationError{Field: "preflight", Message: "冻结前检查存在阻塞项", ConflictFields: blockerFields(preflight.Blockers)}
		}
		if strings.TrimSpace(cmd.PreviewDigest) != "" && strings.TrimSpace(cmd.PreviewDigest) != previewDigest {
			return &domain.ResourceConflictError{Kind: domain.ErrPreviewConflict, Field: "previewDigest", Message: "预览摘要与当前档案不匹配", ResourceID: dossier.ID}
		}
		now := s.now().UTC()
		manifestUpdatedAt := dossier.UpdatedAt
		review := domain.ReviewDecision{Reviewer: cmd.Actor, Conclusion: "approved", Note: strings.TrimSpace(cmd.Note), ReviewedAt: now}
		if err := dossier.Transition(domain.StatusFrozen, now); err != nil {
			return err
		}
		dossier.Version = cmd.ExpectedVersion + 1
		manifestDossier := dossier
		manifestDossier.UpdatedAt = manifestUpdatedAt
		manifest = domain.StableManifest(manifestDossier, sensors, runs, deviations, review)
		var data []byte
		digest, data, err = audit.ManifestDigest(manifest)
		if err != nil {
			return err
		}
		if err := tx.PutReview(dossier.ID, review); err != nil {
			return err
		}
		if err := tx.InsertManifest(repository.ManifestRecord{DossierID: dossier.ID, FrozenVersion: dossier.Version, Digest: digest, JSON: data, CreatedAt: now}); err != nil {
			return err
		}
		if err := tx.UpdateDossier(&dossier, cmd.ExpectedVersion); err != nil {
			return err
		}
		_, err = audit.Append(tx, dossier.ID, "review.approved_and_frozen", cmd.Actor, map[string]any{"manifestDigest": digest, "frozenVersion": dossier.Version, "note": review.Note}, now)
		return err
	})
	return manifest, digest, dossier, err
}

func blockerFields(blockers []domain.PreflightBlocker) []string {
	fields := make([]string, len(blockers))
	for index, blocker := range blockers {
		fields[index] = blocker.Field
	}
	return fields
}
