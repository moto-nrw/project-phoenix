package organizationtenancy

import "context"

// The push „Neues Dokument vom Träger“ of a release for parents (#5941). A
// newly stored release for parents puts one deferred push on Delivery's
// outbox in the Träger transaction of the release; a release for schools, an
// upload without a release and a refused second release of the same audience
// put nothing there. The worker reads the release again when it sends: a
// release revoked or a document deleted in between reaches nobody, and each
// parent gets the push only while the release covers it and no older active
// release already showed it the document. A release after a revocation is a
// new row and so a new push; a retry of the old one is not.

// PushKindTraegerDocumentRelease is the outbox kind of that push.
const PushKindTraegerDocumentRelease = "traeger_document_release"

// PushRelatedTraegerDocumentRelease is the related entity type of that push.
// A release, a contract and a billing document may share a numeric ID; the
// type keeps their pushes apart.
const PushRelatedTraegerDocumentRelease = "traeger_document_release"

// TraegerDocumentReleaseNotice is the deferred push of a release for
// parents. It names no parent: the worker finds them when it sends.
type TraegerDocumentReleaseNotice struct {
	OrganizationID int64 `json:"organization_id"`
	ReleaseID      int64 `json:"release_id"`
	DocumentID     int64 `json:"document_id"`
}

// TraegerDocumentNotices puts the push of a release on Delivery's outbox in
// the caller's Träger transaction; a failure rolls the release back.
type TraegerDocumentNotices interface {
	EnqueueReleaseNotice(ctx context.Context, notice TraegerDocumentReleaseNotice) error
}

// ParentReleaseAudience is whom an active release for parents reaches:
// SchoolID nil for every school of the Träger, GroupID a group of that
// school.
type ParentReleaseAudience struct {
	ReleaseID  int64
	DocumentID int64
	SchoolID   *int64
	GroupID    *int64
}

// ParentTraegerDocumentNotices answers the worker's checks for the push of a
// release, in the caller's Träger transaction of organizationID. Neither
// records a read.
type ParentTraegerDocumentNotices interface {
	// ActiveParentRelease reads an active release for parents of an undeleted
	// document; found is false for a revoked release, a deleted document, a
	// release for schools or one of another Träger.
	ActiveParentRelease(ctx context.Context, organizationID, releaseID int64) (audience ParentReleaseAudience, found bool, err error)
	// ReleaseIsNewForParent is true while the active release covers the
	// parent and no older active release for parents of the same document
	// covers it too.
	ReleaseIsNewForParent(ctx context.Context, organizationID, releaseID int64, parent TraegerDocumentParent) (bool, error)
}
