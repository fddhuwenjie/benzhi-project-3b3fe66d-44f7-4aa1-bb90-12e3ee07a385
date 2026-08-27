package release

import (
	"encoding/json"
	"oralarchive/internal/domain"
)

type ComponentResult struct {
	Component      string `json:"component"`
	Valid          bool   `json:"valid"`
	ErrorCode      string `json:"error_code,omitempty"`
	ExpectedSHA256 string `json:"expected_sha256"`
	ActualSHA256   string `json:"actual_sha256"`
}
type VerificationReport struct {
	Valid                 bool              `json:"valid"`
	Components            []ComponentResult `json:"components"`
	ExpectedContentSHA256 string            `json:"expected_content_sha256"`
	ActualContentSHA256   string            `json:"actual_content_sha256"`
	ProofSHA256           string            `json:"proof_sha256"`
	ErrorCode             string            `json:"error_code,omitempty"`
	FailedComponents      []string          `json:"failed_components,omitempty"`
}

func VerifyDossier(d *domain.InterviewDossier) VerificationReport {
	if d.Package == nil {
		return VerificationReport{Components: []ComponentResult{}}
	}
	return VerifyDossierRecords(d, d.Package, lastAuditHead(d))
}

func VerifyDossierRecords(d *domain.InterviewDossier, persisted *domain.ReleasePackage, normalizedAuditHead string) VerificationReport {
	report := VerificationReport{Components: []ComponentResult{}}
	if d.Package == nil || persisted == nil {
		return report
	}
	p := persisted
	sealed := d.Package
	candidate, _ := d.CandidateText()
	reviewDigest := ""
	if len(d.Reviews) > 0 {
		reviewDigest = d.Reviews[len(d.Reviews)-1].SnapshotSHA256
	}
	head := lastAuditHead(d)
	expectedManifest := manifest{ManifestVersion: sealed.ManifestVersion, DossierID: d.DossierID, SubjectCode: d.SubjectCode, AudioSHA256: d.AudioSHA256, CandidateSHA256: domain.Digest(candidate), ReviewerID: sealed.ReviewerID, ApprovedAt: sealed.ApprovedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), AuditHeadSHA256: head, ReviewSnapshotSHA256: reviewDigest}
	mb, _ := json.Marshal(expectedManifest)
	expectedConsent := consentSnapshot{AllowedUses: append([]string(nil), d.AllowedUses...), EmbargoUntil: d.EmbargoUntil, EvidenceDigest: d.ConsentEvidenceDigest, InterviewedAt: d.InterviewedAt}
	cb, _ := json.Marshal(expectedConsent)
	add := func(name, expected, actual string) {
		valid := expected == actual
		code := ""
		if !valid {
			code = "COMPONENT_DIGEST_MISMATCH"
		}
		report.Components = append(report.Components, ComponentResult{Component: name, Valid: valid, ErrorCode: code, ExpectedSHA256: domain.Digest(expected), ActualSHA256: domain.Digest(actual)})
		if !valid {
			report.Valid = false
			report.FailedComponents = append(report.FailedComponents, name)
		}
	}
	report.Valid = true
	add("manifest", string(mb), p.Manifest)
	if p.ReviewSnapshotSHA256 != reviewDigest || sealed.ReviewSnapshotSHA256 != reviewDigest {
		markComponentFailure(&report, "manifest")
	}
	add("redacted_transcript", candidate, p.RedactedTranscript)
	add("consent_snapshot", string(cb), p.ConsentSnapshot)
	auditActual := sealed.AuditHeadSHA256 + "|" + p.AuditHeadSHA256 + "|" + normalizedAuditHead
	auditExpected := head + "|" + head + "|" + head
	add("audit_head", auditExpected, auditActual)
	report.ExpectedContentSHA256 = sealed.ContentSHA256
	report.ActualContentSHA256 = ContentDigest(p.Manifest, p.RedactedTranscript, p.ConsentSnapshot, p.AuditHeadSHA256)
	if report.ExpectedContentSHA256 != report.ActualContentSHA256 {
		report.Valid = false
		report.ErrorCode = "CONTENT_DIGEST_MISMATCH"
	}
	if !report.Valid && report.ErrorCode == "" {
		report.ErrorCode = "COMPONENT_DIGEST_MISMATCH"
	}
	proofInput := struct {
		Components []ComponentResult `json:"components"`
		Expected   string            `json:"expected"`
		Actual     string            `json:"actual"`
	}{report.Components, report.ExpectedContentSHA256, report.ActualContentSHA256}
	proof, _ := json.Marshal(proofInput)
	report.ProofSHA256 = domain.Digest(string(proof))
	return report
}

func lastAuditHead(d *domain.InterviewDossier) string {
	if len(d.Audit) == 0 {
		return ""
	}
	return d.Audit[len(d.Audit)-1].SHA256
}

func markComponentFailure(report *VerificationReport, name string) {
	for i := range report.Components {
		if report.Components[i].Component == name {
			report.Components[i].Valid = false
			report.Components[i].ErrorCode = "COMPONENT_DIGEST_MISMATCH"
		}
	}
	report.Valid = false
	for _, item := range report.FailedComponents {
		if item == name {
			return
		}
	}
	report.FailedComponents = append(report.FailedComponents, name)
}
