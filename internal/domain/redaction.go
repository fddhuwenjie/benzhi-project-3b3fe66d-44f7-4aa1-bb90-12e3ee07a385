package domain

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func (d *InterviewDossier) SetIssues(issues []RedactionIssue, now time.Time) error {
	if d.Status != StatusFrozen {
		return ErrInvalidState
	}
	for i := range issues {
		issues[i].DossierID = d.DossierID
		issues[i].Status = IssueOpen
	}
	d.Issues = issues
	if d.HasOpenBlockers() {
		d.Status = StatusRemediation
	} else {
		d.Status = StatusConfirmation
	}
	d.Advance(now)
	return nil
}

func (d *InterviewDossier) HasOpenBlockers() bool {
	for _, issue := range d.Issues {
		if issue.Severity == SeverityBlocker && issue.Status == IssueOpen {
			return true
		}
	}
	return false
}

func (d *InterviewDossier) ResolveIssue(issueID string, start, end int, reason, replacement, actor string, now time.Time) error {
	issue := d.issue(issueID)
	if issue == nil {
		return ErrNotFound
	}
	segment := d.segment(issue.SegmentID)
	if segment == nil {
		return ErrValidation
	}
	err := d.ResolveIssues([]RedactionResolution{{IssueID: issueID, StartOffset: start, EndOffset: end, Reason: reason, ReplacementText: replacement, ActorID: actor, SegmentTextSHA256: segment.TextSHA256}}, now)
	var validation ValidationErrors
	if errors.As(err, &validation) && len(validation.Items) == 1 {
		return Invalid(validation.Items[0].Field, validation.Items[0].Message)
	}
	return err
}

func (d *InterviewDossier) segment(id string) *TranscriptSegment {
	for i := range d.Segments {
		if d.Segments[i].SegmentID == id {
			return &d.Segments[i]
		}
	}
	return nil
}

func (d *InterviewDossier) CandidateText() (string, error) {
	if d.HasOpenBlockers() {
		return "", ErrUnresolvedIssues
	}
	lines := make([]string, 0, len(d.Segments))
	for _, segment := range d.Segments {
		runes := []rune(segment.Text)
		var fixes []RedactionIssue
		for _, issue := range d.Issues {
			if issue.SegmentID == segment.SegmentID && issue.Status == IssueResolved {
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
		lines = append(lines, segment.SpeakerCode+": "+string(runes))
	}
	return strings.Join(lines, "\n"), nil
}

// directIdentifierPattern matches phone numbers, Chinese ID card numbers,
// and email addresses the same way the eligibility checker does.
var directIdentifierPattern = regexp.MustCompile(`(?:1[3-9][0-9]{9}|[0-9]{17}[0-9Xx]|[\w.+-]+@[\w.-]+\.[A-Za-z]{2,})`)

// useMarkerPattern matches inline use annotations like "[用途:research]".
var useMarkerPattern = regexp.MustCompile(`\[用途:([^\]]+)\]`)

// ScanBlockers inspects the given text for blocker-level rule hits
// (direct identifiers and unauthorized use markers) using the same rules as
// the eligibility checker. It returns one LocatedError per hit, including the
// segment id and rune offsets when provided.
func ScanBlockers(text, segmentID string, allowedUses []string) []LocatedError {
	errs := []LocatedError{}
	for _, match := range directIdentifierPattern.FindAllStringIndex(text, -1) {
		start := utf8.RuneCountInString(text[:match[0]])
		errs = append(errs, LocatedError{Field: "replacement_text", Code: "blocker", Message: "替换文本再次命中阻断规则 DIRECT_IDENTIFIER", SegmentID: segmentID, Index: start})
	}
	allowed := map[string]bool{}
	for _, use := range allowedUses {
		allowed[use] = true
	}
	for _, match := range useMarkerPattern.FindAllStringSubmatchIndex(text, -1) {
		use := text[match[2]:match[3]]
		if !allowed[use] {
			start := utf8.RuneCountInString(text[:match[0]])
			errs = append(errs, LocatedError{Field: "replacement_text", Code: "blocker", Message: "替换文本再次命中阻断规则 UNAUTHORIZED_USE", SegmentID: segmentID, Index: start})
		}
	}
	return errs
}
