package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// TraegerDocumentStore persists the shared documents of a Träger (#4692).
// Every method runs in the caller's Träger transaction of organizationID
// and names it in its own filter again. The parents' and schools' reads are
// the store's public capability, not part of this port.
type TraegerDocumentStore interface {
	// ListDocuments lists the documents with their active releases and
	// read counts, the newest first.
	ListDocuments(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerDocument, error)
	InsertDocument(ctx context.Context, organizationID, accountID int64, document domain.TraegerDocumentWrite) (organizationtenancy.TraegerDocument, error)
	// DocumentFile reads the file; found is false for a document that is
	// not the Träger's.
	DocumentFile(ctx context.Context, organizationID, documentID int64) (organizationtenancy.TraegerDocumentFile, bool, error)
	// InsertRelease adds an active release; ErrTraegerDocumentNotFound for
	// a document that is not the Träger's, ErrTraegerDocumentAlreadyReleased
	// for an active release of the same audience and
	// ErrTraegerDocumentAudienceInvalid for a school of another Träger.
	InsertRelease(ctx context.Context, organizationID, accountID, documentID int64, input organizationtenancy.TraegerDocumentReleaseInput, groupName string) (organizationtenancy.TraegerDocumentRelease, error)
	// RevokeRelease revokes an active release of the document; revoked is
	// false when there is none.
	RevokeRelease(ctx context.Context, organizationID, accountID, documentID, releaseID int64) (revoked bool, err error)
}
