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
	if report, ok := s.cachedVerification(id); ok {
		return report, nil
	}
	persisted, err := s.store.GetReleasePackage(ctx, id)
	if err != nil {
		return release.VerificationReport{}, err
	}
	auditHead, err := s.store.GetAuditHead(ctx, id)
	if err != nil {
		return release.VerificationReport{}, err
	}
	report := release.VerifyDossierRecords(d, persisted, auditHead)
	if report.Valid {
		s.cacheVerification(id, report)
	}
	return report, nil
}

func (s *Service) cachedVerification(id string) (release.VerificationReport, bool) {
	s.verificationMu.RLock()
	report, ok := s.verificationResults[id]
	s.verificationMu.RUnlock()
	if !ok {
		return release.VerificationReport{}, false
	}
	return cloneVerificationReport(report), true
}

func (s *Service) cacheVerification(id string, report release.VerificationReport) {
	s.verificationMu.Lock()
	s.verificationResults[id] = cloneVerificationReport(report)
	s.verificationMu.Unlock()
}

func cloneVerificationReport(report release.VerificationReport) release.VerificationReport {
	report.Components = append([]release.ComponentResult(nil), report.Components...)
	report.FailedComponents = append([]string(nil), report.FailedComponents...)
	return report
}
