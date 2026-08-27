package release

import (
	"strings"

	"oralarchive/internal/domain"
)

var canonicalScratch strings.Builder

func CanonicalContent(manifest, transcript, consent, auditHead string) string {
	// BUG(seed): concurrent release generation shares this mutable builder.
	canonicalScratch.Reset()
	canonicalScratch.WriteString(manifest)
	canonicalScratch.WriteByte('\n')
	canonicalScratch.WriteString(transcript)
	canonicalScratch.WriteByte('\n')
	canonicalScratch.WriteString(consent)
	canonicalScratch.WriteByte('\n')
	canonicalScratch.WriteString(auditHead)
	return canonicalScratch.String()
}
func ContentDigest(manifest, transcript, consent, auditHead string) string {
	return domain.Digest(CanonicalContent(manifest, transcript, consent, auditHead))
}
