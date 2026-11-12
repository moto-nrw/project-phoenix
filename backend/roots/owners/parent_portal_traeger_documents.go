package owners

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	organizationTenancyCompose "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// parentSharedDocuments binds the documents a Träger released for parents
// (#4692) to Organisation & Tenancy, which reads them in the caller's Träger
// transaction and records every opening.
type parentSharedDocuments struct {
	documents organizationtenancy.ParentTraegerDocuments
}

// NewParentSharedDocuments binds the inbox's released documents to
// Organisation & Tenancy.
func NewParentSharedDocuments() parentportal.SharedDocumentsPort {
	return parentSharedDocuments{documents: organizationTenancyCompose.NewSharedTraegerDocuments()}
}

func (d parentSharedDocuments) ListSharedDocuments(ctx context.Context, organizationID int64, reader parentportal.SharedDocumentReader) ([]parentportal.SharedDocument, error) {
	found, err := d.documents.ParentDocuments(ctx, organizationID, organizationtenancy.TraegerDocumentParent(reader))
	if err != nil {
		return nil, err
	}
	documents := make([]parentportal.SharedDocument, len(found))
	for i, document := range found {
		documents[i] = parentportal.SharedDocument{
			ID: document.ID, OrganizationID: organizationID, Title: document.Title, FileName: document.FileName,
			ContentType: document.ContentType, SizeBytes: document.SizeBytes, ReleasedAt: document.ReleasedAt,
		}
	}
	return documents, nil
}

func (d parentSharedDocuments) SharedDocumentFile(ctx context.Context, organizationID, documentID int64, reader parentportal.SharedDocumentReader) (parentportal.BillingDocumentFile, error) {
	file, err := d.documents.OpenParentDocument(ctx, organizationID, documentID, organizationtenancy.TraegerDocumentParent(reader))
	if errors.Is(err, organizationtenancy.ErrTraegerDocumentNotFound) {
		return parentportal.BillingDocumentFile{}, fmt.Errorf("%w: %w", parentportal.ErrSharedDocumentNotFound, err)
	}
	if err != nil {
		return parentportal.BillingDocumentFile{}, err
	}
	return parentportal.BillingDocumentFile{Filename: file.FileName, ContentType: file.ContentType, Data: file.Data}, nil
}
