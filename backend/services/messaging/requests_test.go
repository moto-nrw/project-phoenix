package messaging_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/messaging"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// stubStaffContext satisfies UserContextService by embedding the interface (so
// only the method the apply actually calls needs an implementation) and returns
// a fixed fixture staff for CreatedBy stamping.
type stubStaffContext struct {
	usercontext.UserContextService
	staff *usersModels.Staff
}

func (s stubStaffContext) GetCurrentStaff(context.Context) (*usersModels.Staff, error) {
	return s.staff, nil
}

func newApplyService(t *testing.T) (*messaging.TestApplyService, *services.Factory, *bun.DB, context.Context) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	serviceFactory, err := services.NewFactory(repositories.NewFactory(db), db, slog.Default())
	require.NoError(t, err)

	staff := testpkg.CreateTestStaff(t, db, "Apply", "Confirmer")
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db, staff.ID)
		_ = db.Close()
	})

	svc := messaging.NewTestApplyService(
		serviceFactory.Students,
		serviceFactory.Users,
		serviceFactory.ArrivalSchedule,
		serviceFactory.PickupSchedule,
		stubStaffContext{staff: staff},
		db,
	)
	return svc, serviceFactory, db, testpkg.TenantContext(1)
}

func careRequest(studentID int64, weekdays []any) *usersModels.ParentMessage {
	return &usersModels.ParentMessage{
		StudentID:   studentID,
		RequestType: usersModels.ParentMessageRequestCareSchedule,
		Payload:     map[string]any{"weekdays": weekdays},
	}
}

func TestApplyCareScheduleRequest_SetsModeArrivalAndPickup(t *testing.T) {
	svc, sf, db, ctx := newApplyService(t)
	student := testpkg.CreateTestStudent(t, db, "Care", "Plan", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	req := careRequest(student.ID, []any{
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"},
	})
	require.NoError(t, svc.ApplyCareSchedule(ctx, req))

	arrivals, err := sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, arrivals, 1)
	assert.Equal(t, 1, arrivals[0].Weekday)
	assert.Equal(t, "08:00", arrivals[0].ExpectedArrival.Format("15:04"))

	pickups, err := sf.PickupSchedule.GetStudentPickupSchedules(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, pickups, 1)
	assert.Equal(t, "16:00", pickups[0].PickupTime.Format("15:04"))

	reloaded, err := sf.Users.GetStudentByID(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.DeparturePickup, reloaded.DepartureDays["mon"])
}

func TestApplyCareScheduleRequest_MergesPreservingOtherDays(t *testing.T) {
	svc, sf, db, ctx := newApplyService(t)
	student := testpkg.CreateTestStudent(t, db, "Merge", "Plan", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	require.NoError(t, svc.ApplyCareSchedule(ctx, careRequest(student.ID,
		[]any{map[string]any{"weekday": 3, "arrival": "07:30"}})))
	require.NoError(t, svc.ApplyCareSchedule(ctx, careRequest(student.ID,
		[]any{map[string]any{"weekday": 1, "arrival": "08:15"}})))

	arrivals, err := sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, student.ID)
	require.NoError(t, err)
	byDay := map[int]string{}
	for _, a := range arrivals {
		byDay[a.Weekday] = a.ExpectedArrival.Format("15:04")
	}
	assert.Equal(t, "07:30", byDay[3], "Wednesday must be preserved by the merge")
	assert.Equal(t, "08:15", byDay[1], "Monday must be added")
}

func TestApplyStudentMasterDataRequest_UpdatesPersonAndStudent(t *testing.T) {
	svc, sf, db, ctx := newApplyService(t)
	student := testpkg.CreateTestStudent(t, db, "Old", "Name", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	req := &usersModels.ParentMessage{
		StudentID:   student.ID,
		RequestType: usersModels.ParentMessageRequestStudentMasterData,
		Payload: map[string]any{"fields": map[string]any{
			"first_name":     "Neu",
			"birthday":       "2018-05-04",
			"guardian_phone": "0151 99",
		}},
	}
	require.NoError(t, svc.ApplyStudentMasterData(ctx, req))

	person, err := sf.Users.Get(ctx, student.PersonID)
	require.NoError(t, err)
	assert.Equal(t, "Neu", person.FirstName)
	require.NotNil(t, person.Birthday)
	assert.Equal(t, "2018-05-04", person.Birthday.String())

	reloaded, err := sf.Users.GetStudentByID(ctx, student.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.GuardianPhone)
	assert.Equal(t, "0151 99", *reloaded.GuardianPhone)
}

func TestValidateRequestPayload(t *testing.T) {
	md := usersModels.ParentMessageRequestStudentMasterData
	cs := usersModels.ParentMessageRequestCareSchedule

	// Master data: disallowed field, empty name → error; valid → ok.
	require.Error(t, messaging.ValidateRequestPayload(md,
		map[string]any{"fields": map[string]any{"health_info": "x"}}))
	require.Error(t, messaging.ValidateRequestPayload(md,
		map[string]any{"fields": map[string]any{"first_name": ""}}))
	// Contact fields are validated with the SAME canonical rules Student.Validate
	// applies on apply, so a malformed value is rejected at create time instead of
	// being accepted and then 500ing every staff ConfirmRequest. ("012" is too short
	// for the canonical phone pattern; an empty value clears the field and is ok.)
	require.Error(t, messaging.ValidateRequestPayload(md,
		map[string]any{"fields": map[string]any{"guardian_phone": "012"}}))
	require.Error(t, messaging.ValidateRequestPayload(md,
		map[string]any{"fields": map[string]any{"guardian_email": "not-an-email"}}))
	require.NoError(t, messaging.ValidateRequestPayload(md,
		map[string]any{"fields": map[string]any{"guardian_phone": "0151 99"}}))
	require.NoError(t, messaging.ValidateRequestPayload(md,
		map[string]any{"fields": map[string]any{"guardian_email": "anna@example.com"}}))

	// Care schedule: bad weekday, bad time, no changes → error; valid → ok.
	require.Error(t, messaging.ValidateRequestPayload(cs,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 9, "mode": "bus"}}}))
	require.Error(t, messaging.ValidateRequestPayload(cs,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "25:99"}}}))
	require.Error(t, messaging.ValidateRequestPayload(cs,
		map[string]any{"weekdays": []any{}}))
	require.NoError(t, messaging.ValidateRequestPayload(cs,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}}))
}

