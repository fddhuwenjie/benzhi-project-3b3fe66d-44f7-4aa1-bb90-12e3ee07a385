package domain

import (
	"sort"
	"strconv"
	"time"
	"unicode/utf8"
)

func (d *InterviewDossier) ConfirmStructured(decision, by, evidence, candidate string, exceptions []ConfirmationException, note string, rejectionIssueIDs []string, now time.Time) error {
	if d.Status != StatusConfirmation {
		return ErrInvalidState
	}
	text, err := d.CandidateText()
	if err != nil {
		return err
	}
	if candidate != Digest(text) {
		return Invalid("candidate_sha256", "候选稿摘要不匹配")
	}
	if Clean(by) == "" || !ValidSHA256(evidence) {
		return Invalid("confirmation", "确认人和证据摘要必须完整")
	}
	if decision != "approved" && decision != "rejected" {
		return Invalid("decision", "必须为 approved 或 rejected")
	}
	exceptions = normalizeConfirmationExceptions(exceptions)
	if err = d.validateConfirmationExceptions(exceptions); err != nil {
		return err
	}
	if round := d.activeRound(); round != nil {
		for _, id := range round.IssueIDs {
			issue := d.issue(id)
			if issue == nil || issue.Status != IssueResolved {
				return Invalid("remediation_round", "当前轮次指定问题尚未全部解决")
			}
		}
		t := now.UTC()
		round.EndedAt = &t
		round.ConfirmedCandidateSHA256 = candidate
		round.CandidateChanged = round.RejectedCandidateSHA256 != candidate
		round.ConfirmationDecision = decision
	}
	if decision == "rejected" && Clean(note) == "" {
		return Invalid("note", "拒绝确认时必须填写原因")
	}
	c := SubjectConfirmation{ConfirmationID: "confirmation-" + Digest(d.DossierID + "|" + now.UTC().Format(time.RFC3339Nano))[:12], DossierID: d.DossierID, Decision: decision, ConfirmedBy: Clean(by), ConfirmedAt: now.UTC(), EvidenceDigest: evidence, AllowedExceptions: append([]ConfirmationException(nil), exceptions...), CandidateSHA256: candidate, Note: Clean(note)}
	d.Confirmations = append(d.Confirmations, c)
	if decision == "approved" {
		d.Status = StatusReview
	} else {
		if err := d.reopenForSubjectRejection(rejectionIssueIDs, note, candidate); err != nil {
			return err
		}
		d.Status = StatusRemediation
	}
	d.Advance(now)
	return nil
}

func normalizeConfirmationExceptions(items []ConfirmationException) []ConfirmationException {
	result := append([]ConfirmationException(nil), items...)
	for i := range result {
		result[i].SegmentID = Clean(result[i].SegmentID)
		result[i].AllowedUse = Clean(result[i].AllowedUse)
		result[i].Description = Clean(result[i].Description)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SegmentID != result[j].SegmentID {
			return result[i].SegmentID < result[j].SegmentID
		}
		if result[i].StartOffset != result[j].StartOffset {
			return result[i].StartOffset < result[j].StartOffset
		}
		if result[i].EndOffset != result[j].EndOffset {
			return result[i].EndOffset < result[j].EndOffset
		}
		return result[i].AllowedUse < result[j].AllowedUse
	})
	return result
}

// Confirm keeps the original domain entry point available for existing callers.
func (d *InterviewDossier) Confirm(decision, by, evidence, candidate string, exceptions []string, note string, now time.Time) error {
	if len(exceptions) > 0 {
		return Invalid("allowed_exceptions", "请使用结构化例外范围")
	}
	return d.ConfirmStructured(decision, by, evidence, candidate, nil, note, nil, now)
}

