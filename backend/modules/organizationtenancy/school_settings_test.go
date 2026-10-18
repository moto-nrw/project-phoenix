package organizationtenancy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
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

func (e *settingsEngine) FindSchoolByID(_ context.Context, _ int64, lock string) (organizationtenancy.School, error) {
	e.lock = lock
	return e.stored, nil
}

func (e *settingsEngine) UpdateSchool(_ context.Context, input organizationtenancy.UpdateSchool) (organizationtenancy.School, error) {
	e.updated = &input
	return organizationtenancy.School{}, nil
}

func TestUpdateSchoolSettingsRewritesOnlyTheSettings(t *testing.T) {
	t.Parallel()
	engine := &settingsEngine{stored: organizationtenancy.School{
		ID: 31, OrganizationID: 41, Name: "Grundschule", Slug: "gs", Subdomain: "gs", Active: true,
		Settings: `{"a":1}`, City: "Köln", Email: "info@gs.test",
	}}
	module := organizationtenancy.NewModule(engine)

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

func TestSchoolLogoURLPrefersTheLogoOverTheLoginImage(t *testing.T) {
	t.Parallel()
	for settings, want := range map[string]string{
		"":                "",
		"   ":             "",
		"{not valid json": "",
		`{"other":"x"}`:   "",
		`{"logoUrl":"https://logo","loginImageUrl":"https://login"}`: "https://logo",
		`{"loginImageUrl":"https://login"}`:                          "https://login",
		`{"logoUrl":"  https://logo  "}`:                             "https://logo",
		`{"logoUrl":42,"loginImageUrl":"https://login"}`:             "https://login",
		`{"logoUrl":"","loginImageUrl":"https://login"}`:             "https://login",
	} {
		assert.Equal(t, want, organizationtenancy.SchoolLogoURL(settings), settings)
		assert.Equal(t, want, organizationtenancy.School{Settings: settings}.LogoURL(), settings)
	}
}
