package release

import (
	"encoding/json"
	"fmt"
	"time"

	"oralarchive/internal/domain"
)

type manifest struct {
	ManifestVersion      string `json:"manifest_version"`
	DossierID            string `json:"dossier_id"`
	SubjectCode          string `json:"subject_code"`
	AudioSHA256          string `json:"audio_sha256"`
	CandidateSHA256      string `json:"candidate_sha256"`
	ReviewerID           string `json:"reviewer_id"`
	ApprovedAt           string `json:"approved_at"`
	AuditHeadSHA256      string `json:"audit_head_sha256"`
	ReviewSnapshotSHA256 string `json:"review_snapshot_sha256"`
}

type consentSnapshot struct {
	AllowedUses    []string `json:"allowed_uses"`
	EmbargoUntil   string   `json:"embargo_until"`
	EvidenceDigest string   `json:"evidence_digest"`
	InterviewedAt  string   `json:"interviewed_at"`
}

func Generate(d *domain.InterviewDossier, reviewer string, approvedAt time.Time) (domain.ReleasePackage, error) {
	if d.Status != domain.StatusReview {
		return domain.ReleasePackage{}, domain.ErrInvalidState
	}
	text, err := d.CandidateText()
	if err != nil {
		return domain.ReleasePackage{}, err
	}
	head := ""
	if len(d.Audit) > 0 {
		head = d.Audit[len(d.Audit)-1].SHA256
	}
	reviewDigest := ""
	if len(d.Reviews) > 0 {
		reviewDigest = d.Reviews[len(d.Reviews)-1].SnapshotSHA256
	}
	m := manifest{ManifestVersion: "2", DossierID: d.DossierID, SubjectCode: d.SubjectCode, AudioSHA256: d.AudioSHA256, CandidateSHA256: domain.Digest(text), ReviewerID: reviewer, ApprovedAt: approvedAt.UTC().Format(time.RFC3339Nano), AuditHeadSHA256: head, ReviewSnapshotSHA256: reviewDigest}
	c := consentSnapshot{AllowedUses: append([]string(nil), d.AllowedUses...), EmbargoUntil: d.EmbargoUntil, EvidenceDigest: d.ConsentEvidenceDigest, InterviewedAt: d.InterviewedAt}
	manifestBytes, err := json.Marshal(m)
	if err != nil {
		return domain.ReleasePackage{}, err
	}
	consentBytes, err := json.Marshal(c)
	if err != nil {
		return domain.ReleasePackage{}, err
	}
	digest := ContentDigest(string(manifestBytes), text, string(consentBytes), head)
	return domain.ReleasePackage{PackageID: "pkg-" + digest[:16], DossierID: d.DossierID, ManifestVersion: "2", Manifest: string(manifestBytes), RedactedTranscript: text, ConsentSnapshot: string(consentBytes), ReviewerID: reviewer, ApprovedAt: approvedAt.UTC(), AuditHeadSHA256: head, ReviewSnapshotSHA256: reviewDigest, ContentSHA256: digest}, nil
}

func Verify(pkg domain.ReleasePackage) error {
	if ContentDigest(pkg.Manifest, pkg.RedactedTranscript, pkg.ConsentSnapshot, pkg.AuditHeadSHA256) != pkg.ContentSHA256 {
		return fmt.Errorf("发布包内容摘要不匹配")
	}
	var m manifest
	if err := json.Unmarshal([]byte(pkg.Manifest), &m); err != nil {
		return fmt.Errorf("发布清单无效: %w", err)
	}
	if m.DossierID != pkg.DossierID || m.ReviewerID != pkg.ReviewerID || m.AuditHeadSHA256 != pkg.AuditHeadSHA256 || m.CandidateSHA256 != domain.Digest(pkg.RedactedTranscript) || m.ReviewSnapshotSHA256 != pkg.ReviewSnapshotSHA256 {
		return fmt.Errorf("发布清单与内容不一致")
	}
	var c consentSnapshot
	if err := json.Unmarshal([]byte(pkg.ConsentSnapshot), &c); err != nil {
		return fmt.Errorf("授权快照无效: %w", err)
	}
	if !domain.ValidSHA256(c.EvidenceDigest) {
		return fmt.Errorf("授权证据摘要无效")
	}
	return nil
}
