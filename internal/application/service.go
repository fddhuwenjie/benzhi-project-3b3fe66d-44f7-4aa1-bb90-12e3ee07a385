package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"oralarchive/internal/domain"
	"oralarchive/internal/release"
	"oralarchive/internal/repository"
)

type Service struct {
	store *repository.Store
	locks *dossierLocks
	now   func() time.Time
}

func New(store *repository.Store) *Service {
	return &Service{store: store, locks: newDossierLocks(), now: time.Now}
}

func (s *Service) Get(ctx context.Context, id string) (Detail, error) {
	d, err := s.store.Get(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	detail := Detail{Dossier: d, StatusLabel: d.Status.Label()}
	if text, candidateErr := d.CandidateText(); candidateErr == nil {
		detail.CandidateSHA256 = domain.Digest(text)
	}
	if d.Package != nil {
		valid := release.Verify(*d.Package) == nil
		detail.PackageValid = &valid
	}
	if d.Status == domain.StatusReview {
		detail.ReviewChecklist = d.ReviewMaterials()
	}
	if d.Status == domain.StatusSealed {
		clone := *d
		clone.Segments = append([]domain.TranscriptSegment(nil), d.Segments...)
		for i := range clone.Segments {
			clone.Segments[i].Text = ""
		}
		detail.Dossier = &clone
	}
	return detail, nil
}

func (s *Service) List(ctx context.Context) ([]domain.InterviewDossier, error) {
	return s.store.List(ctx)
}

func (s *Service) ReviseAuthorization(ctx context.Context, id string, in AuthorizationRevisionInput) (Result, error) {
	return s.mutate(ctx, id, in.Metadata, "revise_authorization", in, in.ActorID, func(d *domain.InterviewDossier, now time.Time) error {
		_, err := d.ReviseAuthorization(in.AuthorizationInput, in.ActorID, now)
		return err
	})
}

func (s *Service) Create(ctx context.Context, meta Metadata, in domain.CreateDossierInput) (Result, error) {
	unlock := s.locks.lock(in.DossierID)
	defer unlock()
	fingerprint, err := fingerprint("create", in)
	if err != nil {
		return Result{}, err
	}
	if replay, err := s.replay(ctx, meta, fingerprint); err != nil || replay.Replayed {
		return replay, err
	}
	if err := validateMeta(meta, false); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	now := s.now()
	d, err := domain.NewDossier(in, now)
	if err != nil {
		return Result{}, err
	}
	d.AppendAudit("dossier_created", in.EditorID, now)
	res, err := result(201, d)
	if err != nil {
		return Result{}, err
	}
	if err = ctx.Err(); err != nil {
		return Result{}, err
	}
	if err = s.store.Create(ctx, d, meta.RequestID, fingerprint, res.Status, res.Body); err != nil {
		return Result{}, err
	}
	return res, nil
}

func (s *Service) LockConsent(ctx context.Context, id string, in ActionInput) (Result, error) {
	return s.mutate(ctx, id, in.Metadata, "lock_consent", in, in.ActorID, func(d *domain.InterviewDossier, now time.Time) error { return d.LockConsent(now) })
}
func (s *Service) SaveTranscript(ctx context.Context, id string, in TranscriptInput) (Result, error) {
	return s.mutate(ctx, id, in.Metadata, "save_transcript", in, "editor", func(d *domain.InterviewDossier, now time.Time) error { return d.ReplaceSegments(in.Segments, now) })
}
func (s *Service) UpdateTranscript(ctx context.Context, id string, in TranscriptOperationsInput) (Result, error) {
	return s.mutate(ctx, id, in.Metadata, "update_transcript", in, in.ActorID, func(d *domain.InterviewDossier, now time.Time) error {
		return d.ApplyTranscriptOperations(in.Operations, now)
	})
}
func (s *Service) PrecheckTranscript(ctx context.Context, id string, in TranscriptPrecheckInput) (domain.TranscriptPreflight, error) {
	d, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.TranscriptPreflight{}, err
	}
	if d.Status != domain.StatusConsentLocked {
		return domain.TranscriptPreflight{}, domain.ErrInvalidState
	}
	segments := d.Segments
	if len(in.Operations) > 0 {
		var errs []domain.LocatedError
		segments, errs = d.MergeTranscriptOperations(in.Operations)
		if len(errs) > 0 {
			return domain.TranscriptPreflight{Valid: false, Errors: errs}, nil
		}
	}
	return domain.TranscriptPrecheck(segments), nil
}
func (s *Service) FreezeTranscript(ctx context.Context, id string, in ActionInput) (Result, error) {
	return s.mutate(ctx, id, in.Metadata, "freeze_transcript", in, in.ActorID, func(d *domain.InterviewDossier, now time.Time) error { return d.Freeze(now) })
}
func (s *Service) RunCheck(ctx context.Context, id string, in ActionInput) (Result, error) {
	return s.mutate(ctx, id, in.Metadata, "run_check", in, in.ActorID, func(d *domain.InterviewDossier, now time.Time) error { return d.SetIssues(CheckEligibility(d), now) })
}
func (s *Service) Resolve(ctx context.Context, id string, in ResolveInput) (Result, error) {
	return s.mutate(ctx, id, in.Metadata, "resolve_issue", in, in.ActorID, func(d *domain.InterviewDossier, now time.Time) error {
		return d.ResolveIssue(in.IssueID, in.StartOffset, in.EndOffset, in.Reason, in.ReplacementText, in.ActorID, now)
	})
}
func (s *Service) ResolveBatch(ctx context.Context, id string, in ResolveBatchInput) (Result, error) {
	actor := ""
	if len(in.Items) > 0 {
		actor = in.Items[0].ActorID
	}
	return s.mutateWithBody(ctx, id, in.Metadata, "resolve_issues_batch", in, actor, func(d *domain.InterviewDossier, now time.Time) error { return d.ResolveIssues(in.Items, now) }, func(d *domain.InterviewDossier) ([]byte, error) {
		candidate := ""
		if text, err := d.CandidateText(); err == nil {
			candidate = domain.Digest(text)
		}
		return json.Marshal(struct {
			*domain.InterviewDossier
			CandidateSHA256 string `json:"candidate_sha256,omitempty"`
			BatchStats      struct {
				Resolved     int `json:"resolved"`
				OpenBlockers int `json:"open_blockers"`
			} `json:"batch_stats"`
		}{InterviewDossier: d, CandidateSHA256: candidate, BatchStats: struct {
			Resolved     int `json:"resolved"`
			OpenBlockers int `json:"open_blockers"`
		}{len(in.Items), len(domain.BlockingIssues(d.Issues))}})
	})
}
func (s *Service) Confirm(ctx context.Context, id string, in ConfirmationInput) (Result, error) {
	if in.Decision == "rejected" && len(in.RejectionIssueIDs) == 0 {
		return Result{}, domain.Invalid("rejection_issue_ids", "拒绝确认时必须明确关联整改问题")
	}
	return s.mutate(ctx, id, in.Metadata, "subject_confirmation", in, in.ConfirmedBy, func(d *domain.InterviewDossier, now time.Time) error {
		return d.ConfirmStructured(in.Decision, in.ConfirmedBy, in.EvidenceDigest, in.CandidateSHA256, in.AllowedExceptions, in.Note, in.RejectionIssueIDs, now)
	})
}

