package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// DemoSchoolQuery is the Organisation & Tenancy read the demo access needs.
type DemoSchoolQuery interface {
	FindSchoolBySlug(ctx context.Context, slug string) (organizationtenancy.School, error)
}

// NewDemoAccess composes the demo access of the public demo (#3462) over the
// composed Identity & Access module and the school owner's query. Only the
// demo environment's root calls it; nothing retains the result here.
func NewDemoAccess(db *bun.DB, sessions *identityaccess.Module, schools DemoSchoolQuery) (*identityaccess.DemoAccess, error) {
	if sessions == nil || schools == nil {
		return nil, errors.New("demo access composition: the identity module and the school query are required")
	}
	return identityaccessCompose.NewDemoAccess(identityaccessCompose.DemoAccessDependencies{
		DB: db, Sessions: sessions, Schools: demoSchoolDirectory{schools: schools},
		NewToken: authjwt.NewOpaqueCapabilityToken, Fingerprint: authjwt.OpaqueCapabilityFingerprint,
	})
}

type demoSchoolDirectory struct{ schools DemoSchoolQuery }

func (d demoSchoolDirectory) FindDemoSchool(ctx context.Context, slug string) (int64, bool, error) {
	school, err := d.schools.FindSchoolBySlug(ctx, slug)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !school.Active || school.IsDeleted() {
		return 0, false, nil
	}
	return school.ID, true, nil
}
