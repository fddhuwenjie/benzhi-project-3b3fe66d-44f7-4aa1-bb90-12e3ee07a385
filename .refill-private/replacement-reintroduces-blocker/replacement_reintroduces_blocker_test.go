package replacement_reintroduces_blocker_test

import (
	"testing"
	"time"

	"oralarchive/internal/application"
	"oralarchive/internal/domain"
)

func TestReplacementCannotReintroduceBlockingIdentifier(t *testing.T) {
	text := "联系电话 13800138000"
	dossier := &domain.InterviewDossier{
		DossierID:   "replacement-1",
		Status:      domain.StatusFrozen,
		AllowedUses: []string{"research"},
		Revision:    3,
		Segments: []domain.TranscriptSegment{{
			SegmentID: "segment-1", DossierID: "replacement-1", StartMS: 0, EndMS: 1000,
			SpeakerCode: "SPEAKER", Text: text, TextSHA256: domain.Digest(text), Revision: 3,
		}},
	}
	issues := application.CheckEligibility(dossier)
	if len(issues) != 1 || issues[0].RuleCode != "DIRECT_IDENTIFIER" {
		t.Fatalf("test setup produced unexpected issues: %#v", issues)
	}
	if err := dossier.SetIssues(issues, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	issue := dossier.Issues[0]
	err := dossier.ResolveIssues([]domain.RedactionResolution{{
		IssueID: issue.IssueID, StartOffset: issue.StartOffset, EndOffset: issue.EndOffset,
		Reason: "替换标识符", ReplacementText: "13900139000", ActorID: "editor-1",
		SegmentTextSHA256: dossier.Segments[0].TextSHA256,
	}}, time.Unix(101, 0))
	if err != nil {
		return
	}
	candidate, candidateErr := dossier.CandidateText()
	if candidateErr != nil {
		t.Fatal(candidateErr)
	}
	rescanned := &domain.InterviewDossier{
		DossierID: "replacement-1", AllowedUses: dossier.AllowedUses,
		Segments: []domain.TranscriptSegment{{SegmentID: "candidate", DossierID: "replacement-1", SpeakerCode: "SPEAKER", Text: candidate}},
	}
	unsafeBlockers := 0
	for _, issue := range application.CheckEligibility(rescanned) {
		if issue.Severity == domain.SeverityBlocker {
			unsafeBlockers++
		}
	}
	if dossier.Status == domain.StatusConfirmation && unsafeBlockers > 0 {
		t.Fatalf("unsafe replacement advanced to confirmation: %q", candidate)
	}
}