func (s *Service) Review(ctx context.Context, id string, in ReviewInput) (Result, error) {
	if in.Decision != "approved" && in.Decision != "rejected" {
		return Result{}, domain.Invalid("decision", "必须为 approved 或 rejected")
	}
	return s.mutate(ctx, id, in.Metadata, "review_"+in.Decision, in, in.ReviewerID, func(d *domain.InterviewDossier, now time.Time) error {
		snapshot, err := d.SubmitReview(in.Decision, in.ReviewerID, in.Reason, in.IssueIDs, in.Checklist, now)
		if err != nil {
			return err
		}
		if in.Decision == "rejected" {
			return nil
		}
		d.AppendAudit("review_approved", in.ReviewerID, now)
		pkg, err := release.Generate(d, in.ReviewerID, now)
		if err != nil {
			return err
		}
		pkg.ReviewSnapshotSHA256 = snapshot.SnapshotSHA256
		return d.Seal(pkg, in.ReviewerID, now)
	})
}

func (s *Service) mutate(ctx context.Context, id string, meta Metadata, action string, input any, actor string, change func(*domain.InterviewDossier, time.Time) error) (Result, error) {
	return s.mutateWithBody(ctx, id, meta, action, input, actor, change, nil)
}

func (s *Service) mutateWithBody(ctx context.Context, id string, meta Metadata, action string, input any, actor string, change func(*domain.InterviewDossier, time.Time) error, encode func(*domain.InterviewDossier) ([]byte, error)) (Result, error) {
	unlock := s.locks.lock(id)
	defer unlock()
	fp, err := fingerprint(action, input)
	if err != nil {
		return Result{}, err
	}
	if replay, err := s.replay(ctx, meta, fp); err != nil || replay.Replayed {
		return replay, err
	}
	if err = validateMeta(meta, true); err != nil {
		return Result{}, err
	}
	if err = ctx.Err(); err != nil {
		return Result{}, err
	}
	d, err := s.store.Get(ctx, id)
	if err != nil {
		return Result{}, err
	}
	if err = d.EnsureMutable(); err != nil {
		return Result{}, err
	}
	if err = d.CheckRevision(meta.ExpectedRevision); err != nil {
		return Result{}, err
	}
	previous := d.Revision
	now := s.now()
	if err = change(d, now); err != nil {
		return Result{}, err
	}
	if action != "review_approved" {
		d.AppendAudit(action, actor, now)
	}
	res := Result{Status: 200, Dossier: d}
	if encode == nil {
		res, err = result(200, d)
	} else {
		res.Body, err = encode(d)
	}
	if err != nil {
		return Result{}, err
	}
	if err = ctx.Err(); err != nil {
		return Result{}, err
	}
	if err = s.store.Save(ctx, d, previous, meta.RequestID, fp, res.Status, res.Body); err != nil {
		return Result{}, err
	}
	return res, nil
}

func (s *Service) replay(ctx context.Context, meta Metadata, fingerprint string) (Result, error) {
	if meta.RequestID == "" {
		return Result{}, nil
	}
	r, err := s.store.Replay(ctx, meta.RequestID, fingerprint)
	if err != nil {
		return Result{}, err
	}
	if !r.Found {
		return Result{}, nil
	}
	var d domain.InterviewDossier
	if err = json.Unmarshal(r.Body, &d); err != nil {
		return Result{}, err
	}
	return Result{Status: r.Status, Body: r.Body, Dossier: &d, Replayed: true}, nil
}

func validateMeta(meta Metadata, revision bool) error {
	if err := ValidateRequestID(meta.RequestID); err != nil {
		return err
	}
	if revision {
		if err := ValidateExpectedRevision(meta.ExpectedRevision); err != nil {
			return err
		}
	}
	return nil
}

func fingerprint(action string, input any) (string, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	return domain.Digest(fmt.Sprintf("%s|%s", action, b)), nil
}

func IsConflict(err error) bool { return errors.Is(err, domain.ErrConflict) }
