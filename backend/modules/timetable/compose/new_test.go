package compose

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type observationLog struct {
	mu   sync.Mutex
	seen []Observation
}

func (l *observationLog) record(observation Observation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, observation)
}

func buildModule(t *testing.T, db *bun.DB, observers ...func(Observation)) *timetable.Module {
	t.Helper()
	observe := func(Observation) {}
	if len(observers) > 0 {
		observe = observers[0]
	}
	students := StudentDirectoryFunc(func(context.Context) ([]TargetStudent, error) { return []TargetStudent{}, nil })
	module, err := New(Dependencies{DB: db, Students: students, Observe: observe})
	require.NoError(t, err)
	return module
}

func createCategory(t *testing.T, ctx context.Context, module *timetable.Module, name string) timetable.Category {
	t.Helper()
	category, err := module.CreateCategory(ctx, timetable.CreateCategory{Name: name, Color: "#abc"})
	require.NoError(t, err)
	return category
}

func TestNewRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	_, err := New(Dependencies{})
	require.Error(t, err)
}

func TestModuleRunsCategoryLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)

	created := createCategory(t, ctx, module, "Werken")
	assert.Positive(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "#abc", created.Color)

	found, err := module.FindCategoryByName(ctx, "Werken")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	updated, err := module.UpdateCategory(ctx, timetable.UpdateCategory{
		ID: created.ID, Name: "Holzwerken", Description: "Werkstatt", Color: "#123456",
	})
	require.NoError(t, err)
	assert.Equal(t, "Holzwerken", updated.Name)

	archived, err := module.ArchiveCategory(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, archived.ArchivedAt)
	_, err = module.FindCategoryByName(ctx, "Holzwerken")
	require.ErrorIs(t, err, timetable.ErrCategoryNotFound)
	_, err = module.UpdateCategory(ctx, timetable.UpdateCategory{ID: created.ID, Name: "Nein", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrCategoryArchived)

	restored, err := module.RestoreCategory(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, restored.ArchivedAt)

	listed, err := module.ListCategories(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	require.NotEmpty(t, log.seen)
	assert.Equal(t, "create_category", log.seen[0].Operation)
	assert.EqualValues(t, 1, log.seen[0].Stats.Rows)
	assert.Positive(t, log.seen[0].Stats.StatementDuration)
}

func TestModuleEnforcesActiveNameUniquenessPerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)

	first := createCategory(t, ctx, module, "Musik")
	_, err := module.CreateCategory(ctx, timetable.CreateCategory{Name: "MUSIK"})
	require.ErrorIs(t, err, timetable.ErrCategoryNameExists)
	assert.Equal(t, "category_name_exists", timetable.ErrorCode(err))

	_, err = module.ArchiveCategory(ctx, first.ID)
	require.NoError(t, err)
	second := createCategory(t, ctx, module, "musik")
	assert.NotEqual(t, first.ID, second.ID)
	_, err = module.RestoreCategory(ctx, first.ID)
	require.ErrorIs(t, err, timetable.ErrCategoryNameExists)

	var conflicts int64
	for _, observation := range log.seen {
		conflicts += observation.Stats.DuplicatePreventionConflicts
	}
	assert.EqualValues(t, 2, conflicts)
}

func TestModuleTenantIsolationHidesForeignCategories(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)

	own := createCategory(t, ctx, module, "Eigene Kategorie")
	foreign := createCategory(t, foreignCtx, module, "Fremde Kategorie")
	assert.Equal(t, foreignTenantID, foreign.TenantID)

	_, err := module.FindCategory(foreignCtx, own.ID)
	require.ErrorIs(t, err, timetable.ErrCategoryNotFound)
	_, err = module.UpdateCategory(foreignCtx, timetable.UpdateCategory{ID: own.ID, Name: "Gekapert", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrCategoryNotFound)
	listed, err := module.ListCategories(foreignCtx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, foreign.ID, listed[0].ID)
}

