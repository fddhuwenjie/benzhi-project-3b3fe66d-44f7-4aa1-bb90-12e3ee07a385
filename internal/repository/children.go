package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"oralarchive/internal/domain"
	"time"
)

func replaceChildren(ctx context.Context, tx *sql.Tx, d *domain.InterviewDossier) error {
	for _, table := range []string{"transcript_segments", "redaction_issues", "confirmations", "release_packages", "audit_events", "authorization_revisions", "review_snapshots", "remediation_rounds", "issue_resolution_history"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE dossier_id=?", d.DossierID); err != nil {
			return err
		}
	}
	if err := insertSegments(ctx, tx, d); err != nil {
		return err
	}
	if err := insertIssues(ctx, tx, d); err != nil {
		return err
	}
	if err := insertConfirmations(ctx, tx, d); err != nil {
		return err
	}
	if err := insertPackage(ctx, tx, d); err != nil {
		return err
	}
	if err := insertExtendedHistory(ctx, tx, d); err != nil {
		return err
	}
	return insertAudit(ctx, tx, d)
}

func insertSegments(ctx context.Context, tx *sql.Tx, d *domain.InterviewDossier) error {
	q := `INSERT INTO transcript_segments(dossier_id,segment_id,sequence,start_ms,end_ms,speaker_code,text,text_sha256,revision) VALUES(?,?,?,?,?,?,?,?,?)`
	st, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, s := range d.Segments {
		if _, err = st.ExecContext(ctx, d.DossierID, s.SegmentID, s.Sequence, s.StartMS, s.EndMS, s.SpeakerCode, s.Text, s.TextSHA256, s.Revision); err != nil {
			return err
		}
	}
	return nil
}
func insertIssues(ctx context.Context, tx *sql.Tx, d *domain.InterviewDossier) error {
	q := `INSERT INTO redaction_issues(dossier_id,issue_id,segment_id,rule_code,severity,start_offset,end_offset,reason,replacement_text,original_sha256,status,resolved_by,resolved_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`
	st, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, i := range d.Issues {
		at := ""
		if i.ResolvedAt != nil {
			at = i.ResolvedAt.UTC().Format(time.RFC3339Nano)
		}
		if _, err = st.ExecContext(ctx, d.DossierID, i.IssueID, i.SegmentID, i.RuleCode, i.Severity, i.StartOffset, i.EndOffset, i.Reason, i.ReplacementText, i.OriginalSHA256, i.Status, i.ResolvedBy, at); err != nil {
			return err
		}
	}
	return nil
}
func insertConfirmations(ctx context.Context, tx *sql.Tx, d *domain.InterviewDossier) error {
	q := `INSERT INTO confirmations(dossier_id,confirmation_id,decision,confirmed_by,confirmed_at,evidence_digest,allowed_exceptions,candidate_sha256,note) VALUES(?,?,?,?,?,?,?,?,?)`
	st, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, c := range d.Confirmations {
		exceptions, e := json.Marshal(c.AllowedExceptions)
		if e != nil {
			return e
		}
		if _, err = st.ExecContext(ctx, d.DossierID, c.ConfirmationID, c.Decision, c.ConfirmedBy, c.ConfirmedAt.UTC().Format(time.RFC3339Nano), c.EvidenceDigest, exceptions, c.CandidateSHA256, c.Note); err != nil {
			return err
		}
	}
	return nil
}
func insertPackage(ctx context.Context, tx *sql.Tx, d *domain.InterviewDossier) error {
	if d.Package == nil {
		return nil
	}
	p := d.Package
	_, err := tx.ExecContext(ctx, `INSERT INTO release_packages(dossier_id,package_id,manifest_version,manifest,redacted_transcript,consent_snapshot,reviewer_id,approved_at,audit_head_sha256,review_snapshot_sha256,content_sha256) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, d.DossierID, p.PackageID, p.ManifestVersion, p.Manifest, p.RedactedTranscript, p.ConsentSnapshot, p.ReviewerID, p.ApprovedAt.UTC().Format(time.RFC3339Nano), p.AuditHeadSHA256, p.ReviewSnapshotSHA256, p.ContentSHA256)
	return err
}
func insertExtendedHistory(ctx context.Context, tx *sql.Tx, d *domain.InterviewDossier) error {
	for _, r := range d.AuthorizationRevisions {
		payload, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO authorization_revisions(dossier_id,revision_id,revision,actor_id,created_at,payload) VALUES(?,?,?,?,?,?)`, d.DossierID, r.RevisionID, r.Revision, r.ActorID, r.CreatedAt.UTC().Format(time.RFC3339Nano), payload); err != nil {
			return err
		}
	}
	for _, r := range d.Reviews {
		payload, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO review_snapshots(dossier_id,review_id,decision,reviewer_id,snapshot_sha256,created_at,payload) VALUES(?,?,?,?,?,?,?)`, d.DossierID, r.ReviewID, r.Decision, r.ReviewerID, r.SnapshotSHA256, r.CreatedAt.UTC().Format(time.RFC3339Nano), payload); err != nil {
			return err
		}
	}
	for _, r := range d.RemediationRounds {
		payload, err := json.Marshal(r)
		if err != nil {
			return err
		}
		ended := ""
		if r.EndedAt != nil {
			ended = r.EndedAt.UTC().Format(time.RFC3339Nano)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO remediation_rounds(dossier_id,round_number,started_at,ended_at,payload) VALUES(?,?,?,?,?)`, d.DossierID, r.RoundNumber, r.StartedAt.UTC().Format(time.RFC3339Nano), ended, payload); err != nil {
			return err
		}
	}
	for _, issue := range d.Issues {
		for index, h := range issue.History {
			payload, err := json.Marshal(h)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO issue_resolution_history(dossier_id,issue_id,history_index,payload) VALUES(?,?,?,?)`, d.DossierID, issue.IssueID, index+1, payload); err != nil {
				return err
			}
		}
	}
	return nil
}
func insertAudit(ctx context.Context, tx *sql.Tx, d *domain.InterviewDossier) error {
	q := `INSERT INTO audit_events(dossier_id,sequence,action,actor_id,occurred_at,previous_sha256,sha256) VALUES(?,?,?,?,?,?,?)`
	st, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, e := range d.Audit {
		if _, err = st.ExecContext(ctx, d.DossierID, e.Sequence, e.Action, e.ActorID, e.At.UTC().Format(time.RFC3339Nano), e.PreviousSHA256, e.SHA256); err != nil {
			return err
		}
	}
	return nil
}
