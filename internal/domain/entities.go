package domain

import "time"

type InterviewDossier struct {
	DossierID              string                  `json:"dossier_id"`
	Status                 Status                  `json:"status"`
	SubjectCode            string                  `json:"subject_code"`
	AudioRef               string                  `json:"audio_ref"`
	AudioSHA256            string                  `json:"audio_sha256"`
	InterviewedAt          string                  `json:"interviewed_at"`
	AllowedUses            []string                `json:"allowed_uses"`
	EmbargoUntil           string                  `json:"embargo_until"`
	ConsentEvidenceDigest  string                  `json:"consent_evidence_digest"`
	EditorID               string                  `json:"editor_id"`
	Revision               int64                   `json:"revision"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              time.Time               `json:"updated_at"`
	Segments               []TranscriptSegment     `json:"segments"`
	Issues                 []RedactionIssue        `json:"issues"`
	Confirmations          []SubjectConfirmation   `json:"confirmations"`
	AuthorizationRevisions []AuthorizationRevision `json:"authorization_revisions"`
	Reviews                []ReviewSnapshot        `json:"reviews"`
	RemediationRounds      []RemediationRound      `json:"remediation_rounds"`
	Package                *ReleasePackage         `json:"release_package,omitempty"`
	Audit                  []AuditEvent            `json:"audit"`
}

type AuthorizationRevision struct {
	RevisionID    string            `json:"revision_id"`
	DossierID     string            `json:"dossier_id"`
	ChangedFields []string          `json:"changed_fields"`
	BeforeSHA256  map[string]string `json:"before_sha256"`
	AfterSHA256   map[string]string `json:"after_sha256"`
	ActorID       string            `json:"actor_id"`
	CreatedAt     time.Time         `json:"created_at"`
	Revision      int64             `json:"revision"`
}

type TranscriptSegment struct {
	SegmentID   string `json:"segment_id"`
	DossierID   string `json:"dossier_id"`
	StartMS     int64  `json:"start_ms"`
	EndMS       int64  `json:"end_ms"`
	SpeakerCode string `json:"speaker_code"`
	Text        string `json:"text"`
	TextSHA256  string `json:"text_sha256"`
	Sequence    int    `json:"sequence"`
	Revision    int64  `json:"revision"`
}

type Severity string

const (
	SeverityBlocker Severity = "blocker"
	SeverityWarning Severity = "warning"
)

type IssueStatus string

const (
	IssueOpen     IssueStatus = "open"
	IssueResolved IssueStatus = "resolved"
)

type RedactionIssue struct {
	IssueID         string                    `json:"issue_id"`
	DossierID       string                    `json:"dossier_id"`
	SegmentID       string                    `json:"segment_id"`
	RuleCode        string                    `json:"rule_code"`
	Severity        Severity                  `json:"severity"`
	StartOffset     int                       `json:"start_offset"`
	EndOffset       int                       `json:"end_offset"`
	Reason          string                    `json:"reason"`
	ReplacementText string                    `json:"replacement_text"`
	OriginalSHA256  string                    `json:"original_sha256"`
	Status          IssueStatus               `json:"status"`
	ResolvedBy      string                    `json:"resolved_by"`
	ResolvedAt      *time.Time                `json:"resolved_at,omitempty"`
	CurrentRound    int                       `json:"current_round,omitempty"`
	History         []IssueResolutionSnapshot `json:"history,omitempty"`
}

type IssueResolutionSnapshot struct {
	RoundNumber     int       `json:"round_number,omitempty"`
	Reason          string    `json:"reason"`
	ReplacementText string    `json:"replacement_text"`
	OriginalSHA256  string    `json:"original_sha256"`
	ResolvedBy      string    `json:"resolved_by"`
	ResolvedAt      time.Time `json:"resolved_at"`
}

type ConfirmationException struct {
	SegmentID   string `json:"segment_id"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	AllowedUse  string `json:"allowed_use"`
	Description string `json:"description"`
}

type SubjectConfirmation struct {
	ConfirmationID    string                  `json:"confirmation_id"`
	DossierID         string                  `json:"dossier_id"`
	Decision          string                  `json:"decision"`
	ConfirmedBy       string                  `json:"confirmed_by"`
	ConfirmedAt       time.Time               `json:"confirmed_at"`
	EvidenceDigest    string                  `json:"evidence_digest"`
	AllowedExceptions []ConfirmationException `json:"allowed_exceptions"`
	CandidateSHA256   string                  `json:"candidate_sha256"`
	Note              string                  `json:"note"`
}

type ReviewChecklistItem struct {
	Code           string `json:"code"`
	MaterialSHA256 string `json:"material_sha256"`
	Conclusion     string `json:"conclusion"`
}

type ReviewSnapshot struct {
	ReviewID          string                `json:"review_id"`
	DossierID         string                `json:"dossier_id"`
	Decision          string                `json:"decision"`
	ReviewerID        string                `json:"reviewer_id"`
	Checklist         []ReviewChecklistItem `json:"checklist"`
	Reason            string                `json:"reason"`
	IssueIDs          []string              `json:"issue_ids"`
	CandidateSHA256   string                `json:"candidate_sha256"`
	SnapshotSHA256    string                `json:"snapshot_sha256"`
	SubmittedRevision int64                 `json:"submitted_revision"`
	CreatedAt         time.Time             `json:"created_at"`
}

type RemediationRound struct {
	RoundNumber              int        `json:"round_number"`
	ReviewerID               string     `json:"reviewer_id"`
	Reason                   string     `json:"reason"`
	IssueIDs                 []string   `json:"issue_ids"`
	RejectedCandidateSHA256  string     `json:"rejected_candidate_sha256"`
	StartedAt                time.Time  `json:"started_at"`
	EndedAt                  *time.Time `json:"ended_at,omitempty"`
	ConfirmedCandidateSHA256 string     `json:"confirmed_candidate_sha256,omitempty"`
	CandidateChanged         bool       `json:"candidate_changed"`
	ConfirmationDecision     string     `json:"confirmation_decision,omitempty"`
}

type ReleasePackage struct {
	PackageID            string    `json:"package_id"`
	DossierID            string    `json:"dossier_id"`
	ManifestVersion      string    `json:"manifest_version"`
	Manifest             string    `json:"manifest"`
	RedactedTranscript   string    `json:"redacted_transcript"`
	ConsentSnapshot      string    `json:"consent_snapshot"`
	ReviewerID           string    `json:"reviewer_id"`
	ApprovedAt           time.Time `json:"approved_at"`
	AuditHeadSHA256      string    `json:"audit_head_sha256"`
	ReviewSnapshotSHA256 string    `json:"review_snapshot_sha256"`
	ContentSHA256        string    `json:"content_sha256"`
}

type AuditEvent struct {
	Sequence       int       `json:"sequence"`
	Action         string    `json:"action"`
	ActorID        string    `json:"actor_id"`
	At             time.Time `json:"at"`
	PreviousSHA256 string    `json:"previous_sha256"`
	SHA256         string    `json:"sha256"`
}