func TestModuleReadsGroupsAndTargetsWithinTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Owner Group")
	class := "2b"
	insertGroupTarget(t, db, testpkg.Tenant(t), group.ID, "klasse", &class)

	found, err := module.FindGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, group.Name, found.Name)
	assert.Nil(t, found.Category, "single-row lookup must not imply cross-table enrichment")

	listed, err := module.ListGroups(ctx, timetable.GroupFilter{IDs: []int64{group.ID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.NotNil(t, listed[0].Category)
	assert.Equal(t, group.CategoryID, listed[0].Category.ID)
	assert.False(t, listed[0].Category.CreatedAt.IsZero())

	targets, err := module.ListGroupTargets(ctx, []int64{group.ID})
	require.NoError(t, err)
	require.Len(t, targets[group.ID], 1)
	assert.Equal(t, class, *targets[group.ID][0].TargetSchoolClass)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_group_targets").Stats.Queries)
}

func TestModuleOwnsGroupLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	category := createCategory(t, ctx, module, "Group lifecycle")

	created, err := module.CreateGroup(ctx, timetable.GroupInput{
		Name: "Owner lifecycle", CategoryID: category.ID, IsOpen: true,
	})
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, timetable.GroupTypeActivity, created.Type)
	assert.Equal(t, timetable.TargetGroupTypeNone, created.TargetGroupType)
	assert.False(t, created.CreatedAt.IsZero())

	found, err := module.FindGroupByName(ctx, " owner LIFECYCLE ")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	updated, err := module.UpdateGroup(ctx, created.ID, timetable.GroupInput{
		Name: "Owner updated", CategoryID: category.ID, MaxParticipants: 12,
	})
	require.NoError(t, err)
	assert.Equal(t, "Owner updated", updated.Name)
	assert.Equal(t, 12, updated.MaxParticipants)

	require.NoError(t, module.DeleteGroup(ctx, created.ID))
	_, err = module.FindGroup(ctx, created.ID)
	require.ErrorIs(t, err, timetable.ErrGroupNotFound)
}

func TestModuleOwnsTemplateFilters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	category := createCategory(t, ctx, module, "Template filters")
	root := createTemplate(t, ctx, module, category.ID, "Root", []int64{101})
	segment := createTemplate(t, ctx, module, category.ID, "Segment", []int64{102})
	archived := createTemplate(t, ctx, module, category.ID, "Archived", []int64{102})
	_, err := module.ArchiveTemplate(ctx, archived.ID)
	require.NoError(t, err)
	_, err = module.CreateGroup(ctx, timetable.GroupInput{
		Name: "Non-template", CategoryID: category.ID, TargetGroupType: timetable.TargetGroupTypeOffering,
		SourceCareOfferingIDs: []int64{102},
	})
	require.NoError(t, err)
	_, err = db.NewUpdate().Table("activities.groups").Set("series_root_id = ?", root.ID).
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("id = ?", segment.ID).Exec(ctx)
	require.NoError(t, err)

	isTemplate := true
	bySource, err := module.ListGroups(ctx, timetable.GroupFilter{
		IsTemplate: &isTemplate, SourceOfferingIDs: []int64{102}, ActiveOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, bySource, 1)
	assert.Equal(t, segment.ID, bySource[0].ID)
	bySources, err := module.ListGroups(ctx, timetable.GroupFilter{
		IsTemplate: &isTemplate, SourceOfferingIDs: []int64{101, 102}, ActiveOnly: true, OrderByID: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{root.ID, segment.ID}, groupIDs(bySources))
	series, err := module.ListGroups(ctx, timetable.GroupFilter{
		IsTemplate: &isTemplate, SeriesForGroupID: &segment.ID, ActiveOnly: true, OrderByID: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{root.ID, segment.ID}, groupIDs(series))
}

func TestModuleOwnsTemplateWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	category := createCategory(t, ctx, module, "Template writes")
	template := createTemplate(t, ctx, module, category.ID, "Before", []int64{201})
	room := testpkg.CreateTestRoom(t, db, "Template write room")

	rows, err := module.UpdateTemplate(ctx, template.ID, validTemplateUpdate(category.ID, room.ID, "After", []int64{202}))
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	require.NoError(t, module.UpdateGroupOfferingSource(ctx, template.ID, timetable.OfferingSourceInput{
		CareOfferingIDs: []int64{203}, GradeLevels: []int{3},
	}))
	updated, err := module.FindGroup(ctx, template.ID)
	require.NoError(t, err)
	assert.Equal(t, "After", updated.Name)
	assert.Equal(t, []int64{203}, updated.SourceCareOfferingIDs)
	assert.Equal(t, []int{3}, updated.SourceGradeLevels)
	rows, err = module.ArchiveTemplate(ctx, template.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
}

func TestTemplateWritesRespectTenantAndOuterRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	category := createCategory(t, ctx, module, "Template isolation")
	template := createTemplate(t, ctx, module, category.ID, "Original", nil)
	room := testpkg.CreateTestRoom(t, db, "Template isolation room")
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCategory := testpkg.CreateTestActivityCategoryForTenant(t, db, foreignTenantID, "Foreign category")
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreignTenantID, "Foreign room")
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)

	rows, err := module.UpdateTemplate(foreignCtx, template.ID, validTemplateUpdate(foreignCategory.ID, foreignRoom.ID, "Foreign", nil))
	require.NoError(t, err)
	assert.Zero(t, rows)
	wantErr := errors.New("abort template update")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, updateErr := module.UpdateTemplate(txCtx, template.ID, validTemplateUpdate(category.ID, room.ID, "Rolled back", nil))
		if updateErr != nil {
			return updateErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	unchanged, err := module.FindGroup(ctx, template.ID)
	require.NoError(t, err)
	assert.Equal(t, "Original", unchanged.Name)

	rows, err = module.UpdateTemplate(ctx, template.ID, validTemplateUpdate(category.ID, room.ID, "Retried", nil))
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	retried, err := module.FindGroup(ctx, template.ID)
	require.NoError(t, err)
	assert.Equal(t, "Retried", retried.Name)
}

func TestTemplateOfferingSourceUpdateRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	category := createCategory(t, ctx, module, "Offering source rollback")
	template := createTemplate(t, ctx, module, category.ID, "Template", []int64{301})
	wantErr := errors.New("abort offering source update")

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if updateErr := module.UpdateGroupOfferingSource(txCtx, template.ID, timetable.OfferingSourceInput{
			CareOfferingIDs: []int64{302}, GradeLevels: []int{2},
		}); updateErr != nil {
			return updateErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	assertTemplateOfferingSource(t, ctx, module, template.ID, []int64{301}, nil)

	require.NoError(t, module.UpdateGroupOfferingSource(ctx, template.ID, timetable.OfferingSourceInput{
		CareOfferingIDs: []int64{302}, GradeLevels: []int{2},
	}))
	assertTemplateOfferingSource(t, ctx, module, template.ID, []int64{302}, []int{2})
}

func TestTemplateArchiveRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	category := createCategory(t, ctx, module, "Archive rollback")
	template := createTemplate(t, ctx, module, category.ID, "Template", nil)
	wantErr := errors.New("abort template archive")

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		rows, archiveErr := module.ArchiveTemplate(txCtx, template.ID)
		require.EqualValues(t, 1, rows)
		if archiveErr != nil {
			return archiveErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	unarchived, err := module.FindGroup(ctx, template.ID)
	require.NoError(t, err)
	assert.Nil(t, unarchived.ArchivedAt)

	rows, err := module.ArchiveTemplate(ctx, template.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	rows, err = module.ArchiveTemplate(ctx, template.ID)
	require.NoError(t, err)
	assert.Zero(t, rows)
}

func assertTemplateOfferingSource(t *testing.T, ctx context.Context, module *timetable.Module, id int64, offeringIDs []int64, grades []int) {
	t.Helper()
	group, err := module.FindGroup(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, offeringIDs, group.SourceCareOfferingIDs)
	assert.Equal(t, grades, group.SourceGradeLevels)
}

func createTemplate(t *testing.T, ctx context.Context, module *timetable.Module, categoryID int64, name string, sourceIDs []int64) timetable.Group {
	t.Helper()
	input := timetable.GroupInput{Name: name, CategoryID: categoryID, IsTemplate: true}
	if len(sourceIDs) > 0 {
		input.TargetGroupType = timetable.TargetGroupTypeOffering
		input.SourceCareOfferingIDs = sourceIDs
	}
	created, err := module.CreateGroup(ctx, input)
	require.NoError(t, err)
	return created
}

func validTemplateUpdate(categoryID, roomID int64, name string, sourceIDs []int64) timetable.TemplateUpdate {
	return timetable.TemplateUpdate{
		Name: name, Type: timetable.GroupTypeActivity, CategoryID: categoryID, RoomID: roomID,
		TargetGroupType: timetable.TargetGroupTypeOffering, SourceCareOfferingIDs: sourceIDs,
	}
}

func groupIDs(groups []timetable.Group) []int64 {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids
}

func TestModuleGroupWritesCannotCrossTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Write isolation")
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCategory := testpkg.CreateTestActivityCategoryForTenant(t, db, foreignTenantID, "Foreign category")
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)

	_, err := module.UpdateGroup(foreignCtx, group.ID, timetable.GroupInput{
		Name: "Foreign overwrite", CategoryID: foreignCategory.ID,
	})
	require.ErrorIs(t, err, timetable.ErrGroupNotFound)
	require.NoError(t, module.DeleteGroup(foreignCtx, group.ID))
	stillPresent, err := module.FindGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, group.Name, stillPresent.Name)
}

