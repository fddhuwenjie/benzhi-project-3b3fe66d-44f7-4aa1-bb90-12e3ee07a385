package application

import (
	"encoding/json"
	"oralarchive/internal/domain"
)

type Metadata struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type Result struct {
	Status   int
	Body     []byte
	Dossier  *domain.InterviewDossier
	Replayed bool
}

func result(status int, d *domain.InterviewDossier) (Result, error) {
	body, err := json.Marshal(d)
	return Result{Status: status, Body: body, Dossier: d}, err
}

type TranscriptInput struct {
	Metadata
	Segments []domain.TranscriptSegment `json:"segments"`
}

type AuthorizationRevisionInput struct {
	Metadata
	domain.AuthorizationInput
	ActorID string `json:"actor_id"`
}

type TranscriptOperationsInput struct {
	Metadata
	Operations []domain.TranscriptOperation `json:"operations"`
	ActorID    string                       `json:"actor_id"`
}

type TranscriptPrecheckInput struct {
	Operations []domain.TranscriptOperation `json:"operations,omitempty"`
}

type ResolveInput struct {
	Metadata
	IssueID         string `json:"issue_id"`
	StartOffset     int    `json:"start_offset"`
	EndOffset       int    `json:"end_offset"`
	Reason          string `json:"reason"`
	ReplacementText string `json:"replacement_text"`
	ActorID         string `json:"actor_id"`
}

type ResolveBatchInput struct {
	Metadata
	Items []domain.RedactionResolution `json:"items"`
}

type ConfirmationInput struct {
	Metadata
	Decision          string                         `json:"decision"`
	ConfirmedBy       string                         `json:"confirmed_by"`
	EvidenceDigest    string                         `json:"evidence_digest"`
	AllowedExceptions []domain.ConfirmationException `json:"allowed_exceptions"`
	CandidateSHA256   string                         `json:"candidate_sha256"`
	Note              string                         `json:"note"`
	RejectionIssueIDs []string                       `json:"rejection_issue_ids,omitempty"`
}

type ReviewInput struct {
	Metadata
	Decision   string                       `json:"decision"`
	ReviewerID string                       `json:"reviewer_id"`
	Reason     string                       `json:"reason"`
	IssueIDs   []string                     `json:"issue_ids"`
	Checklist  []domain.ReviewChecklistItem `json:"checklist"`
}

type ActionInput struct {
	Metadata
	ActorID string `json:"actor_id"`
}

type Detail struct {
	Dossier         *domain.InterviewDossier     `json:"dossier"`
	StatusLabel     string                       `json:"status_label"`
	CandidateSHA256 string                       `json:"candidate_sha256,omitempty"`
	PackageValid    *bool                        `json:"package_valid,omitempty"`
	ReviewChecklist []domain.ReviewChecklistItem `json:"review_checklist,omitempty"`
}

type QueueFilter struct {
	Statuses    []domain.Status
	EditorID    string
	SubjectCode string
	Keyword     string
	UpdatedFrom string
	UpdatedTo   string
	Cursor      string
	PageSize    int
}

type QueueItem struct {
	DossierID            string        `json:"dossier_id"`
	SubjectCode          string        `json:"subject_code"`
	Status               domain.Status `json:"status"`
	StatusLabel          string        `json:"status_label"`
	EditorID             string        `json:"editor_id"`
	Revision             int64         `json:"revision"`
	UpdatedAt            string        `json:"updated_at"`
	OpenBlockers         int           `json:"open_blockers"`
	PendingConfirmations int           `json:"pending_confirmations"`
	PendingReviews       int           `json:"pending_reviews"`
}
type QueueStats struct {
	ByStatus             map[domain.Status]int `json:"by_status"`
	OpenBlockers         int                   `json:"open_blockers"`
	PendingConfirmations int                   `json:"pending_confirmations"`
	PendingReviews       int                   `json:"pending_reviews"`
}
type QueueResult struct {
	Dossiers   []QueueItem `json:"dossiers"`
	Stats      QueueStats  `json:"stats"`
	NextCursor string      `json:"next_cursor,omitempty"`
}
