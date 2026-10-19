//go:build testhooks

package api

import (
	"context"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/parentstore"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// ParentAnnouncementFixtures constructs native test values through the actual
// owner store. Declaration values use the native public workflow vocabulary.
type ParentAnnouncementFixtures interface {
	Create(context.Context, *communication.ParentAnnouncementFixture) error
	FindByID(context.Context, int64) (*communication.ParentAnnouncementFixture, error)
	Delete(context.Context, int64) error
	SetPublished(context.Context, int64, *time.Time) error
	ReplaceTargets(context.Context, int64, int64, []*communication.ParentAnnouncementTargetFixture) error
	ReplaceOptions(context.Context, int64, int64, []*communication.ParentAnnouncementOptionFixture) error
	ListOptions(context.Context, int64) ([]*communication.ParentAnnouncementOptionFixture, error)
	InsertDeclarationVersion(context.Context, *parentportal.DeclarationVersion) error
	ListDeclarationSubmissions(context.Context, int64, int64) ([]*parentportal.DeclarationSubmission, error)
}

// NewParentAnnouncementFixtures binds the test construction seam without a
// second persistence implementation or a foreign row import in the root.
func NewParentAnnouncementFixtures(repo parentstore.ParentAnnouncementRepository) ParentAnnouncementFixtures {
	return parentAnnouncementFixtures{parentstore.NewParentAnnouncementFixtures(repo)}
}

type parentAnnouncementFixtures struct {
	parentstore.ParentAnnouncementFixtures
}

func (f parentAnnouncementFixtures) InsertDeclarationVersion(ctx context.Context, value *parentportal.DeclarationVersion) error {
	row := owners.ParentMessagingPointer(value, owners.ParentMessagingToOwnerDeclarationVersion)
	err := f.ParentAnnouncementFixtures.InsertDeclarationVersion(ctx, row)
	if value != nil && row != nil {
		*value = owners.ParentMessagingFromOwnerDeclarationVersion(*row)
	}
	return err
}
func (f parentAnnouncementFixtures) ListDeclarationSubmissions(ctx context.Context, tenantID, id int64) ([]*parentportal.DeclarationSubmission, error) {
	rows, err := f.ParentAnnouncementFixtures.ListDeclarationSubmissions(ctx, tenantID, id)
	return owners.ParentMessagingSlice(rows, func(row *communication.PortalDeclarationSubmission) *parentportal.DeclarationSubmission {
		return owners.ParentMessagingPointer(row, owners.ParentMessagingFromOwnerDeclarationSubmission)
	}), err
}
func ParentDeclarationCanonicalContent(value *parentportal.DeclarationVersion) []byte {
	return parentstore.NewPortalDeclarationPolicy().PortalCanonicalContent(owners.ParentMessagingToOwnerDeclarationVersion(*value))
}
func ParentDeclarationCanonicalRecord(value *parentportal.DeclarationSubmission) []byte {
	return parentstore.NewPortalDeclarationPolicy().PortalCanonicalRecord(owners.ParentMessagingToOwnerDeclarationSubmission(*value))
}
