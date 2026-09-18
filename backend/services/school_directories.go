package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
)

// Organisation & Tenancy owns platform.schools (#3253). Every retained
// consumer reads schools through a port it owns; the adapters below bind
// those ports to the public capability.

// findSchool reads one school and reports a missing row as found=false.
func findSchool(ctx context.Context, schools organizationtenancy.Query, id int64) (organizationtenancy.School, bool, error) {
	school, err := schools.FindSchool(ctx, id)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return organizationtenancy.School{}, false, nil
	}
	if err != nil {
		return organizationtenancy.School{}, false, err
	}
	return school, true, nil
}

// enrollmentSchoolDirectory answers the enrollment flows.
type enrollmentSchoolDirectory struct {
	schools organizationtenancy.Query
}

func (d enrollmentSchoolDirectory) FindSchool(ctx context.Context, id int64) (*enrollment.School, error) {
	school, found, err := findSchool(ctx, d.schools, id)
	if err != nil || !found {
		return nil, err
	}
	return &enrollment.School{
		Name: school.Name, Subdomain: school.Subdomain, Settings: school.Settings, Deleted: school.IsDeleted(),
	}, nil
}

// schoolContactDirectory answers the reply address of tenant-bound mail.
type schoolContactDirectory struct {
	schools organizationtenancy.Query
}

func (d schoolContactDirectory) FindSchoolContact(ctx context.Context, tenantID int64) (*emailoutbox.SchoolContact, error) {
	school, found, err := findSchool(ctx, d.schools, tenantID)
	if err != nil || !found {
		return nil, err
	}
	return &emailoutbox.SchoolContact{Name: school.Name, Email: school.Email}, nil
}

// schoolNameDirectory answers the parent-message mail branding.
type schoolNameDirectory struct {
	schools organizationtenancy.Query
}

func (d schoolNameDirectory) FindSchoolName(ctx context.Context, id int64) (string, bool, error) {
	school, found, err := findSchool(ctx, d.schools, id)
	return school.Name, found, err
}

// displaySchoolDirectory answers the Device Fleet display tenant facts.
type displaySchoolDirectory struct {
	schools organizationtenancy.Query
}

func (d displaySchoolDirectory) FindDisplaySchool(ctx context.Context, tenantID int64) (devicefleetCompose.School, bool, error) {
	school, found, err := findSchool(ctx, d.schools, tenantID)
	if err != nil || !found {
		return devicefleetCompose.School{}, found, err
	}
	return devicefleetCompose.School{Name: school.Name, Active: school.Active, Deleted: school.IsDeleted()}, true, nil
}
