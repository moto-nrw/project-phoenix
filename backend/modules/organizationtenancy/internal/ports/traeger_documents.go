package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// TraegerDocumentStore persists the shared documents of a Träger (#4692).
// The Träger methods run in the caller's Träger transaction of
// organizationID and name it in their own filter again; the school methods
// run in the tenant transaction of schoolID and reach the documents only
// through the database functions that answer for that school.
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

	// ParentDocuments lists the documents an active parent release covers
	// for the parent's schools and groups.
	ParentDocuments(ctx context.Context, organizationID int64, parent organizationtenancy.TraegerDocumentParent) ([]organizationtenancy.SharedDocument, error)
	// OpenParentDocument reads the file of such a document and records the
	// read in the same transaction; found is false for any other.
	OpenParentDocument(ctx context.Context, organizationID, documentID int64, parent organizationtenancy.TraegerDocumentParent) (organizationtenancy.TraegerDocumentFile, bool, error)

	// SchoolDocuments lists the documents an active release covers for the
	// school of the tenant transaction.
	SchoolDocuments(ctx context.Context, schoolID int64) ([]organizationtenancy.SharedDocument, error)
	// OpenSchoolDocument reads the file of such a document and records the
	// read; found is false for any other.
	OpenSchoolDocument(ctx context.Context, schoolID, accountID, documentID int64) (organizationtenancy.TraegerDocumentFile, bool, error)
}