func TestReplaceGroupTargetsRollsBackAfterDeleteAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Target rollback")
	class := "2b"
	require.NoError(t, module.ReplaceGroupTargets(ctx, group.ID, []timetable.GroupTargetInput{{
		TargetGroupType: "klasse", TargetSchoolClass: &class,
	}}))

	missingEducationGroupID := int64(9_223_372_036_854_775_000)
	err := module.ReplaceGroupTargets(ctx, group.ID, []timetable.GroupTargetInput{{
		TargetGroupType: "gruppe", EducationGroupID: &missingEducationGroupID,
	}})
	require.Error(t, err)
	targets, findErr := module.ListGroupTargets(ctx, []int64{group.ID})
	require.NoError(t, findErr)
	require.Len(t, targets[group.ID], 1, "failed insert must roll back the authoritative delete")
	assert.Equal(t, class, *targets[group.ID][0].TargetSchoolClass)

	grade := int16(2)
	require.NoError(t, module.ReplaceGroupTargets(ctx, group.ID, []timetable.GroupTargetInput{{
		TargetGroupType: "jahrgang", TargetGradeLevel: &grade,
	}}))
	targets, findErr = module.ListGroupTargets(ctx, []int64{group.ID})
	require.NoError(t, findErr)
	require.Len(t, targets[group.ID], 1)
	assert.Equal(t, grade, *targets[group.ID][0].TargetGradeLevel)
}

func TestModuleGroupReadsHideForeignTenantRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)

	own := testpkg.CreateTestActivityGroup(t, db, "Own Group")
	foreign := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "Foreign Group")
	ownClass := "2b"
	foreignClass := "3a"
	insertGroupTarget(t, db, testpkg.Tenant(t), own.ID, timetable.TargetGroupTypeSchoolClass, &ownClass)
	insertGroupTarget(t, db, foreignTenantID, foreign.ID, timetable.TargetGroupTypeSchoolClass, &foreignClass)
	_, err := module.FindGroup(ctx, foreign.ID)
	require.ErrorIs(t, err, timetable.ErrGroupNotFound)
	listed, err := module.ListGroups(foreignCtx, timetable.GroupFilter{IDs: []int64{own.ID, foreign.ID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, foreign.ID, listed[0].ID)
	targets, err := module.ListGroupTargets(ctx, []int64{own.ID, foreign.ID})
	require.NoError(t, err)
	require.Len(t, targets[own.ID], 1)
	assert.Empty(t, targets[foreign.ID])
}

func TestModuleResolvesTargetStudentsThroughPeopleDirectory(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Target students")
	member := testpkg.CreateTestStudent(t, db, "Target", "Member", " 2B ")
	nonMember := testpkg.CreateTestStudent(t, db, "Target", "Other", "3a")
	students := StudentDirectoryFunc(func(context.Context) ([]TargetStudent, error) {
		return []TargetStudent{
			{ID: member.ID, SchoolClass: member.SchoolClass},
			{ID: nonMember.ID, SchoolClass: nonMember.SchoolClass},
		}, nil
	})
	module, err := New(Dependencies{DB: db, Students: students, Observe: func(Observation) {}})
	require.NoError(t, err)
	class := "2b"
	insertGroupTarget(t, db, testpkg.Tenant(t), group.ID, timetable.TargetGroupTypeSchoolClass, &class)

	studentIDs, err := module.ListTargetStudentIDs(ctx, []int64{group.ID})
	require.NoError(t, err)
	assert.Equal(t, []int64{member.ID}, studentIDs[group.ID])
	assert.NotContains(t, studentIDs[group.ID], nonMember.ID)
}

func insertGroupTarget(t *testing.T, db *bun.DB, tenantID, groupID int64, targetType string, schoolClass *string) {
	t.Helper()
	_, err := db.NewRaw(`INSERT INTO activities.group_targets
		(tenant_id, activity_group_id, target_group_type, target_school_class)
		VALUES (?, ?, ?, ?)`, tenantID, groupID, targetType, schoolClass).
		Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)
}

func observedOperation(observations []Observation, operation string) Observation {
	for _, observation := range observations {
		if observation.Operation == operation {
			return observation
		}
	}
	return Observation{}
}

