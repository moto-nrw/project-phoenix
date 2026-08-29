// Integration tests for GET /instances/{id}/participants (#2283): the narrow
// per-instance name list that lets schedules:read holders (Leseansicht) see
// who takes part in a block without the users:read-gated full roster. Same
// no-middleware router pattern as student_day_test.go — tenant context and
// permissions are injected directly into the request context.
package timetable

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type participantsSetup struct {
	res           *Resource
	db            *bun.DB
	ctx           context.Context
	instanceID    int64
	students      []*participantFixture
	staffID       int64
	staffFullName string
	roomID        int64
	activityID    int64
}

type participantFixture struct {
	studentID int64
	rowID     int64
	fullName  string
}

// buildParticipantsSetup creates one planned instance with two enrolled
// students whose person names sort deterministically.
func buildParticipantsSetup(t *testing.T) *participantsSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("PT-Room-%d", suffix))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("PT-Act-%d", suffix))

	inst := testpkg.CreateTestActivityInstance(t, db, timezone.NewDate(2026, 4, 22), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &activity.ID,
		StartHHMM:       "14:00",
		EndHHMM:         "15:00",
		Title:           "PT-Lernzeit",
	})

	setup := &participantsSetup{
		db:         db,
		ctx:        ctx,
		instanceID: inst.ID,
		roomID:     room.ID,
		activityID: activity.ID,
	}
	staff := testpkg.CreateTestStaff(t, db, "Sara", fmt.Sprintf("Staffel-%d", suffix))
	testpkg.CreateTestInstanceStaff(t, db, inst.ID, staff.ID, testpkg.InstanceStaffOpts{})
	setup.staffID = staff.ID
	setup.staffFullName = fmt.Sprintf("Sara Staffel-%d", suffix)

	for i, name := range []struct{ first, last string }{{"Anna", "Alpha"}, {"Ben", "Beta"}} {
		student := testpkg.CreateTestStudent(t, db, name.first, fmt.Sprintf("%s-%d", name.last, suffix), fmt.Sprintf("%da", i+1))
		row := testpkg.CreateTestInstanceStudent(t, db, inst.ID, student.ID, schedule.AttendanceStatusExpected)
		setup.students = append(setup.students, &participantFixture{
			studentID: student.ID,
			rowID:     row.ID,
			fullName:  fmt.Sprintf("%s %s-%d", name.first, name.last, suffix),
		})
	}

	studentRepo := usersRepo.NewStudentRepository(db)
	personRepo := usersRepo.NewPersonRepository(db)
	setup.res = NewResource(Dependencies{
		TimetableData: testTimetableData(db),
		PersonService: usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
			StudentRepo: studentRepo,
			PersonRepo:  personRepo,
			StaffRepo:   usersRepo.NewStaffRepository(db),
		}),
		// UserContextService intentionally nil: the admin-perm path
		// short-circuits CanReadStudent; the non-staff test relies on the
		// fallthrough filtering every student out.
		DB: db,
	})
	return setup
}

func participantsRouter(parentCtx context.Context, res *Resource, perms []string) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			ctx = context.WithValue(ctx, jwt.CtxPermissions, perms)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/instances/{id}/participants", res.getInstanceParticipants)
	return r
}

func TestGetInstanceParticipants_AdminSeesSortedNames(t *testing.T) {
	t.Parallel()

	s := buildParticipantsSetup(t)

	router := participantsRouter(s.ctx, s.res, []string{"admin:*"})
	w := doGet(t, router, fmt.Sprintf("/instances/%d/participants", s.instanceID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var got InstanceParticipantsResponse
	decodeEnvelope(t, w, &got)

	assert.Equal(t, s.instanceID, got.InstanceID)
	require.Len(t, got.Participants, 2)
	assert.Equal(t, s.students[0].studentID, got.Participants[0].StudentID)
	assert.Equal(t, s.students[0].fullName, got.Participants[0].DisplayName)
	assert.Equal(t, s.students[1].fullName, got.Participants[1].DisplayName)
}

func TestGetInstanceParticipants_NonStaffSeesNoStudentNames(t *testing.T) {
	t.Parallel()

	s := buildParticipantsSetup(t)

	// schedules:read alone reaches the endpoint, but with no verified staff
	// context every name is filtered out — 200 with an empty list, never a 403
	// and never a leak.
	router := participantsRouter(s.ctx, s.res, []string{"schedules:read"})
	w := doGet(t, router, fmt.Sprintf("/instances/%d/participants", s.instanceID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var got InstanceParticipantsResponse
	decodeEnvelope(t, w, &got)
	assert.Equal(t, s.instanceID, got.InstanceID)
	assert.Empty(t, got.Participants)
	// Staff names are deliberately unfiltered — the team overview ("wer ist
	// wo eingesetzt") must work even when no child is readable.
	require.Len(t, got.Staff, 1)
	assert.Equal(t, s.staffID, got.Staff[0].StaffID)
	assert.Equal(t, s.staffFullName, got.Staff[0].DisplayName)
}

func TestGetInstanceParticipants_AlumnusExcluded(t *testing.T) {
	t.Parallel()

	s := buildParticipantsSetup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.db.NewUpdate().
		Table("users.students").
		Set("status = ?", "alumnus").
		Where("id = ?", s.students[0].studentID).
		Exec(ctx)
	require.NoError(t, err)

	router := participantsRouter(s.ctx, s.res, []string{"admin:*"})
	w := doGet(t, router, fmt.Sprintf("/instances/%d/participants", s.instanceID))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var got InstanceParticipantsResponse
	decodeEnvelope(t, w, &got)
	require.Len(t, got.Participants, 1)
	assert.Equal(t, s.students[1].studentID, got.Participants[0].StudentID)
}

func TestGetInstanceParticipants_UnknownInstance404(t *testing.T) {
	t.Parallel()

	s := buildParticipantsSetup(t)

	router := participantsRouter(s.ctx, s.res, []string{"admin:*"})
	w := doGet(t, router, "/instances/999999999/participants")
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}
