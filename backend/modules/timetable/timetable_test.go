package timetable_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingEngine struct {
	create     timetable.CreateCategory
	update     timetable.UpdateCategory
	calls      int
	rejections []string
}

func (e *recordingEngine) FindCategory(context.Context, int64) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) FindCategoryForAssignment(context.Context, int64) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) FindCategoryByName(context.Context, string) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) ListCategories(context.Context) ([]timetable.Category, error) {
	e.calls++
	return []timetable.Category{}, nil
}

func (e *recordingEngine) CountCategoryUsage(context.Context) (map[int64]int, error) {
	e.calls++
	return map[int64]int{}, nil
}

func (e *recordingEngine) CreateCategory(_ context.Context, input timetable.CreateCategory) (timetable.Category, error) {
	e.calls++
	e.create = input
	return timetable.Category{Name: input.Name, Description: input.Description, Color: input.Color}, nil
}

func (e *recordingEngine) UpdateCategory(_ context.Context, input timetable.UpdateCategory) (timetable.Category, error) {
	e.calls++
	e.update = input
	return timetable.Category{ID: input.ID, Name: input.Name, Description: input.Description, Color: input.Color}, nil
}

func (e *recordingEngine) ArchiveCategory(context.Context, int64) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) RestoreCategory(context.Context, int64) (timetable.Category, error) {
	e.calls++
	return timetable.Category{}, nil
}

func (e *recordingEngine) SetCategoryShiftTypeLinks(context.Context, int64, []int64) error {
	e.calls++
	return nil
}

func (e *recordingEngine) LockStudentEnrollmentsForCareExit(context.Context, []int64, string) error {
	e.calls++
	return nil
}

func (e *recordingEngine) EndStudentEnrollmentsForCareExit(context.Context, []int64, string) (timetable.CareExitEnrollmentChanges, error) {
	e.calls++
	return timetable.CareExitEnrollmentChanges{}, nil
}

func (e *recordingEngine) RestoreStudentEnrollmentsForCareExit(context.Context, []int64, []int64, []timetable.CareExitEnrollmentRemoval) (int, error) {
	e.calls++
	return 0, nil
}

func (e *recordingEngine) ObserveRejection(operation string, _ time.Duration, _ error) {
	e.rejections = append(e.rejections, operation)
}

func TestModuleNormalizesCategoryWrites(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)

	created, err := module.CreateCategory(context.Background(), timetable.CreateCategory{
		Name: "  Werken  ", Description: "  Holz  ", Color: "abc",
	})
	require.NoError(t, err)
	assert.Equal(t, "Werken", created.Name)
	assert.Equal(t, "Holz", created.Description)
	assert.Equal(t, "#abc", created.Color)
	assert.Equal(t, engine.create.Name, created.Name)

	updated, err := module.UpdateCategory(context.Background(), timetable.UpdateCategory{
		ID: 17, Name: "  Sport  ", Color: "#123456",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(17), updated.ID)
	assert.Equal(t, "Sport", engine.update.Name)
}

func TestModuleRejectsInvalidAndReservedCategoryWrites(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)
	ctx := context.Background()

	_, err := module.CreateCategory(ctx, timetable.CreateCategory{Name: " ", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrInvalidCategory)
	_, err = module.CreateCategory(ctx, timetable.CreateCategory{Name: "wc", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrSystemCategoryName)
	_, err = module.UpdateCategory(ctx, timetable.UpdateCategory{ID: 3, Name: "Schulhof", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrSystemCategoryName)
	_, err = module.UpdateCategory(ctx, timetable.UpdateCategory{ID: 0, Name: "Sport", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrInvalidCategory)
	require.ErrorIs(t, module.SetCategoryShiftTypeLinks(ctx, 4, []int64{2, 0}), timetable.ErrInvalidCategory)
	require.ErrorIs(t, module.LockStudentEnrollmentsForCareExit(ctx, nil, "2026-09-04"), timetable.ErrInvalidCareExitEnrollment)
	_, err = module.EndStudentEnrollmentsForCareExit(ctx, []int64{1}, "04.09.2026")
	require.ErrorIs(t, err, timetable.ErrInvalidCareExitEnrollment)
	_, err = module.RestoreStudentEnrollmentsForCareExit(ctx, []int64{1}, nil, []timetable.CareExitEnrollmentRemoval{{
		CareExitEnrollment: timetable.CareExitEnrollment{ID: 1, TenantID: 7, StudentID: 1, ActivityGroupID: 1},
		WasDeleted:         true,
	}})
	require.ErrorIs(t, err, timetable.ErrInvalidCareExitEnrollment)

	assert.Zero(t, engine.calls, "invalid input must never reach persistence")
	assert.Equal(t, []string{
		"create_category", "create_category", "update_category", "update_category", "set_category_shift_type_links",
		"lock_student_enrollments_for_care_exit", "end_student_enrollments_for_care_exit",
		"restore_student_enrollments_for_care_exit",
	}, engine.rejections)
}

func TestSystemProvisionerMayCreateReservedCategory(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := timetable.NewModule(engine)

	_, err := module.CreateCategory(context.Background(), timetable.CreateCategory{
		Name: timetable.WCCategoryName, IsSystem: true,
	})
	require.NoError(t, err)
	assert.True(t, engine.create.IsSystem)
}

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "none", timetable.ErrorCode(nil))
	assert.Equal(t, "not_found", timetable.ErrorCode(timetable.ErrCategoryNotFound))
	assert.Equal(t, "category_name_exists", timetable.ErrorCode(timetable.ErrCategoryNameExists))
	assert.Equal(t, "internal_error", timetable.ErrorCode(errors.New("boom")))
}

func TestNewModulePanicsWithoutEngine(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { timetable.NewModule(nil) })
}
