package careplanhttp

import (
	context "context"
	slog "log/slog"

	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
)

// CleanupOrphanedStudentDocumentFiles runs Care Plan's recovery pass over the
// stored objects of child documents (careplan.CleanupOrphanedStudentDocumentFiles).
func (rs *Resource) CleanupOrphanedStudentDocumentFiles(ctx context.Context) (int, error) {
	return careplan.CleanupOrphanedStudentDocumentFiles(ctx, rs.StudentDocumentService, studentDocumentFileRemoval(rs.DocumentStorage), rs.getLogger())
}

// studentDocumentFileRemoval opens the object removal of Care Plan's recovery
// pass over the document storage; nil without a storage.
func studentDocumentFileRemoval(storage StudentDocumentStorage) careplan.StudentDocumentFileRemoval {
	if storage == nil {
		return nil
	}
	return func(kind string, documents careplan.StudentDocuments, logger *slog.Logger) (careplan.StudentDocumentFileRemover, error) {
		files, err := storage(kind, documents, logger)
		if err != nil {
			return nil, err
		}
		return files.Remove, nil
	}
}
