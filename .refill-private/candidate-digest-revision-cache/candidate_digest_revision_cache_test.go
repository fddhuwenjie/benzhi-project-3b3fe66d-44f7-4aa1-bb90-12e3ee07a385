package candidatedigestrevisioncache

import (
	"context"
	"testing"

	"oralarchive/internal/application"
	"oralarchive/internal/domain"
	"oralarchive/internal/repository"
)

func TestCandidateDigestRefreshesAfterRemediation(t *testing.T) {
	store, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := application.New(store)
	ctx := context.Background()
	evidence := domain.Digest("evidence")
	created, err := service.Create(ctx, application.Metadata{RequestID: "cache-create"}, domain.CreateDossierInput{
		DossierID:             "cache-dossier",
		SubjectCode:           "SUB-1",
		AudioRef:              "vault://cache.wav",
		AudioSHA256:           domain.Digest("audio"),
		InterviewedAt:         "2026-08-27",
		AllowedUses:           []string{"research"},
		ConsentEvidenceDigest: evidence,
		EditorID:              "editor-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	locked, err := service.LockConsent(ctx, "cache-dossier", application.ActionInput{
		Metadata: application.Metadata{RequestID: "cache-lock", ExpectedRevision: created.Dossier.Revision},
		ActorID:  "editor-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	transcript, err := service.SaveTranscript(ctx, "cache-dossier", application.TranscriptInput{
		Metadata: application.Metadata{RequestID: "cache-transcript", ExpectedRevision: locked.Dossier.Revision},
		Segments: []domain.TranscriptSegment{{
			SegmentID:   "segment-1",
			StartMS:     0,
			EndMS:       1000,
			SpeakerCode: "SUB-1",
			Text:        "联系电话 13800138000",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.FreezeTranscript(ctx, "cache-dossier", application.ActionInput{
		Metadata: application.Metadata{RequestID: "cache-freeze", ExpectedRevision: transcript.Dossier.Revision},
		ActorID:  "editor-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	checked, err := service.RunCheck(ctx, "cache-dossier", application.ActionInput{
		Metadata: application.Metadata{RequestID: "cache-check", ExpectedRevision: frozen.Dossier.Revision},
		ActorID:  "editor-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(checked.Dossier.Issues) != 1 {
		t.Fatalf("expected one direct identifier issue, got %d", len(checked.Dossier.Issues))
	}
	issue := checked.Dossier.Issues[0]
	firstResolution, err := service.Resolve(ctx, "cache-dossier", application.ResolveInput{
		Metadata:        application.Metadata{RequestID: "cache-resolve-first", ExpectedRevision: checked.Dossier.Revision},
		IssueID:         issue.IssueID,
		StartOffset:     issue.StartOffset,
		EndOffset:       issue.EndOffset,
		Reason:          "首次遮蔽",
		ReplacementText: "[已遮蔽]",
		ActorID:         "editor-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	firstDetail, err := service.Get(ctx, "cache-dossier")
	if err != nil {
		t.Fatal(err)
	}
	firstDigest := firstDetail.CandidateSHA256

	rejected, err := service.Confirm(ctx, "cache-dossier", application.ConfirmationInput{
		Metadata:          application.Metadata{RequestID: "cache-reject", ExpectedRevision: firstResolution.Dossier.Revision},
		Decision:          "rejected",
		ConfirmedBy:       "subject-1",
		EvidenceDigest:    evidence,
		CandidateSHA256:   firstDigest,
		Note:              "需要使用新的遮蔽措辞",
		RejectionIssueIDs: []string{issue.IssueID},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondResolution, err := service.Resolve(ctx, "cache-dossier", application.ResolveInput{
		Metadata:        application.Metadata{RequestID: "cache-resolve-second", ExpectedRevision: rejected.Dossier.Revision},
		IssueID:         issue.IssueID,
		StartOffset:     issue.StartOffset,
		EndOffset:       issue.EndOffset,
		Reason:          "按拒绝意见重新遮蔽",
		ReplacementText: "[依要求隐去]",
		ActorID:         "editor-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondDetail, err := service.Get(ctx, "cache-dossier")
	if err != nil {
		t.Fatal(err)
	}
	currentText, err := secondResolution.Dossier.CandidateText()
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Digest(currentText)
	if secondDetail.CandidateSHA256 != want {
		t.Fatalf("candidate digest remained cached across remediation revision: got %s want %s", secondDetail.CandidateSHA256, want)
	}
	if secondDetail.CandidateSHA256 == firstDigest && want != firstDigest {
		t.Log("stale digest came from the first confirmation lifecycle")
	}
}
