package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"oralarchive/internal/domain"
)

type Store struct{ db *sql.DB }

type Replay struct {
	Status int
	Body   []byte
	Found  bool
}

type DossierFilter struct {
	Statuses    map[domain.Status]bool
	EditorID    string
	SubjectCode string
	Keyword     string
	UpdatedFrom *time.Time
	UpdatedTo   *time.Time
}

type DossierSummary struct {
	DossierID             string
	Status                domain.Status
	SubjectCode, EditorID string
	Revision              int64
	UpdatedAt             time.Time
	OpenBlockers          int
}

func (s *Store) QueryDossiers(ctx context.Context, filter DossierFilter) ([]DossierSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT d.dossier_id,d.status,d.subject_code,d.editor_id,d.revision,d.updated_at,COALESCE(SUM(CASE WHEN i.status='open' AND i.severity='blocker' THEN 1 ELSE 0 END),0) FROM dossiers d LEFT JOIN redaction_issues i ON i.dossier_id=d.dossier_id GROUP BY d.dossier_id,d.status,d.subject_code,d.editor_id,d.revision,d.updated_at ORDER BY d.updated_at DESC,d.dossier_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []DossierSummary{}
	editor := strings.ToLower(strings.TrimSpace(filter.EditorID))
	subject := strings.ToLower(strings.TrimSpace(filter.SubjectCode))
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	for rows.Next() {
		var d DossierSummary
		var updated string
		if err := rows.Scan(&d.DossierID, &d.Status, &d.SubjectCode, &d.EditorID, &d.Revision, &updated, &d.OpenBlockers); err != nil {
			return nil, err
		}
		d.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
		if err != nil {
			return nil, fmt.Errorf("档案 %s 更新时间无效: %w", d.DossierID, err)
		}
		if len(filter.Statuses) > 0 && !filter.Statuses[d.Status] {
			continue
		}
		if editor != "" && strings.ToLower(strings.TrimSpace(d.EditorID)) != editor {
			continue
		}
		normalizedSubject := strings.ToLower(strings.TrimSpace(d.SubjectCode))
		if subject != "" && normalizedSubject != subject {
			continue
		}
		if keyword != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(d.DossierID)), keyword) && !strings.HasPrefix(normalizedSubject, keyword) {
			continue
		}
		if filter.UpdatedFrom != nil && d.UpdatedAt.Before(*filter.UpdatedFrom) {
			continue
		}
		if filter.UpdatedTo != nil && d.UpdatedAt.After(*filter.UpdatedTo) {
			continue
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.VerifyAll(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA journal_mode=WAL`,
		dossierTable,
		idempotencyTable,
		dossierIndex,
		segmentsTable,
		issuesTable,
		confirmationsTable,
		packagesTable,
		auditTable,
		auditDigestIndex,
		authorizationRevisionsTable,
		reviewsTable,
		remediationRoundsTable,
		issueHistoryTable,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("迁移数据库: %w", err)
		}
	}
	if err := s.ensureColumn(ctx, "release_packages", "review_snapshot_sha256", `ALTER TABLE release_packages ADD COLUMN review_snapshot_sha256 TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	for _, column := range []struct{ name, statement string }{{"subject_code", `ALTER TABLE dossiers ADD COLUMN subject_code TEXT NOT NULL DEFAULT ''`}, {"editor_id", `ALTER TABLE dossiers ADD COLUMN editor_id TEXT NOT NULL DEFAULT ''`}, {"updated_at", `ALTER TABLE dossiers ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`}} {
		if err := s.ensureColumn(ctx, "dossiers", column.name, column.statement); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, dossierQueueIndex); err != nil {
		return fmt.Errorf("迁移队列索引: %w", err)
	}
	items, err := s.List(ctx)
	if err != nil {
		return err
	}
	for i := range items {
		if _, err = s.db.ExecContext(ctx, `UPDATE dossiers SET subject_code=?,editor_id=?,updated_at=? WHERE dossier_id=?`, items[i].SubjectCode, items[i].EditorID, items[i].UpdatedAt.UTC().Format(time.RFC3339Nano), items[i].DossierID); err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err = replaceChildren(ctx, tx, &items[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("回填规范化记录: %w", err)
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, column, statement string) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			rows.Close()
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, statement)
	return err
}

func (s *Store) GetReleasePackage(ctx context.Context, id string) (*domain.ReleasePackage, error) {
	var p domain.ReleasePackage
	var approved string
	err := s.db.QueryRowContext(ctx, `SELECT package_id,manifest_version,manifest,redacted_transcript,consent_snapshot,reviewer_id,approved_at,audit_head_sha256,review_snapshot_sha256,content_sha256 FROM release_packages WHERE dossier_id=?`, id).Scan(&p.PackageID, &p.ManifestVersion, &p.Manifest, &p.RedactedTranscript, &p.ConsentSnapshot, &p.ReviewerID, &approved, &p.AuditHeadSHA256, &p.ReviewSnapshotSHA256, &p.ContentSHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.DossierID = id
	p.ApprovedAt, err = time.Parse(time.RFC3339Nano, approved)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) GetAuditHead(ctx context.Context, id string) (string, error) {
	var head string
	err := s.db.QueryRowContext(ctx, `SELECT sha256 FROM audit_events WHERE dossier_id=? ORDER BY sequence DESC LIMIT 1`, id).Scan(&head)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return head, err
}

func (s *Store) Get(ctx context.Context, id string) (*domain.InterviewDossier, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM dossiers WHERE dossier_id=?`, id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeDossier(payload)
}

func (s *Store) List(ctx context.Context) ([]domain.InterviewDossier, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM dossiers ORDER BY dossier_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.InterviewDossier
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		d, err := decodeDossier(payload)
		if err != nil {
			return nil, err
		}
		result = append(result, *d)
	}
	return result, rows.Err()
}

func (s *Store) Create(ctx context.Context, d *domain.InterviewDossier, requestID, fingerprint string, status int, response []byte) error {
	payload, err := encodeDossier(d)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO dossiers(dossier_id,revision,status,subject_code,editor_id,updated_at,payload) VALUES(?,?,?,?,?,?,?)`, d.DossierID, d.Revision, d.Status, d.SubjectCode, d.EditorID, d.UpdatedAt.UTC().Format(time.RFC3339Nano), payload); err != nil {
		return err
	}
	if err = replaceChildren(ctx, tx, d); err != nil {
		return err
	}
	if requestID != "" {
		if _, err = tx.ExecContext(ctx, `INSERT INTO idempotency(request_id,fingerprint,status,response) VALUES(?,?,?,?)`, requestID, fingerprint, status, response); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Save(ctx context.Context, d *domain.InterviewDossier, previousRevision int64, requestID, fingerprint string, status int, response []byte) error {
	payload, err := encodeDossier(d)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE dossiers SET revision=?,status=?,subject_code=?,editor_id=?,updated_at=?,payload=? WHERE dossier_id=? AND revision=?`, d.Revision, d.Status, d.SubjectCode, d.EditorID, d.UpdatedAt.UTC().Format(time.RFC3339Nano), payload, d.DossierID, previousRevision)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return domain.ErrConflict
	}
	if err = replaceChildren(ctx, tx, d); err != nil {
		return err
	}
	if requestID != "" {
		if _, err = tx.ExecContext(ctx, `INSERT INTO idempotency(request_id,fingerprint,status,response) VALUES(?,?,?,?)`, requestID, fingerprint, status, response); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Replay(ctx context.Context, requestID, fingerprint string) (Replay, error) {
	if requestID == "" {
		return Replay{}, nil
	}
	var stored string
	var status int
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT fingerprint,status,response FROM idempotency WHERE request_id=?`, requestID).Scan(&stored, &status, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return Replay{}, nil
	}
	if err != nil {
		return Replay{}, err
	}
	if stored != fingerprint {
		return Replay{}, domain.Invalid("request_id", "已被不同请求使用")
	}
	return Replay{Status: status, Body: body, Found: true}, nil
}

func (s *Store) VerifyAll(ctx context.Context) error {
	items, err := s.List(ctx)
	if err != nil {
		return err
	}
	for i := range items {
		if err := domain.VerifyAudit(&items[i]); err != nil {
			return fmt.Errorf("档案 %s 审计链损坏: %w", items[i].DossierID, err)
		}
		if err := s.verifyAuditRows(ctx, items[i].DossierID, items[i].Audit); err != nil {
			return fmt.Errorf("档案 %s 持久化审计链损坏: %w", items[i].DossierID, err)
		}
		if err := s.verifyReleaseRows(ctx, &items[i]); err != nil {
			return fmt.Errorf("档案 %s 发布包记录损坏: %w", items[i].DossierID, err)
		}
	}
	return nil
}
