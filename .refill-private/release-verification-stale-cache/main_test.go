package release_verification_stale_cache_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"oralarchive/internal/application"
	"oralarchive/internal/domain"
	"oralarchive/internal/httpui"
	"oralarchive/internal/release"
	"oralarchive/internal/repository"

	_ "modernc.org/sqlite"
)

func TestReleaseVerificationRefreshesAfterPersistedPackageChange(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "archive.db")
	store, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2025, 4, 5, 6, 7, 8, 0, time.UTC)
	dossier := &domain.InterviewDossier{
		DossierID:             "verification-cache-dossier",
		Status:                domain.StatusReview,
		SubjectCode:           "SUBJECT-1",
		AudioSHA256:           domain.Digest("audio"),
		AllowedUses:           []string{"research"},
		ConsentEvidenceDigest: domain.Digest("consent"),
		EditorID:              "editor-1",
		Revision:              1,
		CreatedAt:             now,
		UpdatedAt:             now,
		Segments: []domain.TranscriptSegment{{
			SegmentID: "segment-1", DossierID: "verification-cache-dossier",
			StartMS: 0, EndMS: 1000, SpeakerCode: "SUBJECT-1", Text: "可公开内容",
			TextSHA256: domain.Digest("可公开内容"), Sequence: 1, Revision: 1,
		}},
		Reviews: []domain.ReviewSnapshot{{
			ReviewID: "review-1", DossierID: "verification-cache-dossier",
			Decision: "approved", ReviewerID: "reviewer-1", SnapshotSHA256: domain.Digest("review"), CreatedAt: now,
		}},
	}
	dossier.AppendAudit("review_approved", "reviewer-1", now)
	pkg, err := release.Generate(dossier, "reviewer-1", now)
	if err != nil {
		t.Fatal(err)
	}
	if err = dossier.Seal(pkg, "reviewer-1", now); err != nil {
		t.Fatal(err)
	}
	if err = store.Create(ctx, dossier, "", "", 201, nil); err != nil {
		t.Fatal(err)
	}

	handler := httpui.New(application.New(store)).Handler()
	first := requestVerification(t, handler, dossier.DossierID)
	if !first.Valid {
		t.Fatal("初始发布包应通过校验")
	}

	external, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer external.Close()
	if _, err = external.ExecContext(ctx, `UPDATE release_packages SET redacted_transcript=? WHERE dossier_id=?`, "已损坏的发布正文", dossier.DossierID); err != nil {
		t.Fatal(err)
	}

	persisted, err := store.GetReleasePackage(ctx, dossier.DossierID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.RedactedTranscript != "已损坏的发布正文" {
		t.Fatal("测试前置条件未建立：持久化发布正文没有改变")
	}
	second := requestVerification(t, handler, dossier.DossierID)
	if second.Valid {
		t.Fatal("持久化发布包变化后仍复用了过期的有效校验结果")
	}
}

func requestVerification(t *testing.T, handler http.Handler, dossierID string) release.VerificationReport {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/dossiers/"+dossierID+"/release/verification", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("发布校验返回状态 %d: %s", recorder.Code, recorder.Body.String())
	}
	var report release.VerificationReport
	if err := json.NewDecoder(recorder.Body).Decode(&report); err != nil {
		t.Fatal(err)
	}
	return report
}
