package domain

func OpenIssues(issues []RedactionIssue) []RedactionIssue {
	result := make([]RedactionIssue, 0)
	for _, issue := range issues {
		if issue.Status == IssueOpen {
			result = append(result, issue)
		}
	}
	return result
}
func BlockingIssues(issues []RedactionIssue) []RedactionIssue {
	result := make([]RedactionIssue, 0)
	for _, issue := range issues {
		if issue.Status == IssueOpen && issue.Severity == SeverityBlocker {
			result = append(result, issue)
		}
	}
	return result
}
func (d *InterviewDossier) Issue(issueID string) (*RedactionIssue, bool) {
	for i := range d.Issues {
		if d.Issues[i].IssueID == issueID {
			return &d.Issues[i], true
		}
	}
	return nil, false
}
