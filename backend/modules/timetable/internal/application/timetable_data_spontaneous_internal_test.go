package application

import (
	"context"
	"errors"
	"testing"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The spontaneous start resolves its activity by name, creating it (and the
// "Spontan" category) on first use. These cases were asserted by the
// api/timetable handler tests over model-typed fakes before #3554.

type spontaneousTemplates struct {
	ports.DataTemplates
	byName  map[string]*activityContract.Group
	created []*activityContract.Group
}

func (f *spontaneousTemplates) FindGroupByName(_ context.Context, name string) (timetable.Group, error) {
	group := f.byName[name]
	if group == nil {
		return timetable.Group{}, timetable.ErrGroupNotFound
	}
	return CatalogGroup(group), nil
}

// CreateGroup stores the group in the caller's tenant, as the owner does.
func (f *spontaneousTemplates) CreateGroup(ctx context.Context, input timetable.GroupInput) (timetable.Group, error) {
	created := legacyGroup(timetable.Group{
		ID: 73, TenantID: tenant.FromContext(ctx), Name: input.Name, CategoryID: input.CategoryID,
		MaxParticipants: input.MaxParticipants, IsOpen: input.IsOpen, CreatedBy: input.CreatedBy,
		Type: input.Type, IsTemplate: input.IsTemplate,
	})
	f.created = append(f.created, created)
	return CatalogGroup(created), nil
}

type spontaneousCategories struct {
	existing *activityContract.Category
	created  []timetable.CreateCategory
}

func (f *spontaneousCategories) FindCategoryByNameForAssignment(context.Context, string) (timetable.Category, error) {
	if f.existing == nil {
		return timetable.Category{}, timetable.ErrCategoryNotFound
	}
	return timetable.Category{ID: f.existing.ID, Name: f.existing.Name, ArchivedAt: f.existing.ArchivedAt}, nil
}

func (f *spontaneousCategories) CreateCategory(_ context.Context, input timetable.CreateCategory) (timetable.Category, error) {
	f.created = append(f.created, input)
	return timetable.Category{ID: 910, Name: input.Name, Description: input.Description, Color: input.Color}, nil
}

func spontaneousData(templates *spontaneousTemplates, categories *spontaneousCategories) *timetableData {
	return &timetableData{deps: ports.TimetableDataDependencies{RunInTx: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }, Templates: templates, Categories: categories}}
}

func TestResolveSpontaneousActivityReusesActivityByName(t *testing.T) {
	t.Parallel()
	existing := &activityContract.Group{Name: "freispiel"}
	existing.ID = 72
	templates := &spontaneousTemplates{byName: map[string]*activityContract.Group{"freispiel": existing}}
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
	assert.Equal(t, activityContract.GroupTypeActivity, group.Type)
	require.NotNil(t, group.CreatedBy)
	assert.Equal(t, int64(322), *group.CreatedBy)
	assert.Equal(t, int64(7402), group.GetTenantID())
}

func TestResolveSpontaneousActivityRejectsArchivedCategory(t *testing.T) {
	t.Parallel()
	archivedAt := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	templates := &spontaneousTemplates{}
	categories := &spontaneousCategories{existing: &activityContract.Category{Name: spontaneousCategoryName, ArchivedAt: &archivedAt}}

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
