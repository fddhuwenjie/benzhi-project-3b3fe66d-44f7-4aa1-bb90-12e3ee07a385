package httpui

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"oralarchive/internal/application"
	"oralarchive/internal/domain"
)

//go:embed static/*
var assets embed.FS

type Server struct {
	app *application.Service
	mux *http.ServeMux
}

func New(app *application.Service) *Server {
	s := &Server{app: app, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return security(s.mux) }

func (s *Server) routes() {
	static, _ := fs.Sub(assets, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	s.mux.HandleFunc("GET /", s.RootHandler)
	s.mux.HandleFunc("GET /workbench", s.WorkbenchHandler)
	s.mux.HandleFunc("GET /healthz", s.HealthHandler)
	s.mux.HandleFunc("GET /api/dossiers", s.ListDossiersHandler)
	s.mux.HandleFunc("POST /api/dossiers", s.CreateDossierHandler)
	s.mux.HandleFunc("GET /api/dossiers/{id}", s.GetDossierHandler)
	s.mux.HandleFunc("POST /api/dossiers/{id}/consent/lock", s.LockConsentHandler)
	s.mux.HandleFunc("PATCH /api/dossiers/{id}/consent", s.ReviseConsentHandler)
	s.mux.HandleFunc("PUT /api/dossiers/{id}/consent", s.ReviseConsentHandler)
	s.mux.HandleFunc("PUT /api/dossiers/{id}/transcript", s.SaveTranscriptHandler)
	s.mux.HandleFunc("PATCH /api/dossiers/{id}/transcript", s.UpdateTranscriptHandler)
	s.mux.HandleFunc("POST /api/dossiers/{id}/transcript/operations", s.UpdateTranscriptHandler)
	s.mux.HandleFunc("POST /api/dossiers/{id}/transcript/precheck", s.PrecheckTranscriptHandler)
	s.mux.HandleFunc("POST /api/dossiers/{id}/transcript/freeze", s.FreezeTranscriptHandler)
	s.mux.HandleFunc("POST /api/dossiers/{id}/checks", s.RunCheckHandler)
	s.mux.HandleFunc("POST /api/dossiers/{id}/issues/resolve", s.ResolveIssueHandler)
	s.mux.HandleFunc("POST /api/dossiers/{id}/issues/batch-resolve", s.ResolveIssuesBatchHandler)
	s.mux.HandleFunc("POST /api/dossiers/{id}/confirmations", s.ConfirmationHandler)
	s.mux.HandleFunc("POST /api/dossiers/{id}/reviews", s.ReviewHandler)
	s.mux.HandleFunc("GET /api/dossiers/{id}/release/verification", s.VerifyReleaseHandler)
}

func (s *Server) RootHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/workbench", http.StatusSeeOther)
}
func (s *Server) WorkbenchHandler(w http.ResponseWriter, r *http.Request) {
	data, err := assets.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "页面不可用", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) ListDossiersHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := application.QueueFilter{EditorID: q.Get("editor_id"), SubjectCode: q.Get("subject_code"), Keyword: q.Get("keyword"), UpdatedFrom: q.Get("updated_from"), UpdatedTo: q.Get("updated_to"), Cursor: q.Get("cursor")}
	statusValues := q["status"]
	if len(statusValues) == 0 {
		statusValues = q["statuses"]
	}
	if len(statusValues) == 1 {
		statusValues = strings.Split(statusValues[0], ",")
	}
	for _, value := range statusValues {
		value = strings.TrimSpace(value)
		if value != "" {
			filter.Statuses = append(filter.Statuses, domain.Status(value))
		}
	}
	if raw := q.Get("page_size"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil {
			writeError(w, domain.Invalid("page_size", "必须为整数"))
			return
		}
		filter.PageSize = n
	}
	items, err := s.app.QueryQueue(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) ReviseConsentHandler(w http.ResponseWriter, r *http.Request) {
	var in application.AuthorizationRevisionInput
	if !decode(w, r, &in) {
		return
	}
	if !validDate(w, "interviewed_at", in.InterviewedAt, false) || !validDate(w, "embargo_until", in.EmbargoUntil, true) {
		return
	}
	res, err := s.app.ReviseAuthorization(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) GetDossierHandler(w http.ResponseWriter, r *http.Request) {
	detail, err := s.app.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, detail)
}
func (s *Server) CreateDossierHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		application.Metadata
		domain.CreateDossierInput
	}
	if !decode(w, r, &in) {
		return
	}
	if !validDate(w, "interviewed_at", in.InterviewedAt, false) || !validDate(w, "embargo_until", in.EmbargoUntil, true) {
		return
	}
	res, err := s.app.Create(r.Context(), in.Metadata, in.CreateDossierInput)
	writeResult(w, res, err)
}
func (s *Server) LockConsentHandler(w http.ResponseWriter, r *http.Request) {
	var in application.ActionInput
	if !decode(w, r, &in) {
		return
	}
	res, err := s.app.LockConsent(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) SaveTranscriptHandler(w http.ResponseWriter, r *http.Request) {
	var in application.TranscriptInput
	if !decode(w, r, &in) {
		return
	}
	res, err := s.app.SaveTranscript(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) UpdateTranscriptHandler(w http.ResponseWriter, r *http.Request) {
	var in application.TranscriptOperationsInput
	if !decode(w, r, &in) {
		return
	}
	if len(in.Operations) > 200 {
		writeError(w, domain.Invalid("operations", "单次最多 200 项"))
		return
	}
	res, err := s.app.UpdateTranscript(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) PrecheckTranscriptHandler(w http.ResponseWriter, r *http.Request) {
	var in application.TranscriptPrecheckInput
	if !decode(w, r, &in) {
		return
	}
	if len(in.Operations) > 200 {
		writeError(w, domain.Invalid("operations", "单次最多 200 项"))
		return
	}
	report, err := s.app.PrecheckTranscript(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, report)
}
func (s *Server) FreezeTranscriptHandler(w http.ResponseWriter, r *http.Request) {
	var in application.ActionInput
	if !decode(w, r, &in) {
		return
	}
	res, err := s.app.FreezeTranscript(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) RunCheckHandler(w http.ResponseWriter, r *http.Request) {
	var in application.ActionInput
	if !decode(w, r, &in) {
		return
	}
	res, err := s.app.RunCheck(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) ResolveIssueHandler(w http.ResponseWriter, r *http.Request) {
	var in application.ResolveInput
	if !decode(w, r, &in) {
		return
	}
	res, err := s.app.Resolve(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) ResolveIssuesBatchHandler(w http.ResponseWriter, r *http.Request) {
	var in application.ResolveBatchInput
	if !decode(w, r, &in) {
		return
	}
	if len(in.Items) > 100 {
		writeError(w, domain.Invalid("items", "单次最多处理 100 项"))
		return
	}
	for i, item := range in.Items {
		if len(item.ReplacementText) > 2000 {
			writeError(w, domain.Invalid("items["+strconv.Itoa(i)+"].replacement_text", "长度不能超过 2000"))
			return
		}
	}
	res, err := s.app.ResolveBatch(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) ConfirmationHandler(w http.ResponseWriter, r *http.Request) {
	var in application.ConfirmationInput
	if !decode(w, r, &in) {
		return
	}
	res, err := s.app.Confirm(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) ReviewHandler(w http.ResponseWriter, r *http.Request) {
	var in application.ReviewInput
	if !decode(w, r, &in) {
		return
	}
	res, err := s.app.Review(r.Context(), r.PathValue("id"), in)
	writeResult(w, res, err)
}
func (s *Server) VerifyReleaseHandler(w http.ResponseWriter, r *http.Request) {
	report, err := s.app.VerifyRelease(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, report)
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, 400, map[string]any{"error": "请求 JSON 无效", "detail": err.Error()})
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, 400, map[string]any{"error": "请求 JSON 只能包含一个对象"})
		return false
	}
	return true
}
func validDate(w http.ResponseWriter, field, value string, optional bool) bool {
	if optional && value == "" {
		return true
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		writeError(w, domain.Invalid(field, "必须为 YYYY-MM-DD 日期"))
		return false
	}
	return true
}
func writeResult(w http.ResponseWriter, res application.Result, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if res.Replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.WriteHeader(res.Status)
	w.Write(res.Body)
}
func writeError(w http.ResponseWriter, err error) {
	status := 422
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = 404
	case errors.Is(err, domain.ErrConflict):
		status = 409
	case errors.Is(err, domain.ErrTerminal):
		status = 409
	case errors.Is(err, domain.ErrInvalidState):
		status = 409
	}
	payload := map[string]any{"error": err.Error()}
	var field domain.FieldError
	if errors.As(err, &field) {
		payload["field"] = field.Field
		payload["error"] = field.Message
	}
	var validation domain.ValidationErrors
	if errors.As(err, &validation) {
		payload["errors"] = validation.Items
	}
	writeJSON(w, status, payload)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	allowJSON(w)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
