package owners

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
	return s.schools.UpdateSchoolSettings(ctx, schoolID, update)
}
