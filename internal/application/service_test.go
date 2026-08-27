package application

import (
	"context"
	"oralarchive/internal/domain"
	"oralarchive/internal/repository"
	"testing"
)

func TestWorkflowAndConflict(t *testing.T) {
	store, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	svc := New(store)
	ctx := context.Background()
	hash := domain.Digest("x")
	res, err := svc.Create(ctx, Metadata{RequestID: "1"}, domain.CreateDossierInput{DossierID: "d", SubjectCode: "S", AudioRef: "audio", AudioSHA256: hash, InterviewedAt: "2024-01-01", AllowedUses: []string{"research"}, ConsentEvidenceDigest: hash, EditorID: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.LockConsent(ctx, "d", ActionInput{Metadata: Metadata{RequestID: "2", ExpectedRevision: res.Dossier.Revision}, ActorID: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.LockConsent(ctx, "d", ActionInput{Metadata: Metadata{RequestID: "3", ExpectedRevision: 1}, ActorID: "editor"})
	if !IsConflict(err) {
		t.Fatalf("want conflict, got %v", err)
	}
}
