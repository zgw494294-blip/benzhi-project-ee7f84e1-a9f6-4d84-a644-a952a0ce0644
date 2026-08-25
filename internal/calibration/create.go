package calibration

import (
	"context"
	"strings"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func (s *Service) CreateDossier(ctx context.Context, cmd CreateDossierCommand) (domain.CalibrationDossier, bool, error) {
	var result domain.CalibrationDossier
	var replay bool
	if err := requireActor(cmd.Actor); err != nil {
		return result, false, err
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return result, false, &domain.RuleError{Field: "Idempotency-Key", Message: "不能为空"}
	}
	normalizedBuoy := domain.NormalizeBuoyCode(cmd.BuoyCode)
	hash, err := requestHash(struct {
		BuoyCode, TargetArea, Owner string
		Planned                     timeValue
	}{normalizedBuoy, strings.TrimSpace(cmd.TargetArea), strings.TrimSpace(cmd.Owner), timeValue(cmd.PlannedDeploymentAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"))})
	if err != nil {
		return result, false, err
	}
	err = s.store.Transaction(ctx, func(tx *repository.Tx) error {
		existing, found, err := tx.FindIdempotency("create-dossier", cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			if existing.RequestHash != hash {
				return domain.ErrIdempotencyKey
			}
			result, err = tx.GetDossier(existing.ResourceID)
			replay = err == nil
			return err
		}
		now := s.now().UTC()
		result, err = domain.NewDossier(s.id(), normalizedBuoy, cmd.TargetArea, cmd.Owner, cmd.PlannedDeploymentAt, now)
		if err != nil {
			return err
		}
		if occupied, found, err := tx.FindActiveDossierByBuoyCode(result.BuoyCode); err != nil {
			return err
		} else if found {
			return &domain.ResourceConflictError{Kind: domain.ErrActiveDossier, Field: "buoyCode", Message: "该浮标已有未结束档案", ResourceID: occupied.ID}
		}
		if err := tx.InsertDossier(result); err != nil {
			return err
		}
		if _, err := audit.Append(tx, result.ID, "dossier.created", cmd.Actor, result, now); err != nil {
			return err
		}
		return tx.InsertIdempotency(repository.IdempotencyRecord{Scope: "create-dossier", Key: cmd.IdempotencyKey, RequestHash: hash, ResourceID: result.ID, CreatedAt: now})
	})
	return result, replay, err
}

type timeValue string
