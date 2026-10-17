//go:build testhooks

package api

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	peopleVocabulary "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// ParentAnnouncementFixtures constructs native test values through the actual
// owner store. Declaration methods retain the workflow's existing vocabulary;
// this fixture cutover does not remove its production DTO dependency.
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
func NewParentAnnouncementFixtures(repo peopleVocabulary.ParentAnnouncementRepository) ParentAnnouncementFixtures {
	return parentstore.NewParentAnnouncementFixtures(repo)
}
