package domain

import (
	"errors"
	"testing"
	"time"
)

func TestReviseAuthorizationNormalizesAndRecordsDiff(t *testing.T) {
	now := time.Unix(100, 0)
	d, err := NewDossier(CreateDossierInput{DossierID: "d", SubjectCode: "S1", AudioRef: "audio", AudioSHA256: Digest("audio"), InterviewedAt: "2026-01-01", AllowedUses: []string{"research"}, EmbargoUntil: "2027-01-01", ConsentEvidenceDigest: Digest("evidence"), EditorID: "editor"}, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.ReviseAuthorization(AuthorizationInput{SubjectCode: "S2", AudioRef: "audio", AudioSHA256: Digest("audio"), InterviewedAt: "2026-01-01", AllowedUses: []string{" research ", "exhibition", "research"}, EmbargoUntil: "2027-01-01", ConsentEvidenceDigest: Digest("evidence")}, "editor", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if d.Revision != 2 || len(d.AllowedUses) != 2 || len(d.AuthorizationRevisions) != 1 {
		t.Fatalf("修订结果不完整: %#v", d)
	}
	if len(d.AuthorizationRevisions[0].ChangedFields) != 2 {
		t.Fatalf("差异字段错误: %#v", d.AuthorizationRevisions[0])
	}
	d.Status = StatusConsentLocked
	before := d.SubjectCode
	_, err = d.ReviseAuthorization(AuthorizationInput{}, "editor", now)
	if !errors.Is(err, ErrInvalidState) || d.SubjectCode != before {
		t.Fatal("锁定后修订应被原子拒绝")
	}
}

func TestTranscriptOperationsPreserveStableSegments(t *testing.T) {
	oldHash := Digest("one")
	d := &InterviewDossier{DossierID: "d", Status: StatusConsentLocked, Revision: 4, Segments: []TranscriptSegment{{SegmentID: "s1", StartMS: 0, EndMS: 10, SpeakerCode: "A", Text: "one", TextSHA256: oldHash, Sequence: 1, Revision: 3}, {SegmentID: "s2", StartMS: 10, EndMS: 20, SpeakerCode: "B", Text: "two", TextSHA256: Digest("two"), Sequence: 2, Revision: 3}, {SegmentID: "s3", StartMS: 20, EndMS: 30, SpeakerCode: "C", Text: "three", TextSHA256: Digest("three"), Sequence: 3, Revision: 3}}}
	err := d.ApplyTranscriptOperations([]TranscriptOperation{{Type: "update", SegmentID: "s2", Segment: &TranscriptSegment{StartMS: 10, EndMS: 20, SpeakerCode: "B", Text: "changed"}}, {Type: "delete", SegmentID: "s3"}, {Type: "add", SegmentID: "s4", Segment: &TranscriptSegment{StartMS: 20, EndMS: 35, SpeakerCode: "D", Text: "four"}}}, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if d.Revision != 5 || len(d.Segments) != 3 || d.Segments[0].SegmentID != "s1" || d.Segments[0].TextSHA256 != oldHash || d.Segments[1].TextSHA256 == Digest("two") {
		t.Fatalf("增量结果错误: %#v", d.Segments)
	}
}

func TestTranscriptAndRedactionBatchFailuresAreAtomic(t *testing.T) {
	d := &InterviewDossier{DossierID: "d", Status: StatusConsentLocked, Revision: 1}
	err := d.ApplyTranscriptOperations([]TranscriptOperation{{Type: "add", SegmentID: "a", Segment: &TranscriptSegment{StartMS: 0, EndMS: 20, SpeakerCode: "A", Text: "ok"}}, {Type: "add", SegmentID: "b", Segment: &TranscriptSegment{StartMS: 10, EndMS: 30, SpeakerCode: "B", Text: " "}}}, time.Now())
	var validation ValidationErrors
	if !errors.As(err, &validation) || len(validation.Items) < 2 || len(d.Segments) != 0 || d.Revision != 1 {
		t.Fatalf("时间轴批次未原子拒绝: %v %#v", err, d)
	}
	text := "abcdefghij"
	d.Status = StatusRemediation
	d.Segments = []TranscriptSegment{{SegmentID: "s", DossierID: "d", Text: text, TextSHA256: Digest(text)}}
	d.Issues = []RedactionIssue{{IssueID: "i1", DossierID: "d", SegmentID: "s", Severity: SeverityBlocker, Status: IssueOpen}, {IssueID: "i2", DossierID: "d", SegmentID: "s", Severity: SeverityBlocker, Status: IssueOpen}}
	err = d.ResolveIssues([]RedactionResolution{{IssueID: "i1", StartOffset: 1, EndOffset: 5, Reason: "r", ReplacementText: "x", ActorID: "e", SegmentTextSHA256: Digest(text)}, {IssueID: "i2", StartOffset: 4, EndOffset: 8, Reason: "r", ReplacementText: "y", ActorID: "e", SegmentTextSHA256: Digest(text)}}, time.Now())
	if !errors.As(err, &validation) || d.Issues[0].Status != IssueOpen || d.Issues[1].Status != IssueOpen || len(d.Audit) != 0 {
		t.Fatalf("遮蔽批次未原子拒绝: %v %#v", err, d.Issues)
	}
}
