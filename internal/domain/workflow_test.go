package domain

import (
	"errors"
	"testing"
	"time"
)

func TestTranscriptRejectsReverseOrder(t *testing.T) {
	d := &InterviewDossier{DossierID: "d", Status: StatusConsentLocked}
	err := d.ReplaceSegments([]TranscriptSegment{{StartMS: 100, EndMS: 200, SpeakerCode: "S", Text: "二"}, {StartMS: 0, EndMS: 50, SpeakerCode: "S", Text: "一"}}, time.Now())
	if err == nil {
		t.Fatal("逆序片段应被拒绝")
	}
}
func TestRejectedConfirmationCreatesRemediation(t *testing.T) {
	d := &InterviewDossier{DossierID: "d", Status: StatusConfirmation, Segments: []TranscriptSegment{{SegmentID: "s", SpeakerCode: "S", Text: "候选内容"}}}
	candidate, _ := d.CandidateText()
	err := d.Confirm("rejected", "subject", Digest("e"), Digest(candidate), nil, "拒绝", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != StatusRemediation || len(BlockingIssues(d.Issues)) != 1 {
		t.Fatalf("拒绝后状态不可整改: %#v", d)
	}
}
func TestTerminalMutationRejected(t *testing.T) {
	d := &InterviewDossier{Status: StatusSealed}
	if !errors.Is(d.EnsureMutable(), ErrTerminal) {
		t.Fatal("终态必须拒绝修改")
	}
}
