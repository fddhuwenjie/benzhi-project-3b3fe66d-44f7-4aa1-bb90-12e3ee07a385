package audit_timestamp_integrity_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"oralarchive/internal/domain"
	"oralarchive/internal/repository"
)

func TestAuditTimestampMutationIsDetected(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "audit.db")
	store, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	at := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	dossier, err := domain.NewDossier(domain.CreateDossierInput{
		DossierID:             "audit-1",
		SubjectCode:           "SUB-1",
		AudioRef:              "vault://audit.wav",
		AudioSHA256:           domain.Digest("audio"),
		InterviewedAt:         "2026-08-27",
		AllowedUses:           []string{"research"},
		ConsentEvidenceDigest: domain.Digest("evidence"),
		EditorID:              "editor-1",
	}, at)
	if err != nil {
		t.Fatal(err)
	}
	dossier.AppendAudit("dossier_created", "editor-1", at)
	if err := store.Create(ctx, dossier, "audit-create", "fingerprint", 201, []byte(`{"dossier_id":"audit-1"}`)); err != nil {
		t.Fatal(err)
	}

	dossier.Audit[0].At = at.Add(24 * time.Hour)
	domainAccepted := domain.VerifyAudit(dossier) == nil

	external, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = external.ExecContext(ctx, `UPDATE audit_events SET occurred_at=? WHERE dossier_id=? AND sequence=?`, at.Add(48*time.Hour).Format(time.RFC3339Nano), "audit-1", 1); err != nil {
		external.Close()
		t.Fatal(err)
	}
	if err = external.Close(); err != nil {
		t.Fatal(err)
	}
	repositoryAccepted := store.VerifyAll(ctx) == nil

	if domainAccepted || repositoryAccepted {
		t.Fatalf("audit timestamp mutation was accepted: domain=%v repository=%v", domainAccepted, repositoryAccepted)
	}
}
