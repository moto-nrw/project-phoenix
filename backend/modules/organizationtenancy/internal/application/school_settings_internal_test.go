package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// settingsEngine serves one stored school and records the update.
type settingsEngine struct {
	recordingEngine
	stored  organizationtenancy.School
	lock    string
	updated *organizationtenancy.UpdateSchool
}

func (e *settingsEngine) FindSchoolByID(_ context.Context, _ int64, lock string) (organizationtenancy.School, bool, domain.OperationStats, error) {
	e.lock = lock
	return e.stored, true, domain.OperationStats{}, nil
}

func (e *settingsEngine) UpdateSchool(_ context.Context, input organizationtenancy.UpdateSchool) (organizationtenancy.School, domain.OperationStats, error) {
	e.updated = &input
	return organizationtenancy.School{}, domain.OperationStats{}, nil
}

func TestUpdateSchoolSettingsRewritesOnlyTheSettings(t *testing.T) {
	t.Parallel()
	engine := &settingsEngine{stored: organizationtenancy.School{
		ID: 31, OrganizationID: 41, Name: "Grundschule", Slug: "gs", Subdomain: "gs", Active: true,
		Settings: `{"a":1}`, City: "Köln", Email: "info@gs.test",
	}}
	module := New(engine, ownerTestTransaction{}, func(ports.Observation) {})

	require.NoError(t, module.UpdateSchoolSettings(context.Background(), 31, func(current string) (string, error) {
		assert.Equal(t, `{"a":1}`, current)
		return `{"a":2}`, nil
	}))
	assert.NotEmpty(t, engine.lock, "the school is locked before its settings are read")
	require.NotNil(t, engine.updated)
	assert.Equal(t, organizationtenancy.UpdateSchool{
		ID: 31, OrganizationID: 41, Name: "Grundschule", Slug: "gs", Subdomain: "gs", Active: true,
		Settings: `{"a":2}`, City: "Köln", Email: "info@gs.test",
	}, *engine.updated)

	engine.updated = nil
	refused := errors.New("refused")
	err := module.UpdateSchoolSettings(context.Background(), 31, func(string) (string, error) { return "", refused })
	require.ErrorIs(t, err, refused)
	assert.Nil(t, engine.updated, "a refused update writes nothing")
}