func TestModuleWritesRollBackWithOuterTransactionAndRetryCleanly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	wantErr := errors.New("abort transaction")
	var categoryID int64

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		categoryID = createCategory(t, txCtx, module, "Rollback").ID
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindCategory(ctx, categoryID)
	require.ErrorIs(t, err, timetable.ErrCategoryNotFound)

	retried := createCategory(t, ctx, module, "Rollback")
	assert.Positive(t, retried.ID)

	var groupID int64
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created, createErr := module.CreateGroup(txCtx, timetable.GroupInput{Name: "Rollback group", CategoryID: retried.ID})
		if createErr != nil {
			return createErr
		}
		groupID = created.ID
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindGroup(ctx, groupID)
	require.ErrorIs(t, err, timetable.ErrGroupNotFound)
	created, err := module.CreateGroup(ctx, timetable.GroupInput{Name: "Rollback group", CategoryID: retried.ID})
	require.NoError(t, err)
	assert.Positive(t, created.ID)
}

func TestCategoryShiftLinksValidateBeforeMutation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	category := createCategory(t, ctx, module, "Dienstplan")
	shiftTypeID := insertShiftType(t, ctx, "Betreuung")

	require.NoError(t, module.SetCategoryShiftTypeLinks(ctx, shiftTypeID, []int64{category.ID, category.ID}))
	linked, err := module.FindCategory(ctx, category.ID)
	require.NoError(t, err)
	require.NotNil(t, linked.ShiftTypeID)
	assert.Equal(t, shiftTypeID, *linked.ShiftTypeID)

	err = module.SetCategoryShiftTypeLinks(ctx, shiftTypeID, []int64{category.ID, 9_223_372_036_854_775_000})
	require.ErrorIs(t, err, timetable.ErrUnknownCategoryIDs)
	stillLinked, findErr := module.FindCategory(ctx, category.ID)
	require.NoError(t, findErr)
	require.NotNil(t, stillLinked.ShiftTypeID, "failed validation must not clear existing links")
}

func insertShiftType(t *testing.T, ctx context.Context, name string) int64 {
	t.Helper()
	var id int64
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		transaction, ok := tenant.TransactionFromContext(txCtx)
		if !ok {
			return errors.New("test transaction missing")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return fmt.Errorf("unexpected test transaction %T", transaction)
		}
		return tx.NewRaw(`
			INSERT INTO schedule.shift_types (tenant_id, name, color, is_active)
			VALUES (?, ?, '#123456', TRUE)
			RETURNING id
		`, testpkg.Tenant(t), name).Scan(txCtx, &id)
	})
	require.NoError(t, err)
	return id
}

func TestModuleRefusesUnscopedPersistence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	unscoped := testpkg.WithPackageTenantRuntime(context.Background())

	_, err := module.CreateCategory(unscoped, timetable.CreateCategory{Name: "Unscoped"})
	require.ErrorContains(t, err, "tenant ID is required")
	_, err = module.ListCategories(unscoped)
	require.ErrorContains(t, err, "tenant is required")
}

func TestCategoryColorFallbackDoesNotChangeStoredValue(t *testing.T) {
	t.Parallel()
	category := timetable.Category{}
	assert.Equal(t, timetable.DefaultCategoryColor, category.ColorOrDefault())
	assert.Empty(t, category.Color)
	assert.WithinDuration(t, time.Time{}, category.CreatedAt, 0)
}

func TestCareExitEnrollmentLifecycleIsReversibleAndIdempotent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "CareExit", "Lifecycle", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "CareExitLifecycle")

	active := insertCareExitEnrollment(t, db, testpkg.Tenant(t), student.ID, group.ID, "2026-08-01", nil)
	futureEnd := "2026-12-01"
	future := insertCareExitEnrollment(t, db, testpkg.Tenant(t), student.ID, group.ID, "2026-09-10", &futureEnd)
	closedEnd := "2026-09-01"
	closed := insertCareExitEnrollment(t, db, testpkg.Tenant(t), student.ID, group.ID, "2026-08-01", &closedEnd)

	require.NoError(t, module.LockStudentEnrollmentsForCareExit(ctx, []int64{student.ID}, "2026-09-10"))
	changes, err := module.EndStudentEnrollmentsForCareExit(ctx, []int64{student.ID}, "2026-09-10")
	require.NoError(t, err)
	require.Len(t, changes.Deleted, 1)
	require.Len(t, changes.Capped, 1)
	assert.Equal(t, future, changes.Deleted[0].ID)
	assert.Equal(t, active, changes.Capped[0].ID)
	assert.Equal(t, "2026-09-10", careExitEnrollmentEnd(t, db, testpkg.Tenant(t), active))
	assert.False(t, careExitEnrollmentExists(t, db, testpkg.Tenant(t), future))
	assert.Equal(t, closedEnd, careExitEnrollmentEnd(t, db, testpkg.Tenant(t), closed))

	removals := careExitRemovals(changes)
	restored, err := module.RestoreStudentEnrollmentsForCareExit(ctx, []int64{student.ID}, nil, removals)
	require.NoError(t, err)
	assert.Equal(t, 2, restored)
	restored, err = module.RestoreStudentEnrollmentsForCareExit(ctx, []int64{student.ID}, nil, removals)
	require.NoError(t, err)
	assert.Zero(t, restored, "a retry must not report already-restored rows as new work")
	assert.Empty(t, careExitEnrollmentEnd(t, db, testpkg.Tenant(t), active))
	assert.Equal(t, futureEnd, careExitEnrollmentEnd(t, db, testpkg.Tenant(t), future))
	assert.Positive(t, observedDuplicateConflicts(log.seen))
}

