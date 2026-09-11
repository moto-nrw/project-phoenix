package enrollment

import (
	"fmt"
	"strings"
)

const (
	EnrollmentFormLegalDocumentDir          = "public/uploads/enrollment-form-legal-documents"
	EnrollmentFormLegalDocumentUploadPrefix = "/uploads/enrollment-form-legal-documents/"
	EnrollmentFormLegalDocumentPublicPrefix = "/api/public/enrollment-form-legal-documents/"
	EnrollmentLegalDocumentUploadPrefix     = "/uploads/enrollment-legal-documents/"
	EnrollmentLegalDocumentPublicPrefix     = "/api/public/enrollment-legal-documents/"
)

// NormalizeEnrollmentFormLegalDocumentURL returns the canonical stored upload
// URL for tenant-owned enrollment-form legal documents.
func NormalizeEnrollmentFormLegalDocumentURL(rawURL string, tenantID int64) (string, bool) {
	return normalizeLegalDocumentURL(rawURL, tenantID, map[string]string{
		EnrollmentFormLegalDocumentPublicPrefix: EnrollmentFormLegalDocumentUploadPrefix,
	}, EnrollmentFormLegalDocumentUploadPrefix)
}

// NormalizeTenantLegalDocumentURL returns the canonical stored upload URL for
// tenant-owned enrollment legal documents, including tenant-wide settings PDFs
// and form-template PDFs.
func NormalizeTenantLegalDocumentURL(rawURL string, tenantID int64) (string, bool) {
	return normalizeLegalDocumentURL(rawURL, tenantID, map[string]string{
		EnrollmentLegalDocumentPublicPrefix:     EnrollmentLegalDocumentUploadPrefix,
		EnrollmentFormLegalDocumentPublicPrefix: EnrollmentFormLegalDocumentUploadPrefix,
	}, EnrollmentLegalDocumentUploadPrefix, EnrollmentFormLegalDocumentUploadPrefix)
}

func normalizeLegalDocumentURL(rawURL string, tenantID int64, publicPrefixes map[string]string, uploadPrefixes ...string) (string, bool) {
	if tenantID <= 0 {
		return "", false
	}

	storedURL := strings.TrimSpace(rawURL)
	for publicPrefix, uploadPrefix := range publicPrefixes {
		if strings.HasPrefix(storedURL, publicPrefix) {
			storedURL = uploadPrefix + strings.TrimPrefix(storedURL, publicPrefix)
			break
		}
	}

	for _, uploadPrefix := range uploadPrefixes {
		if !strings.HasPrefix(storedURL, uploadPrefix) {
			continue
		}
		filename := strings.TrimPrefix(storedURL, uploadPrefix)
		if filename == "" ||
			strings.Contains(filename, "..") ||
			strings.ContainsAny(filename, `/\`) ||
			!strings.HasPrefix(filename, fmt.Sprintf("%d_", tenantID)) ||
			!strings.HasSuffix(strings.ToLower(filename), ".pdf") {
			return "", false
		}

		return storedURL, true
	}

	return "", false
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
