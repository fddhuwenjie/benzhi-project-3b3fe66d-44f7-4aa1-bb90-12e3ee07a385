package domain

type Status string

const (
	StatusDraft         Status = "draft"
	StatusConsentLocked Status = "consent_locked"
	StatusFrozen        Status = "transcript_frozen"
	StatusRemediation   Status = "remediation"
	StatusConfirmation  Status = "awaiting_confirmation"
	StatusReview        Status = "awaiting_review"
	StatusSealed        Status = "sealed"
)

func (s Status) Label() string {
	switch s {
	case StatusDraft:
		return "草稿"
	case StatusConsentLocked:
		return "授权已锁定"
	case StatusFrozen:
		return "转写已冻结"
	case StatusRemediation:
		return "定向整改"
	case StatusConfirmation:
		return "待受访者确认"
	case StatusReview:
		return "待独立复核"
	case StatusSealed:
		return "已封存"
	default:
		return string(s)
	}
}

func ValidStatus(value string) bool {
	switch Status(value) {
	case StatusDraft, StatusConsentLocked, StatusFrozen, StatusRemediation, StatusConfirmation, StatusReview, StatusSealed:
		return true
	default:
		return false
	}
}

func AllStatuses() []Status {
	return []Status{StatusDraft, StatusConsentLocked, StatusFrozen, StatusRemediation, StatusConfirmation, StatusReview, StatusSealed}
}
