package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
)

type schoolSettingsStore struct {
	schools organizationtenancy.Capability
}

func NewSchoolSettingsStore(schools organizationtenancy.Capability) settingsCompose.SchoolSettingsStore {
	if schools == nil {
		return nil
	}
	return schoolSettingsStore{schools: schools}
}

func (s schoolSettingsStore) FindSettings(ctx context.Context, schoolID int64) (string, error) {
	school, err := s.schools.FindSchool(ctx, schoolID)
	if err != nil {
		return "", err
	}
	return school.Settings, nil
}

func (s schoolSettingsStore) UpdateSettings(ctx context.Context, schoolID int64, update func(string) (string, error)) error {
	school, err := s.schools.FindSchoolForMutation(ctx, schoolID)
	if err != nil {
		return err
	}
	settings, err := update(school.Settings)
	if err != nil {
		return err
	}
	_, err = s.schools.UpdateSchool(ctx, organizationtenancy.UpdateSchool{
		ID: school.ID, OrganizationID: school.OrganizationID, Name: school.Name, Slug: school.Slug,
		Subdomain: school.Subdomain, Active: school.Active, Hidden: school.Hidden, Settings: settings,
		Address: school.Address, City: school.City, Zip: school.Zip, Phone: school.Phone,
		Email: school.Email,
	})
	return err
}
