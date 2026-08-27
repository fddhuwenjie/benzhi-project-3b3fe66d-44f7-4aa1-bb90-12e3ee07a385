package domain

import (
	"sort"
	"strings"
	"time"
)

func ValidateAuthorization(subject, audioRef, audioHash, interviewedAt, evidence string, uses []string) error {
	if Clean(subject) == "" {
		return Invalid("subject_code", "不能为空")
	}
	if Clean(audioRef) == "" {
		return Invalid("audio_ref", "不能为空")
	}
	if !ValidSHA256(audioHash) {
		return Invalid("audio_sha256", "必须是 SHA-256 摘要")
	}
	if Clean(interviewedAt) == "" {
		return Invalid("interviewed_at", "不能为空")
	}
	if _, err := time.Parse("2006-01-02", interviewedAt); err != nil {
		return Invalid("interviewed_at", "必须为 YYYY-MM-DD 日期")
	}
	if len(uses) == 0 {
		return Invalid("allowed_uses", "至少登记一种授权用途")
	}
	seen := map[string]bool{}
	for _, use := range uses {
		value := strings.TrimSpace(use)
		if value == "" {
			return Invalid("allowed_uses", "用途不能包含空值")
		}
		if seen[value] {
			return Invalid("allowed_uses", "用途不能重复")
		}
		seen[value] = true
	}
	if !ValidSHA256(evidence) {
		return Invalid("consent_evidence_digest", "必须是 SHA-256 摘要")
	}
	return nil
}

func NormalizeUses(uses []string) ([]string, error) {
	seen := map[string]bool{}
	result := make([]string, 0, len(uses))
	for _, raw := range uses {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		return nil, Invalid("allowed_uses", "至少登记一种非空授权用途")
	}
	sort.Strings(result)
	return result, nil
}

func ValidateEmbargo(interviewedAt, embargoUntil string) error {
	if embargoUntil == "" {
		return nil
	}
	interviewed, err := time.Parse("2006-01-02", interviewedAt)
	if err != nil {
		return Invalid("interviewed_at", "必须为 YYYY-MM-DD 日期")
	}
	embargo, err := time.Parse("2006-01-02", embargoUntil)
	if err != nil {
		return Invalid("embargo_until", "必须为 YYYY-MM-DD 日期")
	}
	if embargo.Before(interviewed) {
		return Invalid("embargo_until", "不得早于访谈日期")
	}
	return nil
}
