package release

import (
	"oralarchive/internal/domain"
	"testing"
	"time"
)

func TestStablePackage(t *testing.T) {
	d := &domain.InterviewDossier{DossierID: "d", Status: domain.StatusReview, SubjectCode: "S", AudioSHA256: domain.Digest("a"), AllowedUses: []string{"research"}, ConsentEvidenceDigest: domain.Digest("c"), Segments: []domain.TranscriptSegment{{SegmentID: "s", SpeakerCode: "S", Text: "公开内容"}}}
	at := time.Unix(10, 0)
	a, err := Generate(d, "reviewer", at)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(d, "reviewer", at)
	if err != nil {
		t.Fatal(err)
	}
	if a.ContentSHA256 != b.ContentSHA256 || a.Manifest != b.Manifest {
		t.Fatal("生成结果不稳定")
	}
	if err := Verify(a); err != nil {
		t.Fatal(err)
	}
}
