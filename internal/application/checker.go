package application

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"oralarchive/internal/domain"
)

var directIdentifier = regexp.MustCompile(`(?:1[3-9][0-9]{9}|[0-9]{17}[0-9Xx]|[\w.+-]+@[\w.-]+\.[A-Za-z]{2,})`)
var useMarker = regexp.MustCompile(`\[用途:([^\]]+)\]`)

func CheckEligibility(d *domain.InterviewDossier) []domain.RedactionIssue {
	var issues []domain.RedactionIssue
	allowed := map[string]bool{}
	for _, use := range d.AllowedUses {
		allowed[use] = true
	}
	for i, segment := range d.Segments {
		if i > 0 && segment.StartMS > d.Segments[i-1].EndMS {
			issues = append(issues, issue(segment, "TIMELINE_GAP", domain.SeverityWarning, 0, 0, fmt.Sprintf("上一片段后存在 %d 毫秒未覆盖时间", segment.StartMS-d.Segments[i-1].EndMS)))
		}
		if segment.SpeakerCode == "UNDECLARED" {
			issues = append(issues, issue(segment, "UNDECLARED_SPEAKER", domain.SeverityBlocker, 0, utf8.RuneCountInString(segment.Text), "说话人未声明"))
		}
		for _, match := range directIdentifier.FindAllStringIndex(segment.Text, -1) {
			start, end := byteRangeToRune(segment.Text, match[0], match[1])
			issues = append(issues, issue(segment, "DIRECT_IDENTIFIER", domain.SeverityBlocker, start, end, "疑似直接标识符"))
		}
		for _, match := range useMarker.FindAllStringSubmatchIndex(segment.Text, -1) {
			use := segment.Text[match[2]:match[3]]
			if !allowed[use] {
				start, end := byteRangeToRune(segment.Text, match[0], match[1])
				issues = append(issues, issue(segment, "UNAUTHORIZED_USE", domain.SeverityBlocker, start, end, "片段标注用途超出授权范围"))
			}
		}
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].IssueID < issues[j].IssueID })
	return issues
}

func issue(segment domain.TranscriptSegment, rule string, severity domain.Severity, start, end int, reason string) domain.RedactionIssue {
	key := strings.Join([]string{segment.DossierID, segment.SegmentID, rule, fmt.Sprint(start), fmt.Sprint(end)}, "|")
	return domain.RedactionIssue{IssueID: "issue-" + domain.Digest(key)[:16], SegmentID: segment.SegmentID, RuleCode: rule, Severity: severity, StartOffset: start, EndOffset: end, Reason: reason}
}

func byteRangeToRune(text string, start, end int) (int, int) {
	return utf8.RuneCountInString(text[:start]), utf8.RuneCountInString(text[:end])
}
