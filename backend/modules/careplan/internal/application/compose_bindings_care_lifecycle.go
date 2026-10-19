package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// studentDocuments joins the authorized document application with the file
// bookkeeping File Storage's coordinator settles.
type StudentDocumentsBinding struct {
	*StudentDocuments
	DocumentFileRecords
}

// documentFileRecords binds File Storage's coordinator store to Care Plan's
// document records. The coordinator acts after its own authorization and
// needs no rule of the Dokumente tab, and each record command already commits
// its own tenant transaction.
type DocumentFileRecords struct{ Records careplan.Capability }

func (r DocumentFileRecords) MarkFileDeleted(ctx context.Context, documentID int64) error {
	return r.Records.MarkCareDocumentFileDeleted(ctx, documentID)
}
func (r DocumentFileRecords) MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	return r.Records.CompleteCareDocumentCleanupByFilename(ctx, storedName)
}
func (r DocumentFileRecords) ActivateQueuedCleanup(ctx context.Context, storedName string) error {
	return r.Records.ActivateCareDocumentCleanup(ctx, storedName)
}
