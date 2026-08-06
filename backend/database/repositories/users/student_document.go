package users

import (
	"github.com/moto-nrw/project-phoenix/database/repositories/documents"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// StudentDocumentRepository implements users.StudentDocumentRepository (#777).
//
// The whole implementation is the generic document repository plus a table
// description. Everything that used to be copied per domain — soft delete,
// tenant filtering, the FOR UPDATE SKIP LOCKED cleanup scan — lives once in
// database/repositories/documents.
type StudentDocumentRepository struct {
	*documents.Repository[*users.StudentDocument, *users.StudentDocumentFileCleanup]
}

// NewStudentDocumentRepository creates the student document metadata
// repository.
//
// OwnerTable is intentionally left empty: children are hard-deleted, so there
// is no "owner is soft-deleted" sweep to run. Their documents cascade away
// and the stored bytes are reclaimed through cleanup intents queued before
// the delete, which survive the cascade because they carry no student FK.
func NewStudentDocumentRepository(db *bun.DB) users.StudentDocumentRepository {
	return &StudentDocumentRepository{
		Repository: documents.NewRepository[*users.StudentDocument, *users.StudentDocumentFileCleanup](db, documents.Config{
			Table:        "users.student_documents",
			Alias:        "student_document",
			OwnerColumn:  "student_id",
			CleanupTable: "users.student_document_file_cleanup",
			CleanupAlias: "student_document_file_cleanup",
		}),
	}
}
