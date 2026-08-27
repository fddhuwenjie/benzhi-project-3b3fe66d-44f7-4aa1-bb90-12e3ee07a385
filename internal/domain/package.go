package domain

func PackageReadable(pkg *ReleasePackage) bool {
	return pkg != nil && pkg.PackageID != "" && pkg.ManifestVersion != "" && ValidSHA256(pkg.ContentSHA256)
}
func RedactedOnly(d *InterviewDossier) bool {
	if d.Package == nil {
		return false
	}
	for _, s := range d.Package.RedactedTranscript {
		if s == 0 {
			return false
		}
	}
	return true
}
