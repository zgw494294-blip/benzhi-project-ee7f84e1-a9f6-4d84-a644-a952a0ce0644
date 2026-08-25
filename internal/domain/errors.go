package domain

import "errors"

var (
	ErrNotFound         = errors.New("resource not found")
	ErrConflict         = errors.New("version conflict")
	ErrInvalidState     = errors.New("operation is not allowed in current state")
	ErrValidation       = errors.New("validation failed")
	ErrFrozen           = errors.New("frozen dossier cannot be modified")
	ErrAlreadyIssued    = errors.New("deployment permit already issued")
	ErrUnauthorized     = errors.New("role is not authorized")
	ErrIdempotencyKey   = errors.New("idempotency key was reused with different content")
	ErrEvidenceConflict = errors.New("evidence digest is already in use")
	ErrSensorConflict   = errors.New("sensor identity is already in use")
	ErrActiveDossier    = errors.New("buoy already has an active dossier")
	ErrPreviewConflict  = errors.New("preview digest does not match current dossier")
)

type RuleError struct {
	Field   string
	Message string
}

func (e *RuleError) Error() string { return e.Field + ": " + e.Message }
func (e *RuleError) Unwrap() error { return ErrValidation }

// ValidationError carries actionable details for forms which validate a set of
// related fields atomically.
type ValidationError struct {
	Field          string
	Message        string
	MissingTypes   []string
	ConflictFields []string
	ItemIndex      *int
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
func (e *ValidationError) Unwrap() error { return ErrValidation }

type ResourceConflictError struct {
	Kind       error
	Field      string
	Message    string
	ResourceID string
}

func (e *ResourceConflictError) Error() string { return e.Field + ": " + e.Message }
func (e *ResourceConflictError) Unwrap() error { return e.Kind }
