package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func Digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func ValidSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func Clean(value string) string { return strings.TrimSpace(value) }
