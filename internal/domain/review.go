package domain

import (
	"encoding/json"
	"sort"
	"time"
)

const (
	ReviewAuthorization  = "authorization_integrity"
	ReviewTranscript     = "transcript_check"
	ReviewRedaction      = "redaction_result"
	ReviewConfirmation   = "confirmation_material"
	ReviewRoleSeparation = "role_separation"
)

func ReviewCodes() []string {
	return []string{ReviewAuthorization, ReviewTranscript, ReviewRedaction, ReviewConfirmation, ReviewRoleSeparation}
}

func (d *InterviewDossier) ReviewMaterials() []ReviewChecklistItem {
	values := map[string]any{
		ReviewAuthorization: AuthorizationInput{d.SubjectCode, d.AudioRef, d.AudioSHA256, d.InterviewedAt, d.AllowedUses, d.EmbargoUntil, d.ConsentEvidenceDigest},
		ReviewTranscript: struct {
			Segments []TranscriptSegment `json:"segments"`
			Issues   []RedactionIssue    `json:"issues"`
		}{d.Segments, d.Issues},
		ReviewRedaction:    d.Issues,
		ReviewConfirmation: d.latestConfirmation(),
		ReviewRoleSeparation: struct {
			EditorID string `json:"editor_id"`
		}{d.EditorID},
	}
	items := make([]ReviewChecklistItem, 0, 5)
	for _, code := range ReviewCodes() {
		b, _ := json.Marshal(values[code])
		items = append(items, ReviewChecklistItem{Code: code, MaterialSHA256: Digest(string(b))})
	}
	return items
}

func (d *InterviewDossier) SubmitReview(decision, reviewer, reason string, issueIDs []string, submitted []ReviewChecklistItem, now time.Time) (*ReviewSnapshot, error) {
	if d.Status != StatusReview {
		return nil, ErrInvalidState
	}
	if Clean(reviewer) == "" {
		return nil, Invalid("reviewer_id", "不能为空")
	}
	if reviewer == d.EditorID {
		return nil, ErrRoleSeparation
	}
	current := d.ReviewMaterials()
	currentByCode := map[string]string{}
	for _, item := range current {
		currentByCode[item.Code] = item.MaterialSHA256
	}
	seen := map[string]bool{}
	errs := []LocatedError{}
	failures := 0
	for i, item := range submitted {
		if seen[item.Code] {
			errs = append(errs, LocatedError{Field: "checklist", Code: "duplicate", Message: "清单代码重复", Index: i})
			continue
		}
		seen[item.Code] = true
		expected, ok := currentByCode[item.Code]
		if !ok {
			errs = append(errs, LocatedError{Field: "checklist", Code: "unknown", Message: "未知清单代码", Index: i})
			continue
		}
		if item.MaterialSHA256 != expected {
			errs = append(errs, LocatedError{Field: "material_sha256", Code: "stale", Message: "复核材料摘要已变化", Index: i})
		}
		if item.Conclusion != "passed" && item.Conclusion != "rejected" {
			errs = append(errs, LocatedError{Field: "conclusion", Code: "invalid", Message: "结论必须为 passed 或 rejected", Index: i})
		}
		if item.Conclusion == "rejected" {
			failures++
		}
	}
	for _, code := range ReviewCodes() {
		if !seen[code] {
			errs = append(errs, LocatedError{Field: "checklist", Code: "missing", Message: "缺少清单项 " + code})
		}
	}
	if decision != "approved" && decision != "rejected" {
		errs = append(errs, LocatedError{Field: "decision", Code: "invalid", Message: "必须为 approved 或 rejected"})
	}
	if decision == "approved" && failures > 0 {
		errs = append(errs, LocatedError{Field: "checklist", Code: "rejected", Message: "批准时所有清单项必须通过"})
	}
	if decision == "rejected" && failures == 0 {
		errs = append(errs, LocatedError{Field: "checklist", Code: "failure_required", Message: "驳回至少需要一个失败清单项"})
	}
	if decision == "rejected" && (Clean(reason) == "" || len(issueIDs) == 0) {
		errs = append(errs, LocatedError{Field: "review", Code: "required", Message: "驳回理由和整改问题不能为空"})
	}
	if d.HasOpenBlockers() {
		errs = append(errs, LocatedError{Field: "issues", Code: "open_blocker", Message: "仍有开放阻断问题"})
	}
	candidateText, candidateErr := d.CandidateText()
	if candidateErr != nil {
		return nil, candidateErr
	}
	candidate := Digest(candidateText)
	latest := d.latestConfirmation()
	if latest == nil || latest.Decision != "approved" || latest.CandidateSHA256 != candidate {
		errs = append(errs, LocatedError{Field: "confirmation", Code: "candidate_mismatch", Message: "候选稿与确认摘要不一致"})
	}
	selected := map[string]bool{}
	for _, id := range issueIDs {
		selected[id] = true
	}
	for id := range selected {
		issue := d.issue(id)
		if issue == nil {
			errs = append(errs, LocatedError{Field: "issue_ids", Code: "not_found", Message: "整改问题不属于当前档案", IssueID: id})
		} else if issue.Status != IssueResolved {
			errs = append(errs, LocatedError{Field: "issue_ids", Code: "not_resolved", Message: "只能驳回已处理的问题", IssueID: id})
		}
	}
	if len(errs) > 0 {
		return nil, ValidationErrors{Items: errs}
	}
	issueIDs = issueIDs[:0]
	for id := range selected {
		issueIDs = append(issueIDs, id)
	}
	sort.Strings(issueIDs)
	conclusions := map[string]string{}
	for _, item := range submitted {
		conclusions[item.Code] = item.Conclusion
	}
	normalizedChecklist := d.ReviewMaterials()
	for i := range normalizedChecklist {
		normalizedChecklist[i].Conclusion = conclusions[normalizedChecklist[i].Code]
	}
	canonical := struct {
		Decision, Reviewer, Reason, Candidate string
		Items                                 []ReviewChecklistItem
		IssueIDs                              []string
	}{decision, reviewer, Clean(reason), candidate, normalizedChecklist, issueIDs}
	b, _ := json.Marshal(canonical)
	snapshot := ReviewSnapshot{ReviewID: "review-" + Digest(string(b) + now.UTC().Format(time.RFC3339Nano))[:12], DossierID: d.DossierID, Decision: decision, ReviewerID: reviewer, Checklist: normalizedChecklist, Reason: Clean(reason), IssueIDs: append([]string(nil), issueIDs...), CandidateSHA256: candidate, SnapshotSHA256: Digest(string(b)), SubmittedRevision: d.Revision, CreatedAt: now.UTC()}
	d.Reviews = append(d.Reviews, snapshot)
	if decision == "rejected" {
		round := RemediationRound{RoundNumber: len(d.RemediationRounds) + 1, ReviewerID: reviewer, Reason: Clean(reason), IssueIDs: append([]string(nil), issueIDs...), RejectedCandidateSHA256: candidate, StartedAt: now.UTC()}
		for _, id := range issueIDs {
			issue := d.issue(id)
			issue.CurrentRound = round.RoundNumber
			issue.Status = IssueOpen
			issue.ReplacementText = ""
			issue.ResolvedAt = nil
			issue.ResolvedBy = ""
		}
		d.RemediationRounds = append(d.RemediationRounds, round)
		d.Status = StatusRemediation
		d.Advance(now)
	}
	return &snapshot, nil
}

func (d *InterviewDossier) latestConfirmation() *SubjectConfirmation {
	if len(d.Confirmations) == 0 {
		return nil
	}
	return &d.Confirmations[len(d.Confirmations)-1]
}
