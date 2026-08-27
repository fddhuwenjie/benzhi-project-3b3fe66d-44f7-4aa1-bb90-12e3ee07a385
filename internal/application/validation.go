package application

import (
	"oralarchive/internal/domain"
	"strings"
)

func ValidateRequestID(id string) error {
	if strings.TrimSpace(id) == "" {
		return domain.Invalid("request_id", "不能为空")
	}
	if len(id) > 128 {
		return domain.Invalid("request_id", "长度不能超过 128")
	}
	return nil
}
func ValidateExpectedRevision(revision int64) error {
	if revision < 1 {
		return domain.Invalid("expected_revision", "必须为正整数")
	}
	return nil
}
