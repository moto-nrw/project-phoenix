package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// enqueueReleaseNotice puts the push of a newly stored release for parents
// on the outbox in the release's Träger transaction (#5941). A release for
// schools announces nothing to parents; the worker finds the parents of a
// release for parents when it sends.
func (d *TraegerDocuments) enqueueReleaseNotice(ctx context.Context, organizationID, documentID int64, release organizationtenancy.TraegerDocumentRelease) error {
	if release.Audience != organizationtenancy.TraegerDocumentForParents {
		return nil
	}
	err := d.ports.Notices.EnqueueReleaseNotice(ctx, organizationtenancy.TraegerDocumentReleaseNotice{
		OrganizationID: organizationID, ReleaseID: release.ID, DocumentID: documentID,
	})
	if err != nil {
		return fmt.Errorf("organization tenancy: enqueue release push: %w", err)
	}
	return nil
}
