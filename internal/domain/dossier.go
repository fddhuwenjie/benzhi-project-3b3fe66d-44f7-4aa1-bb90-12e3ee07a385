package domain

import (
	"fmt"
	"strings"
	"time"
)

type CreateDossierInput struct {
	DossierID             string   `json:"dossier_id"`
	SubjectCode           string   `json:"subject_code"`
	AudioRef              string   `json:"audio_ref"`
	AudioSHA256           string   `json:"audio_sha256"`
	InterviewedAt         string   `json:"interviewed_at"`
	AllowedUses           []string `json:"allowed_uses"`
	EmbargoUntil          string   `json:"embargo_until"`
	ConsentEvidenceDigest string   `json:"consent_evidence_digest"`
	EditorID              string   `json:"editor_id"`
}

func NewDossier(in CreateDossierInput, now time.Time) (*InterviewDossier, error) {
	if Clean(in.DossierID) == "" {
		return nil, Invalid("dossier_id", "不能为空")
	}
	uses, err := NormalizeUses(in.AllowedUses)
	if err != nil {
		return nil, err
	}
	if err := ValidateAuthorization(in.SubjectCode, in.AudioRef, in.AudioSHA256, in.InterviewedAt, in.ConsentEvidenceDigest, uses); err != nil {
		return nil, err
	}
	if err := ValidateEmbargo(in.InterviewedAt, in.EmbargoUntil); err != nil {
		return nil, err
	}
	if Clean(in.EditorID) == "" {
		return nil, Invalid("editor_id", "不能为空")
	}
	return &InterviewDossier{DossierID: Clean(in.DossierID), Status: StatusDraft, SubjectCode: Clean(in.SubjectCode), AudioRef: Clean(in.AudioRef), AudioSHA256: strings.ToLower(in.AudioSHA256), InterviewedAt: in.InterviewedAt, AllowedUses: uses, EmbargoUntil: in.EmbargoUntil, ConsentEvidenceDigest: strings.ToLower(in.ConsentEvidenceDigest), EditorID: Clean(in.EditorID), Revision: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (d *InterviewDossier) EnsureMutable() error {
	if d.Status == StatusSealed {
		return ErrTerminal
	}
	return nil
}

func (d *InterviewDossier) CheckRevision(expected int64) error {
	if expected != d.Revision {
		return ErrConflict
	}
	return nil
}

func (d *InterviewDossier) Advance(now time.Time) { d.Revision++; d.UpdatedAt = now.UTC() }

func (d *InterviewDossier) LockConsent(now time.Time) error {
	if err := d.EnsureMutable(); err != nil {
		return err
	}
	if d.Status != StatusDraft {
		return ErrInvalidState
	}
	if d.SubjectCode == "" || d.AudioRef == "" || !ValidSHA256(d.AudioSHA256) || len(d.AllowedUses) == 0 || !ValidSHA256(d.ConsentEvidenceDigest) {
		return ErrValidation
	}
	d.Status = StatusConsentLocked
	d.Advance(now)
	return nil
}

func (d *InterviewDossier) ReplaceSegments(segments []TranscriptSegment, now time.Time) error {
	if err := d.EnsureMutable(); err != nil {
		return err
	}
	if d.Status != StatusConsentLocked {
		return ErrInvalidState
	}
	if err := ValidateSegments(segments); err != nil {
		return err
	}
	for i := range segments {
		s := &segments[i]
		if s.StartMS < 0 || s.EndMS <= s.StartMS {
			return Invalid("segments", "时间范围必须递增")
		}
		if i > 0 && segments[i-1].EndMS > s.StartMS {
			return Invalid("segments", "时间片段不能重叠")
		}
		if Clean(s.Text) == "" {
			return Invalid("segments", "正文不能为空")
		}
		if Clean(s.SpeakerCode) == "" {
			s.SpeakerCode = "UNDECLARED"
		}
		s.DossierID, s.Sequence, s.TextSHA256, s.Revision = d.DossierID, i+1, Digest(s.Text), d.Revision+1
		if s.SegmentID == "" {
			s.SegmentID = fmt.Sprintf("seg-%03d", i+1)
		}
	}
	d.Segments = append([]TranscriptSegment(nil), segments...)
	d.Advance(now)
	return nil
}

func (d *InterviewDossier) Freeze(now time.Time) error {
	if d.Status != StatusConsentLocked || len(d.Segments) == 0 {
		return ErrInvalidState
	}
	d.Status = StatusFrozen
	d.Advance(now)
	return nil
}
