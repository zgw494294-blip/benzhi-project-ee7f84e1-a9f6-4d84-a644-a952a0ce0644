package calibration

import (
	"context"
	"strings"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

type creationReplay struct {
	requestHash string
	resourceID  string
}

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
	select {
	case <-ctx.Done():
		return result, false, ctx.Err()
	case <-s.createGate:
	}
	defer func() { s.createGate <- struct{}{} }()
	if existing, found := s.creationReplays[cmd.IdempotencyKey]; found {
		if existing.requestHash != hash {
			return result, false, domain.ErrIdempotencyKey
		}
		err = s.store.View(ctx, func(tx *repository.Tx) error {
			result, err = tx.GetDossier(existing.resourceID)
			return err
		})
		return result, err == nil, err
	}
	err = s.store.Transaction(ctx, func(tx *repository.Tx) error {
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
	if err == nil {
		s.creationReplays[cmd.IdempotencyKey] = creationReplay{requestHash: hash, resourceID: result.ID}
	}
	return result, replay, err
}

type timeValue string
