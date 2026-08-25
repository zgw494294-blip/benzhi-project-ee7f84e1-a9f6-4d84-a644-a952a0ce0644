package domain

type DossierStatus string

const (
	StatusDraft               DossierStatus = "draft"
	StatusCalibrating         DossierStatus = "calibrating"
	StatusRemediationRequired DossierStatus = "remediation_required"
	StatusReviewPending       DossierStatus = "review_pending"
	StatusFrozen              DossierStatus = "frozen"
	StatusPermitted           DossierStatus = "permitted"
)

func (s DossierStatus) Valid() bool {
	switch s {
	case StatusDraft, StatusCalibrating, StatusRemediationRequired, StatusReviewPending, StatusFrozen, StatusPermitted:
		return true
	default:
		return false
	}
}

type DeviationStatus string

const (
	DeviationOpen     DeviationStatus = "open"
	DeviationClosed   DeviationStatus = "closed"
	DeviationReturned DeviationStatus = "returned"
)
