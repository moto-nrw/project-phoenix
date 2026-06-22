package enrollment

import (
	"fmt"
	"strings"
)

const (
	EnrollmentFormLegalDocumentDir          = "public/uploads/enrollment-form-legal-documents"
	EnrollmentFormLegalDocumentUploadPrefix = "/uploads/enrollment-form-legal-documents/"
	EnrollmentFormLegalDocumentPublicPrefix = "/api/public/enrollment-form-legal-documents/"
)

// NormalizeEnrollmentFormLegalDocumentURL returns the canonical stored upload
// URL for tenant-owned enrollment-form legal documents.
func NormalizeEnrollmentFormLegalDocumentURL(rawURL string, tenantID int64) (string, bool) {
	if tenantID <= 0 {
		return "", false
	}

	storedURL := strings.TrimSpace(rawURL)
	if strings.HasPrefix(storedURL, EnrollmentFormLegalDocumentPublicPrefix) {
		storedURL = EnrollmentFormLegalDocumentUploadPrefix + strings.TrimPrefix(storedURL, EnrollmentFormLegalDocumentPublicPrefix)
	}

	if !strings.HasPrefix(storedURL, EnrollmentFormLegalDocumentUploadPrefix) {
		return "", false
	}
	filename := strings.TrimPrefix(storedURL, EnrollmentFormLegalDocumentUploadPrefix)
	if filename == "" ||
		strings.Contains(filename, "..") ||
		strings.ContainsAny(filename, `/\`) ||
		!strings.HasPrefix(filename, fmt.Sprintf("%d_", tenantID)) ||
		!strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		return "", false
	}

	return storedURL, true
}

func EnrollmentFormLegalDocumentBelongsToTenant(storedURL string, tenantID int64) bool {
	_, ok := NormalizeEnrollmentFormLegalDocumentURL(storedURL, tenantID)
	return ok
}

func PublicEnrollmentFormLegalDocumentURL(storedURL string) string {
	if strings.HasPrefix(storedURL, EnrollmentFormLegalDocumentUploadPrefix) {
		return EnrollmentFormLegalDocumentPublicPrefix + strings.TrimPrefix(storedURL, EnrollmentFormLegalDocumentUploadPrefix)
	}
	return storedURL
}
