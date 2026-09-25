package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The spontaneous start resolves its activity by name, creating it (and the
// "Spontan" category) on first use. These cases were asserted by the
// api/timetable handler tests over model-typed fakes before #3554.

type spontaneousTemplates struct {
	DataTemplates
	byName  map[string]*activitiesModels.Group
	created []*activitiesModels.Group
}

func (f *spontaneousTemplates) FindByName(_ context.Context, name string) (*activitiesModels.Group, error) {
	return f.byName[name], nil
}

func (f *spontaneousTemplates) Create(_ context.Context, group *activitiesModels.Group) error {
	group.ID = 73
	f.created = append(f.created, group)
	return nil
}

type spontaneousCategories struct {
	existing *activitiesModels.Category
	created  []*activitiesModels.Category
}

func (f *spontaneousCategories) FindByNameIncludingArchivedForShare(context.Context, string) (*activitiesModels.Category, error) {
	return f.existing, nil
}

func (f *spontaneousCategories) Create(_ context.Context, category *activitiesModels.Category) error {
	category.ID = 910
	f.created = append(f.created, category)
	return nil
}

func spontaneousData(templates *spontaneousTemplates, categories *spontaneousCategories) *timetableData {
	return &timetableData{deps: TimetableDataDependencies{Templates: templates, Categories: categories}}
}

func TestResolveSpontaneousActivityReusesActivityByName(t *testing.T) {
	t.Parallel()
	existing := &activitiesModels.Group{Name: "freispiel"}
	existing.ID = 72
	templates := &spontaneousTemplates{byName: map[string]*activitiesModels.Group{"freispiel": existing}}
	categories := &spontaneousCategories{}

	id, err := spontaneousData(templates, categories).ResolveSpontaneousActivity(tenant.WithTenantID(context.Background(), 7401), "freispiel", nil, 321)

	require.NoError(t, err)
	require.NotNil(t, id)
	assert.Equal(t, int64(72), *id)
	assert.Empty(t, templates.created, "an existing activity must not be recreated")
	assert.Empty(t, categories.created)
}

func TestResolveSpontaneousActivityCreatesActivityAndCategoryForNewName(t *testing.T) {
	t.Parallel()
	templates := &spontaneousTemplates{}
	categories := &spontaneousCategories{}

	id, err := spontaneousData(templates, categories).ResolveSpontaneousActivity(tenant.WithTenantID(context.Background(), 7402), "Bastelecke", nil, 322)

	require.NoError(t, err)
	require.NotNil(t, id)
	assert.Equal(t, int64(73), *id)
	require.Len(t, categories.created, 1)
	assert.Equal(t, spontaneousCategoryName, categories.created[0].Name)
	require.Len(t, templates.created, 1)
	group := templates.created[0]
	assert.Equal(t, "Bastelecke", group.Name)
	assert.Equal(t, int64(910), group.CategoryID)
	assert.Equal(t, activitiesModels.GroupTypeActivity, group.Type)
	require.NotNil(t, group.CreatedBy)
	assert.Equal(t, int64(322), *group.CreatedBy)
	assert.Equal(t, int64(7402), group.GetTenantID())
}

func TestResolveSpontaneousActivityRejectsArchivedCategory(t *testing.T) {
	t.Parallel()
	archivedAt := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	templates := &spontaneousTemplates{}
	categories := &spontaneousCategories{existing: &activitiesModels.Category{Name: spontaneousCategoryName, ArchivedAt: &archivedAt}}

	_, err := spontaneousData(templates, categories).ResolveSpontaneousActivity(tenant.WithTenantID(context.Background(), 7403), "Bastelecke", nil, 323)

	require.True(t, errors.Is(err, timetable.ErrSpontaneousCategoryArchived))
	assert.Empty(t, templates.created, "no activity may be created under an archived category")
	assert.Empty(t, categories.created, "the archived category must not be recreated")
}

func TestResolveSpontaneousActivityKeepsRequestedActivity(t *testing.T) {
	t.Parallel()
	requested := int64(75)
	templates := &spontaneousTemplates{}
	categories := &spontaneousCategories{}

	id, err := spontaneousData(templates, categories).ResolveSpontaneousActivity(tenant.WithTenantID(context.Background(), 7404), "egal", &requested, 324)

	require.NoError(t, err)
	assert.Equal(t, &requested, id)
	assert.Empty(t, templates.created)
}
