package application

import (
	"context"
	"oralarchive/internal/domain"
	"oralarchive/internal/release"
)

func (s *Service) VerifyRelease(ctx context.Context, id string) (release.VerificationReport, error) {
	d, err := s.store.Get(ctx, id)
	if err != nil {
		return release.VerificationReport{}, err
	}
	if d.Status != domain.StatusSealed || d.Package == nil {
		return release.VerificationReport{}, domain.ErrInvalidState
	}
	persisted, err := s.store.GetReleasePackage(ctx, id)
	if err != nil {
		return release.VerificationReport{}, err
	}
	auditHead, err := s.store.GetAuditHead(ctx, id)
	if err != nil {
		return release.VerificationReport{}, err
	}
	return release.VerifyDossierRecords(d, persisted, auditHead), nil
}
