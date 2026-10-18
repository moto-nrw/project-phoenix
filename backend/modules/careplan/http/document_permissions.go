package careplanhttp

import "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

// StudentDocumentPermissions are the permissions guarding a child's
// documents per category: health documents need student_documents:health,
// legal documents student_documents:legal, and every other category the
// general users:update.
func StudentDocumentPermissions() (health, legal, fallback string) {
	return permissions.StudentDocumentsHealth, permissions.StudentDocumentsLegal, permissions.UsersUpdate
}
