package idempotency_replay_fingerprint_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"oralarchive/internal/application"
	"oralarchive/internal/domain"
	"oralarchive/internal/httpui"
	"oralarchive/internal/repository"
	"strings"
	"testing"
)

func TestIdempotencyReplayCacheSeparatesFingerprints(t *testing.T) {
	store, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	server := httptest.NewServer(httpui.New(application.New(store)).Handler())
	t.Cleanup(server.Close)

	first := createPayload("shared-request-id", "dossier-first", "SUBJECT-A")
	postCreate(t, server.URL, first, http.StatusCreated)
	postCreate(t, server.URL, first, http.StatusCreated)

	conflicting := createPayload("shared-request-id", "dossier-second", "SUBJECT-B")
	status, response := postCreate(t, server.URL, conflicting, 0)
	if status != http.StatusUnprocessableEntity || !strings.Contains(response, "已被不同请求使用") {
		t.Fatalf("相同 request_id 的不同请求必须被拒绝，实际 status=%d body=%s", status, response)
	}
}

func createPayload(requestID, dossierID, subjectCode string) []byte {
	payload := struct {
		application.Metadata
		domain.CreateDossierInput
	}{
		Metadata: application.Metadata{RequestID: requestID},
		CreateDossierInput: domain.CreateDossierInput{
			DossierID: dossierID, SubjectCode: subjectCode, AudioRef: "audio://archive/item",
			AudioSHA256: domain.Digest("audio"), InterviewedAt: "2024-01-02",
			AllowedUses: []string{"research"}, ConsentEvidenceDigest: domain.Digest("consent"),
			EditorID: "editor-a",
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return data
}

func postCreate(t *testing.T, baseURL string, payload []byte, expectedStatus int) (int, string) {
	t.Helper()
	response, err := http.Post(baseURL+"/api/dossiers", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if expectedStatus != 0 && response.StatusCode != expectedStatus {
		t.Fatalf("预置请求失败：status=%d body=%s", response.StatusCode, string(body))
	}
	return response.StatusCode, string(body)
}