func TestCareExitEnrollmentWritesRespectTenantAndOuterRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Own", "Student", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "OwnGroup")
	own := insertCareExitEnrollment(t, db, testpkg.Tenant(t), student.ID, group.ID, "2026-08-01", nil)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreignTenantID, "Foreign", "Student", "1b")
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "ForeignGroup")
	foreign := insertCareExitEnrollment(t, db, foreignTenantID, foreignStudent.ID, foreignGroup.ID, "2026-08-01", nil)

	wantErr := errors.New("roll back care-exit mutation")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		changes, endErr := module.EndStudentEnrollmentsForCareExit(txCtx, []int64{student.ID, foreignStudent.ID}, "2026-09-10")
		require.NoError(t, endErr)
		require.Len(t, changes.Capped, 1)
		assert.Equal(t, own, changes.Capped[0].ID)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	assert.Empty(t, careExitEnrollmentEnd(t, db, testpkg.Tenant(t), own))
	assert.Empty(t, careExitEnrollmentEnd(t, db, foreignTenantID, foreign))
}

func insertCareExitEnrollment(t *testing.T, db *bun.DB, tenantID, studentID, groupID int64, validFrom string, validUntil *string) int64 {
	t.Helper()
	var id int64
	err := db.NewRaw(`INSERT INTO activities.student_enrollments
		(tenant_id, student_id, activity_group_id, valid_from, valid_until)
		VALUES (?, ?, ?, ?::date, ?::date) RETURNING id`,
		tenantID, studentID, groupID, validFrom, validUntil).Scan(testpkg.WithPackageTenantRuntime(context.Background()), &id)
	require.NoError(t, err)
	return id
}

func careExitEnrollmentEnd(t *testing.T, db *bun.DB, tenantID, enrollmentID int64) string {
	t.Helper()
	var value string
	require.NoError(t, db.NewRaw(`SELECT COALESCE(valid_until::text, '') FROM activities.student_enrollments WHERE tenant_id = ? AND id = ?`, tenantID, enrollmentID).Scan(context.Background(), &value))
	return value
}

func careExitEnrollmentExists(t *testing.T, db *bun.DB, tenantID, enrollmentID int64) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM activities.student_enrollments WHERE tenant_id = ? AND id = ?)`, tenantID, enrollmentID).Scan(context.Background(), &exists))
	return exists
}

func careExitRemovals(changes timetable.CareExitEnrollmentChanges) []timetable.CareExitEnrollmentRemoval {
	result := make([]timetable.CareExitEnrollmentRemoval, 0, len(changes.Deleted)+len(changes.Capped))
	for _, enrollment := range changes.Deleted {
		result = append(result, timetable.CareExitEnrollmentRemoval{
			CareExitEnrollment: enrollment, WasDeleted: true, PreviousValidUntil: enrollment.ValidUntil,
		})
	}
	for _, enrollment := range changes.Capped {
		result = append(result, timetable.CareExitEnrollmentRemoval{
			CareExitEnrollment: timetable.CareExitEnrollment{
				ID: enrollment.ID, TenantID: enrollment.TenantID, StudentID: enrollment.StudentID,
			},
			PreviousValidUntil: enrollment.PreviousValidUntil,
		})
	}
	return result
}

func observedDuplicateConflicts(observations []Observation) int64 {
	var total int64
	for _, observation := range observations {
		total += observation.Stats.DuplicatePreventionConflicts
	}
	return total
}
