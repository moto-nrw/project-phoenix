package owners

import (
	"context"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"

	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// EnrollmentSchoolDirectory answers the enrollment flows.
type EnrollmentSchoolDirectory struct {
	Schools organizationtenancy.Query
}

func (d EnrollmentSchoolDirectory) FindSchool(ctx context.Context, id int64) (*enrollmentOwner.School, error) {
	school, found, err := LookupSchool(ctx, d.Schools, id)
	if err != nil || !found {
		return nil, err
	}
	return &enrollmentOwner.School{
		Name: school.Name, Subdomain: school.Subdomain, Settings: school.Settings, Deleted: school.IsDeleted(),
	}, nil
}

// SchoolContactDirectory answers the reply address of tenant-bound mail.
type SchoolContactDirectory struct {
	Schools organizationtenancy.Query
}

func (d SchoolContactDirectory) FindSchoolContact(ctx context.Context, tenantID int64) (*deliveryCompose.SchoolContact, error) {
	school, found, err := LookupSchool(ctx, d.Schools, tenantID)
	if err != nil || !found {
		return nil, err
	}
	return &deliveryCompose.SchoolContact{Name: school.Name, Email: school.Email}, nil
}

// schoolNameDirectory answers the parent-message mail branding.
type SchoolNameDirectory struct {
	Schools organizationtenancy.Query
}

func (d SchoolNameDirectory) FindSchoolName(ctx context.Context, id int64) (string, bool, error) {
	school, found, err := LookupSchool(ctx, d.Schools, id)
	return school.Name, found, err
}

// displaySchoolDirectory answers the Device Fleet display tenant facts.
type displaySchoolDirectory struct {
	schools organizationtenancy.Query
}

func (d displaySchoolDirectory) FindDisplaySchool(ctx context.Context, tenantID int64) (devicefleetCompose.School, bool, error) {
	school, found, err := LookupSchool(ctx, d.schools, tenantID)
	if err != nil || !found {
		return devicefleetCompose.School{}, found, err
	}
	return devicefleetCompose.School{Name: school.Name, Active: school.Active, Deleted: school.IsDeleted()}, true, nil
}
