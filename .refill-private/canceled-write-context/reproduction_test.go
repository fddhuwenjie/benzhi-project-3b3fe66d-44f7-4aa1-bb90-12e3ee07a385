package canceledwritecontext_test

import (
	"context"
	"errors"
	"testing"

	"oralarchive/internal/application"
	"oralarchive/internal/domain"
	"oralarchive/internal/repository"
)

func TestCanceledWriteContextCannotCommit(t *testing.T) {
	store, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := application.New(store)
	hash := domain.Digest("evidence")
	input := func(id string) domain.CreateDossierInput {
		return domain.CreateDossierInput{
			DossierID: id, SubjectCode: "S-001", AudioRef: "audio.wav",
			AudioSHA256: hash, InterviewedAt: "2024-01-01",
			AllowedUses: []string{"research"}, ConsentEvidenceDigest: hash,
			EditorID: "editor-a",
		}
	}

	created, err := service.Create(context.Background(), application.Metadata{RequestID: "setup-create"}, input("existing"))
	if err != nil {
		t.Fatal(err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, lockErr := service.LockConsent(canceled, "existing", application.ActionInput{
		Metadata: application.Metadata{RequestID: "canceled-lock", ExpectedRevision: created.Dossier.Revision},
		ActorID:  "editor-a",
	})
	_, createErr := service.Create(canceled, application.Metadata{RequestID: "canceled-create"}, input("unexpected"))

	existing, getErr := store.Get(context.Background(), "existing")
	if getErr != nil {
		t.Fatal(getErr)
	}
	_, unexpectedErr := store.Get(context.Background(), "unexpected")
	if !errors.Is(lockErr, context.Canceled) || !errors.Is(createErr, context.Canceled) ||
		existing.Status != domain.StatusDraft || !errors.Is(unexpectedErr, domain.ErrNotFound) {
		t.Fatalf("已取消的写命令仍被提交: lock_err=%v create_err=%v status=%s unexpected_err=%v", lockErr, createErr, existing.Status, unexpectedErr)
	}
}
