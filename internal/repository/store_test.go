package repository

import (
	"context"
	"oralarchive/internal/domain"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d, err := domain.NewDossier(domain.CreateDossierInput{DossierID: "d1", SubjectCode: "S", AudioRef: "a", AudioSHA256: domain.Digest("a"), InterviewedAt: "2024-01-01", AllowedUses: []string{"research"}, ConsentEvidenceDigest: domain.Digest("c"), EditorID: "e"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	d.AppendAudit("create", "e", time.Now())
	if err := s.Create(context.Background(), d, "r1", "f1", 201, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background(), "d1")
	if err != nil || got.DossierID != "d1" {
		t.Fatalf("get: %v %#v", err, got)
	}
	replay, err := s.Replay(context.Background(), "r1", "f1")
	if err != nil || !replay.Found || replay.Status != 201 {
		t.Fatalf("replay: %v %#v", err, replay)
	}
}
