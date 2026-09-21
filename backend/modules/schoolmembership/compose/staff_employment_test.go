package compose

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memoryEmployment stands in for the Workforce owner of the employment
// profile. The module's own tests cannot compose Workforce; the real binding
// is covered where both owners are composed (database/repositories).
type memoryEmployment struct {
	mu       sync.Mutex
	profiles map[int64]StaffEmploymentProfile
	failSave error
}

// testEmployment is shared by the modules one test composes separately
// (membership and offboarding). Membership IDs are unique across tenants, so
// parallel tests never meet in it.
var testEmployment = newMemoryEmployment()

func newMemoryEmployment() *memoryEmployment {
	return &memoryEmployment{profiles: map[int64]StaffEmploymentProfile{}}
}

func (m *memoryEmployment) StaffEmployments(_ context.Context, ids []int64) (map[int64]StaffEmploymentProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := map[int64]StaffEmploymentProfile{}
	for _, id := range ids {
		if profile, ok := m.profiles[id]; ok {
			result[id] = profile
		}
	}
	return result, nil
}

func (m *memoryEmployment) SaveStaffEmployment(_ context.Context, value StaffEmploymentProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failSave != nil {
		return m.failSave
	}
	m.profiles[value.MembershipID] = value
	return nil
}

func (m *memoryEmployment) ClearStaffWorkTimeModel(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	profile, ok := m.profiles[id]
	if !ok {
		return schoolmembership.ErrStaffNotFound
	}
	profile.WorkTimeModelID = nil
	m.profiles[id] = profile
	return nil
}

func TestModuleRequiresTheEmploymentOwner(t *testing.T) {
	t.Parallel()
	_, err := New(Dependencies{DB: testpkg.SetupTestDB(t), Observe: func(Observation) {}})
	require.ErrorIs(t, err, errStaffEmploymentUnbound)
	_, err = NewOffboarding(Dependencies{DB: testpkg.SetupTestDB(t), Observe: func(Observation) {}})
	require.ErrorIs(t, err, errStaffEmploymentUnbound)
}

func TestModuleComposesTheEmploymentProfileIntoStaff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := newMemoryEmployment()
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: employment})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	created := createStaff(t, ctx, db, module, "Composed", "Staff", schoolmembership.StaffFields{
		StaffNotes: "Notiz", PersonnelNumber: testpkg.StrPtr("P-7"),
	})
	require.Equal(t, StaffEmploymentProfile{MembershipID: created.ID, StaffNotes: "Notiz", PersonnelNumber: testpkg.StrPtr("P-7")},
		employment.profiles[created.ID], "the employment half is handed to its owner under the membership ID")

	employment.profiles[created.ID] = StaffEmploymentProfile{MembershipID: created.ID, StaffNotes: "Vom Owner"}
	found, err := module.FindStaff(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Vom Owner", found.StaffNotes, "reads take the employment half from its owner")
	assert.Nil(t, found.PersonnelNumber)
	listed, err := module.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{created.ID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "Vom Owner", listed[0].StaffNotes)
}

func TestModuleRollsTheMembershipBackWhenTheEmploymentWriteFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	employment := newMemoryEmployment()
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: employment})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	person := testpkg.CreateTestPerson(t, db, "Failed", "Employment")

	injected := errors.New("employment owner refused")
	employment.failSave = injected
	_, err = module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.ErrorIs(t, err, injected)
	_, err = module.FindStaffByPerson(ctx, person.ID)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound, "no membership may outlive a refused employment write")

	employment.failSave = nil
	created, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.NoError(t, err, "the retry succeeds once the owner accepts")
	assert.Positive(t, created.ID)
}
