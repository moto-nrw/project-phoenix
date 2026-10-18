package workforce

import (
	"context"
	"path/filepath"
	"strconv"
)

// staffDocumentsBaseDir is the relative fallback of the staff document files
// when no public directory exists at all.
const staffDocumentsBaseDir = "public/uploads/staff-documents"

// StaffDocumentUploads resolves the public upload directory the staff
// document files live below. The root binds the Delivery Platform's upload
// files.
type StaffDocumentUploads interface {
	PublicDir() (string, error)
}

// StaffDocumentsDir is the per-tenant staff document directory. Save, serve
// and remove all derive it from the same public directory resolution so the
// three operations can never disagree about where the bytes live.
func StaffDocumentsDir(tenantID int64, uploads StaffDocumentUploads) string {
	base := staffDocumentsBaseDir
	if pubDir, err := uploads.PublicDir(); err == nil {
		base = filepath.Join(pubDir, "uploads", "staff-documents")
	}
	return filepath.Join(base, strconv.FormatInt(tenantID, 10))
}

// StaffDocumentFileRecovery removes the stored bytes of staff documents: the
// scheduler's recovery pass, the leased offboarding cleanup and the removal
// of one stored file.
type StaffDocumentFileRecovery interface {
	// CleanupOrphanedStaffDocumentFiles retries file removal independently of
	// document UI traffic. The scheduler calls it in each tenant transaction
	// after the upload grace period has elapsed.
	CleanupOrphanedStaffDocumentFiles(ctx context.Context) (int, error)
	// RunOffboardingDocumentCleanup claims and runs due offboarding cleanups.
	RunOffboardingDocumentCleanup(ctx context.Context) (int, error)
	// WakeOffboardingDocumentCleanup runs the offboarding cleanup after the
	// transaction of ctx commits.
	WakeOffboardingDocumentCleanup(ctx context.Context)
	// RemoveStoredDocument deletes one stored file of the tenant; a missing
	// file is no error.
	RemoveStoredDocument(tenantID int64, storedName string) error
}
