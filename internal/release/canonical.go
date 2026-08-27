package release

import "oralarchive/internal/domain"

func CanonicalContent(manifest, transcript, consent, auditHead string) string {
	return manifest + "\n" + transcript + "\n" + consent + "\n" + auditHead
}
func ContentDigest(manifest, transcript, consent, auditHead string) string {
	return domain.Digest(CanonicalContent(manifest, transcript, consent, auditHead))
}