// TestCanonicalizeRequestPayload locks in that the create path persists only the
// sanitized payload: unknown keys are dropped and duplicate weekday entries are
// collapsed to one canonical entry, so a direct API client cannot store ignored
// junk that every later thread load would echo back.
func TestCanonicalizeRequestPayload(t *testing.T) {
	md := usersModels.ParentMessageRequestStudentMasterData
	cs := usersModels.ParentMessageRequestCareSchedule

	// Master data: a valid field plus an unknown sibling key → the sibling is
	// dropped, only {"fields": {...}} survives.
	out, err := messaging.CanonicalizeRequestPayload(md, map[string]any{
		"fields":  map[string]any{"first_name": "Anna"},
		"garbage": "ignored",
		"bloat":   []any{1, 2, 3},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"fields"}, keysOf(out))
	fields, ok := out["fields"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"first_name": "Anna"}, fields)

	// Care schedule: duplicate Monday entries (last value wins per aspect) plus an
	// unknown key → exactly one Monday entry, no unknown keys.
	out, err = messaging.CanonicalizeRequestPayload(cs, map[string]any{
		"weekdays": []any{
			map[string]any{"weekday": 1, "arrival": "08:00"},
			map[string]any{"weekday": 1, "arrival": "09:00"},
			map[string]any{"weekday": 1, "pickup": "15:00"},
		},
		"junk": "ignored",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"weekdays"}, keysOf(out))
	weekdays, ok := out["weekdays"].([]any)
	require.True(t, ok)
	require.Len(t, weekdays, 1)
	monday, ok := weekdays[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "09:00", monday["arrival"])
	assert.Equal(t, "15:00", monday["pickup"])

	// Invalid payloads still error (canonicalize is also the validator).
	_, err = messaging.CanonicalizeRequestPayload(cs,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 9, "mode": "bus"}}})
	require.Error(t, err)
	_, err = messaging.CanonicalizeRequestPayload("unknown.type", map[string]any{})
	require.Error(t, err)
}

func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestCareScheduleDiff_ShowsCurrentVsRequested(t *testing.T) {
	svc, _, db, ctx := newApplyService(t)
	student := testpkg.CreateTestStudent(t, db, "Diff", "Plan", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	// Seed a current Monday arrival, then diff a request changing it + the mode.
	require.NoError(t, svc.ApplyCareSchedule(ctx, careRequest(student.ID,
		[]any{map[string]any{"weekday": 1, "arrival": "07:30"}})))

	diff, err := svc.CareScheduleDiff(ctx, student.ID, map[string]any{"weekdays": []any{
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00"},
	}})
	require.NoError(t, err)

	byLabel := map[string]messaging.RequestDiffEntry{}
	for _, e := range diff {
		byLabel[e.Label] = e
	}
	assert.Equal(t, "07:30", byLabel["Montag · Bringzeit"].Old)
	assert.Equal(t, "08:00", byLabel["Montag · Bringzeit"].New)
	assert.Equal(t, "Geht alleine", byLabel["Montag · Abholart"].Old)
	assert.Equal(t, "Wird abgeholt", byLabel["Montag · Abholart"].New)
}

func TestApplyStudentMasterDataRequest_RejectsDisallowedAndEmpty(t *testing.T) {
	svc, _, db, ctx := newApplyService(t)
	student := testpkg.CreateTestStudent(t, db, "Keep", "Name", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	disallowed := &usersModels.ParentMessage{
		StudentID:   student.ID,
		RequestType: usersModels.ParentMessageRequestStudentMasterData,
		Payload:     map[string]any{"fields": map[string]any{"health_info": "Allergie"}},
	}
	require.Error(t, svc.ApplyStudentMasterData(ctx, disallowed),
		"health_info (Art. 9) must be rejected")

	empty := &usersModels.ParentMessage{
		StudentID:   student.ID,
		RequestType: usersModels.ParentMessageRequestStudentMasterData,
		Payload:     map[string]any{"fields": map[string]any{"first_name": ""}},
	}
	require.Error(t, svc.ApplyStudentMasterData(ctx, empty),
		"empty name must be rejected")
}
