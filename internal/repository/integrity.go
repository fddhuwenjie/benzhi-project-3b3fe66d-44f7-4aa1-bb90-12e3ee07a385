package repository

import (
	"context"
	"fmt"
	"oralarchive/internal/domain"
)

func (s *Store) verifyAuditRows(ctx context.Context, dossierID string, expected []domain.AuditEvent) error {
	rows, err := s.db.QueryContext(ctx, `SELECT sequence,action,actor_id,previous_sha256,sha256 FROM audit_events WHERE dossier_id=? ORDER BY sequence`, dossierID)
	if err != nil {
		return err
	}
	defer rows.Close()
	index := 0
	previous := ""
	for rows.Next() {
		var sequence int
		var action, actor, storedPrevious, digest string
		if err := rows.Scan(&sequence, &action, &actor, &storedPrevious, &digest); err != nil {
			return err
		}
		if sequence != index+1 {
			return fmt.Errorf("审计序号不连续: %d", sequence)
		}
		if storedPrevious != previous {
			return fmt.Errorf("审计前序摘要不连续: %d", sequence)
		}
		if index >= len(expected) {
			return fmt.Errorf("审计表存在额外事件: %d", sequence)
		}
		event := expected[index]
		if event.Sequence != sequence || event.Action != action || event.ActorID != actor || event.PreviousSHA256 != storedPrevious || event.SHA256 != digest {
			return fmt.Errorf("审计表与聚合不一致: %d", sequence)
		}
		previous = digest
		index++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if index != len(expected) {
		return fmt.Errorf("审计表缺少事件: 期望 %d，实际 %d", len(expected), index)
	}
	return nil
}

func (s *Store) verifyReleaseRows(ctx context.Context, d *domain.InterviewDossier) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM release_packages WHERE dossier_id=?`, d.DossierID).Scan(&count); err != nil {
		return err
	}
	if d.Package == nil && count != 0 {
		return fmt.Errorf("未封存档案存在发布包")
	}
	if d.Package != nil && count != 1 {
		return fmt.Errorf("封存档案发布包数量无效")
	}
	return nil
}
