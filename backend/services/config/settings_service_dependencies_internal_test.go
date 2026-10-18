package config

import (
	"context"
	"log/slog"
	"testing"
	"time"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The construction tests call no collaborator, so the stubs embed their
// interfaces unset.
type (
	stubValueRepository struct{ config.SettingValueRepository }
	stubAuditRepository struct{ config.SettingAuditRepository }
	stubSchoolStore     struct{ SchoolSettingsStore }
	stubRuntime         struct{ Runtime }
)

func completeSettingsDependencies() SettingsServiceDependencies {
	return SettingsServiceDependencies{
		Values: stubValueRepository{}, Audit: stubAuditRepository{}, Schools: stubSchoolStore{}, Runtime: stubRuntime{},
		Logger: slog.Default(), HomeLayouts: &HomeLayoutService{},
		ObserveLookup:         func(string, string, string, time.Duration) {},
		ClassRestrictionGuard: func(context.Context) (bool, error) { return true, nil },
		GradeRestrictionGuard: func(context.Context) (bool, error) { return false, nil },
		GradeCapGuard:         func(context.Context) (int, error) { return 4, nil },
	}
}

func TestNewSettingsServiceWithDependenciesBindsEveryDependencyAtConstruction(t *testing.T) {
	t.Parallel()
	built, err := NewSettingsServiceWithDependencies(completeSettingsDependencies())
	require.NoError(t, err)
	service := built.(*settingsService)
	assert.NotNil(t, service.runtime)
	assert.NotNil(t, service.observeLookup)
	assert.NotNil(t, service.homeLayouts)
	assert.Same(t, config.DefaultRegistry(), service.registry, "a missing registry defaults to the global one")
	restricted, err := service.classRestrictionGuard(context.Background())
	require.NoError(t, err)
	assert.True(t, restricted)
	restricted, err = service.gradeRestrictionGuard(context.Background())
	require.NoError(t, err)
	assert.False(t, restricted)
	highest, err := service.gradeCapGuard(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, highest)
}

func TestNewSettingsServiceWithDependenciesRejectsAMissingDependency(t *testing.T) {
	t.Parallel()
	for name, drop := range map[string]func(*SettingsServiceDependencies){
		"values":            func(d *SettingsServiceDependencies) { d.Values = nil },
		"audit":             func(d *SettingsServiceDependencies) { d.Audit = nil },
		"schools":           func(d *SettingsServiceDependencies) { d.Schools = nil },
		"runtime":           func(d *SettingsServiceDependencies) { d.Runtime = nil },
		"logger":            func(d *SettingsServiceDependencies) { d.Logger = nil },
		"home layouts":      func(d *SettingsServiceDependencies) { d.HomeLayouts = nil },
		"lookup observer":   func(d *SettingsServiceDependencies) { d.ObserveLookup = nil },
		"class restriction": func(d *SettingsServiceDependencies) { d.ClassRestrictionGuard = nil },
		"grade restriction": func(d *SettingsServiceDependencies) { d.GradeRestrictionGuard = nil },
		"grade cap":         func(d *SettingsServiceDependencies) { d.GradeCapGuard = nil },
	} {
		dependencies := completeSettingsDependencies()
		drop(&dependencies)
		_, err := NewSettingsServiceWithDependencies(dependencies)
		assert.Error(t, err, name)
	}
}