func (d *InterviewDossier) validateConfirmationExceptions(exceptions []ConfirmationException) error {
	errs := []LocatedError{}
	allowed := map[string]bool{}
	for _, use := range d.AllowedUses {
		allowed[use] = true
	}
	seen := map[string]bool{}
	ranges := map[string][]ConfirmationException{}
	for i, e := range exceptions {
		seg := d.segment(e.SegmentID)
		if seg == nil {
			errs = append(errs, LocatedError{Field: "segment_id", Code: "not_found", Message: "例外引用了未知片段", SegmentID: e.SegmentID, Index: i})
			continue
		}
		candidate, err := d.candidateSegmentText(seg.SegmentID)
		if err != nil {
			return err
		}
		length := utf8.RuneCountInString(candidate)
		if e.StartOffset < 0 || e.EndOffset <= e.StartOffset || e.EndOffset > length {
			errs = append(errs, LocatedError{Field: "range", Code: "invalid", Message: "例外字符范围越界", SegmentID: e.SegmentID, Index: i})
		}
		if !allowed[Clean(e.AllowedUse)] {
			errs = append(errs, LocatedError{Field: "allowed_use", Code: "unauthorized", Message: "例外用途未获授权", SegmentID: e.SegmentID, Index: i})
		}
		if Clean(e.Description) == "" {
			errs = append(errs, LocatedError{Field: "description", Code: "required", Message: "例外说明不能为空", SegmentID: e.SegmentID, Index: i})
		}
		key := e.SegmentID + "|" + strconv.Itoa(e.StartOffset) + "|" + strconv.Itoa(e.EndOffset)
		if seen[key] {
			errs = append(errs, LocatedError{Field: "range", Code: "duplicate", Message: "例外范围重复", SegmentID: e.SegmentID, Index: i})
		}
		seen[key] = true
		ranges[e.SegmentID] = append(ranges[e.SegmentID], e)
	}
	for _, rs := range ranges {
		sort.Slice(rs, func(i, j int) bool { return rs[i].StartOffset < rs[j].StartOffset })
		for i := 1; i < len(rs); i++ {
			if rs[i-1].EndOffset > rs[i].StartOffset {
				errs = append(errs, LocatedError{Field: "range", Code: "overlap", Message: "例外范围相互重叠", SegmentID: rs[i].SegmentID})
			}
		}
	}
	if len(errs) > 0 {
		return ValidationErrors{Items: errs}
	}
	return nil
}

func (d *InterviewDossier) candidateSegmentText(segmentID string) (string, error) {
	segment := d.segment(segmentID)
	if segment == nil {
		return "", ErrNotFound
	}
	runes := []rune(segment.Text)
	fixes := []RedactionIssue{}
	for _, issue := range d.Issues {
		if issue.SegmentID == segmentID && issue.Status == IssueResolved {
			fixes = append(fixes, issue)
		}
	}
	sort.Slice(fixes, func(i, j int) bool { return fixes[i].StartOffset > fixes[j].StartOffset })
	for _, fix := range fixes {
		if fix.StartOffset < 0 || fix.EndOffset > len(runes) || fix.EndOffset <= fix.StartOffset {
			return "", ErrValidation
		}
		runes = append(append(append([]rune(nil), runes[:fix.StartOffset]...), []rune(fix.ReplacementText)...), runes[fix.EndOffset:]...)
	}
	return string(runes), nil
}

func (d *InterviewDossier) reopenForSubjectRejection(ids []string, reason, candidate string) error {
	if len(ids) == 0 {
		for _, issue := range d.Issues {
			if issue.Status == IssueResolved {
				ids = append(ids, issue.IssueID)
			}
		}
	}
	if len(ids) == 0 && len(d.Segments) > 0 {
		segment := d.Segments[0]
		id := "issue-" + Digest(d.DossierID + "|SUBJECT_REJECTION|" + candidate)[:16]
		d.Issues = append(d.Issues, RedactionIssue{IssueID: id, DossierID: d.DossierID, SegmentID: segment.SegmentID, RuleCode: "SUBJECT_REJECTION", Severity: SeverityBlocker, StartOffset: 0, EndOffset: utf8.RuneCountInString(segment.Text), Reason: reason, Status: IssueOpen})
		return nil
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		issue := d.issue(id)
		if issue == nil {
			return Invalid("rejection_issue_ids", "包含未知问题")
		}
		if issue.Status != IssueResolved {
			return Invalid("rejection_issue_ids", "只能关联已处理的问题")
		}
		issue.Status = IssueOpen
		issue.ReplacementText = ""
		issue.Reason = reason
		issue.ResolvedAt = nil
		issue.ResolvedBy = ""
	}
	return nil
}

func (d *InterviewDossier) activeRound() *RemediationRound {
	for i := len(d.RemediationRounds) - 1; i >= 0; i-- {
		if d.RemediationRounds[i].EndedAt == nil {
			return &d.RemediationRounds[i]
		}
	}
	return nil
}

func (d *InterviewDossier) RejectReview(reviewer, reason string, issueIDs []string, now time.Time) error {
	items := d.ReviewMaterials()
	for i := range items {
		items[i].Conclusion = "passed"
	}
	if len(items) > 0 {
		items[0].Conclusion = "rejected"
	}
	_, err := d.SubmitReview("rejected", reviewer, reason, issueIDs, items, now)
	return err
}

func (d *InterviewDossier) Seal(pkg ReleasePackage, reviewer string, now time.Time) error {
	if d.Status != StatusReview {
		return ErrInvalidState
	}
	if reviewer == d.EditorID {
		return ErrRoleSeparation
	}
	pkg.DossierID, pkg.ReviewerID, pkg.ApprovedAt = d.DossierID, reviewer, now.UTC()
	d.Package = &pkg
	d.Status = StatusSealed
	d.Advance(now)
	return nil
}
