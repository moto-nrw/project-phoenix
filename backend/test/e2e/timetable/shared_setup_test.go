// Package e2e_timetable exercises the timetable epic as end-to-end HTTP flows.
//
// Each flow test spins up the real service factory against the test DB and
// drives real HTTP requests through the production timetable router, minting
// actual JWTs with admin permissions. Tenant isolation is verified in-flow
// by issuing a second tenant's token and asserting 404.
//
// Fixtures follow the existing testpkg pattern; each scenario owns a primary
// tenant and an isolated neighbour, both created per test (#2419).
package e2e_timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Each scenario owns two tenants (#2419): the primary one every fixture lands
// in, and a neighbour used to prove cross-tenant isolation. Both are created
// per test, so no flow depends on the fixed bootstrap tenant.

// scenario bundles the common infrastructure a single flow needs.
type scenario struct {
	t              *testing.T
	db             *bun.DB
	resource       *timetableTestResource
	createVisit    func(context.Context, *activeModel.Visit) error
	endVisit       func(context.Context, int64) error
	previewCleanup func(context.Context) (*scheduleSvc.TimetableCleanupPreview, error)
	cleanup        func(context.Context) (*scheduleSvc.TimetableCleanupResult, error)
	router         timetableTestRouter
	tokenAuth      *timetableTestTokenAuth
	today          func() timezone.Date

	primaryTenant   int64
	secondaryTenant int64
}

// setupTimetableScenarioModule returns a ready-to-use scenario with real DB + timetable services
// + fully mounted timetable router + JWT signer.
func setupTimetableScenarioModule(t *testing.T, clocks ...func() time.Time) *scenario {
	t.Helper()

	db, factory := testutil.SetupAPITest(t, clocks...)
	primaryTenant := testpkg.Tenant(t)
	secondaryTenant := testpkg.NewTenantScope(t, db).TenantID

	// The package init below seeds the viper values NewTokenAuth reads.
	ta, err := newTimetableTokenAuth()
	require.NoError(t, err, "init JWT token auth")

	resource := newTimetableTestResource(timetableTestDependencies{
		CalendarPeriodService:  factory.CalendarPeriod,
		MaterializationService: factory.Materialization,
		InstanceService:        factory.Instance,
		PersonService:          factory.Users,
		TimetableData:          factory.TimetableData,
		UserContextService:     factory.UserContext,
		SettingsService:        factory.Settings,
		Broadcaster:            factory.RealtimeHub,
		Logger:                 slog.Default(), DB: db,
	})
	s := &scenario{
		t:               t,
		db:              db,
		resource:        resource,
		createVisit:     factory.Active.CreateVisit,
		endVisit:        factory.Active.EndVisit,
		previewCleanup:  factory.TimetableCleanup.PreviewExpiredTimetableData,
		cleanup:         factory.TimetableCleanup.CleanupExpiredTimetableData,
		tokenAuth:       ta,
		today:           timezone.CalendarDateClock(clocks...),
		primaryTenant:   primaryTenant,
		secondaryTenant: secondaryTenant,
	}
	s.router = s.mountRouter()
	return s
}

func (s *scenario) createActiveVisit(ctx context.Context, visit *activeModel.Visit) error {
	return s.createVisit(ctx, visit)
}

func (s *scenario) endActiveVisit(ctx context.Context, visitID int64) error {
	return s.endVisit(ctx, visitID)
}

func (s *scenario) previewTimetableCleanup(ctx context.Context) (*scheduleSvc.TimetableCleanupPreview, error) {
	return s.previewCleanup(ctx)
}

func (s *scenario) cleanupTimetable(ctx context.Context) (*scheduleSvc.TimetableCleanupResult, error) {
	return s.cleanup(ctx)
}

// tenantCtx returns a context bound to the primary tenant.
func (s *scenario) tenantCtx() context.Context {
	return tenant.WithTenantID(testpkg.WithTestTenantRuntime(s.t, context.Background()), s.primaryTenant)
}

// mountRouter builds the full timetable Resource with real services and
// returns its router (with JWT + tenant middleware intact).
func (s *scenario) mountRouter() timetableTestRouter {
	return s.resource.Router()
}

// do executes an HTTP request against the timetable router.
// method, path: e.g. ("POST", "/materialize"). body: nil or a JSON-marshalable struct.
// claims: which tenant/permissions to authenticate as.
func (s *scenario) do(method, path string, body any, claims timetableTestClaims) *httptest.ResponseRecorder {
	s.t.Helper()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(s.t, err, "marshal request body")
		reader = bytes.NewBuffer(b)
	}

	token, err := s.tokenAuth.CreateJWT(claims)
	require.NoError(s.t, err, "mint JWT")

	req := httptest.NewRequest(method, path, reader)
	req = req.WithContext(testpkg.WithTestTenantRuntime(s.t, req.Context()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	return rr
}

// decodeResponse parses {"status":"...","data":...} into a target struct's
// Data field. Uses the common envelope exposed by api/common.Respond.
func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder, target any) {
	t.Helper()
	var env struct {
		Status  string          `json:"status"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
		Error   string          `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env),
		"unmarshal envelope: %s", rr.Body.String())
	if target != nil && len(env.Data) > 0 {
		require.NoError(t, json.Unmarshal(env.Data, target),
			"unmarshal data payload: %s", string(env.Data))
	}
}

// adminClaimsForTenant builds admin JWT claims pinned to the given tenant,
// with the full permission set the timetable routes care about.
func adminClaimsForTenant(accountID, tenantID int64) timetableTestClaims {
	c := testutil.AdminTestClaims(int(accountID))
	c.Permissions = []string{
		permissions.SchedulesRead,
		permissions.SchedulesCreate,
		permissions.SchedulesUpdate,
		permissions.SchedulesDelete,
		permissions.SchedulesManage,
		permissions.ConfigRead,
		permissions.ConfigUpdate,
		permissions.ConfigManage,
		permissions.UsersRead,
		"admin:*",
	}
	c.IsAdmin = true
	c.TenantID = tenantID
	return c
}

// primaryAdminClaims returns claims for the primary tenant admin.
func (s *scenario) primaryAdminClaims() timetableTestClaims {
	return adminClaimsForTenant(1, s.primaryTenant)
}

// secondaryAdminClaims returns claims for an admin on the isolated tenant.
func (s *scenario) secondaryAdminClaims() timetableTestClaims {
	return adminClaimsForTenant(2, s.secondaryTenant)
}

// parseHHMM parses "HH:MM" into a time.Time anchored on 2000-01-01.
func parseHHMM(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err, "invalid HH:MM literal %q", hhmm)
	return time.Date(2000, 1, 1, parsed.Hour(), parsed.Minute(), 0, 0, time.UTC)
}

// createActivePeriod creates an active calendar period spanning 1 year before
// and 1 year after `anchor`.
func (s *scenario) createActivePeriod(name string, anchor timezone.Date) *scheduleModels.CalendarPeriod {
	s.t.Helper()
	period := &scheduleModels.CalendarPeriod{
		Name:            name,
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       timezone.NewDate(anchor.Year()-1, 8, 1),
		EndDate:         timezone.NewDate(anchor.Year()+1, 7, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(s.t, s.resource.CalendarPeriodService.CreatePeriod(s.tenantCtx(), period))
	return period
}

// createTimeframeWithTimes inserts a timeframe with explicit HH:MM times.
// schedule.timeframes stores timezone-free SQL TIME values; the service layer
// still normalises through timezone.NormalizeWallClock() so driver-specific date
// anchors cannot affect comparisons.
func (s *scenario) createTimeframeWithTimes(description, startHHMM, endHHMM string) *scheduleModels.Timeframe {
	s.t.Helper()
	start := parseHHMM(s.t, startHHMM)
	end := parseHHMM(s.t, endHHMM)

	tf := &scheduleModels.Timeframe{
		StartTime:   start,
		EndTime:     &end,
		IsActive:    true,
		Description: description,
	}
	tf.SetTenantID(s.primaryTenant)

	_, err := s.db.NewInsert().
		Model(tf).
		ModelTableExpr(`schedule.timeframes`).
		Exec(s.tenantCtx())
	require.NoError(s.t, err, "insert timeframe")
	return tf
}

func parseHHMMLocal(t *testing.T, hhmm string) time.Time {
	t.Helper()
	return parseHHMM(t, hhmm)
}

// templateSpec describes a weekly template to materialize later.
type templateSpec struct {
	name       string
	weekday    int
	startHHMM  string
	endHHMM    string
	roomID     int64
	staffIDs   []int64
	studentIDs []int64
	periodID   *int64
	validFrom  timezone.Date
	validUntil *timezone.Date
}

// templateFixture is the full bundle of rows behind one activity template.
type templateFixture struct {
	group        *activitiesModels.Group
	schedule     *activitiesModels.Schedule
	timeframe    *scheduleModels.Timeframe
	enrollments  []int64
	supervisors  []int64
	roomID       int64
	staffIDs     []int64
	studentIDs   []int64
	validFromUTC timezone.Date
}

// buildTemplate inserts a full template bundle so materialize() will pick it up.
func (s *scenario) buildTemplate(spec templateSpec) *templateFixture {
	s.t.Helper()
	ctx := s.tenantCtx()

	category := testpkg.CreateTestActivityCategory(s.t, s.db, spec.name)

	creator := testpkg.CreateTestStaff(s.t, s.db, "Creator", spec.name)

	group := &activitiesModels.Group{
		Name:            spec.name,
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &creator.ID,
		PlannedRoomID:   &spec.roomID,
		IsTemplate:      true,
	}
	group.SetTenantID(s.primaryTenant)
	_, err := s.db.NewInsert().
		Model(group).
		ModelTableExpr(`activities.groups AS "group"`).
		Exec(ctx)
	require.NoError(s.t, err, "insert activity group")

	timeframe := s.createTimeframeWithTimes(spec.name+"-tf", spec.startHHMM, spec.endHHMM)

	sched := &activitiesModels.Schedule{
		Weekday:          spec.weekday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  group.ID,
		WeekPattern:      0,
		CalendarPeriodID: spec.periodID,
	}
	sched.SetTenantID(s.primaryTenant)
	_, err = s.db.NewInsert().
		Model(sched).
		ModelTableExpr(`activities.schedules`).
		Exec(ctx)
	require.NoError(s.t, err, "insert schedule")

	validFrom := spec.validFrom
	if validFrom.IsZero() {
		validFrom = s.today().AddDays(-30)
	}

	var enrollmentIDs []int64
	for _, sid := range spec.studentIDs {
		enroll := &activitiesModels.StudentEnrollment{
			StudentID:       sid,
			ActivityGroupID: group.ID,
			ValidFrom:       validFrom,
			ValidUntil:      spec.validUntil,
		}
		enroll.SetTenantID(s.primaryTenant)
		_, err := s.db.NewInsert().
			Model(enroll).
			ModelTableExpr(`activities.student_enrollments`).
			Exec(ctx)
		require.NoError(s.t, err, "insert enrollment")
		enrollmentIDs = append(enrollmentIDs, enroll.ID)
	}
	var supervisorIDs []int64
	for i, stid := range spec.staffIDs {
		sup := &activitiesModels.SupervisorPlanned{
			StaffID:    stid,
			GroupID:    group.ID,
			IsPrimary:  i == 0,
			ValidFrom:  validFrom,
			ValidUntil: spec.validUntil,
		}
		sup.SetTenantID(s.primaryTenant)
		_, err := s.db.NewInsert().
			Model(sup).
			ModelTableExpr(`activities.supervisors`).
			Exec(ctx)
		require.NoError(s.t, err, "insert supervisor")
		supervisorIDs = append(supervisorIDs, sup.ID)
	}
	return &templateFixture{
		group:        group,
		schedule:     sched,
		timeframe:    timeframe,
		enrollments:  enrollmentIDs,
		supervisors:  supervisorIDs,
		roomID:       spec.roomID,
		staffIDs:     spec.staffIDs,
		studentIDs:   spec.studentIDs,
		validFromUTC: validFrom,
	}
}

// queryCounter is a bun.QueryHook that counts queries between reset and read.
type queryCounter struct {
	count atomic.Int64
}

func (q *queryCounter) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}
func (q *queryCounter) AfterQuery(_ context.Context, _ *bun.QueryEvent) {
	q.count.Add(1)
}
func (q *queryCounter) reset()     { q.count.Store(0) }
func (q *queryCounter) get() int64 { return q.count.Load() }

// nextWeekday returns the next occurrence of the given ISO weekday (1=Mon...7=Sun)
// at least `minDaysAhead` days from `from`.
func nextWeekday(from timezone.Date, isoWeekday, minDaysAhead int) timezone.Date {
	d := from.AddDays(minDaysAhead)
	for {
		w := int(d.Weekday())
		if w == 0 {
			w = 7
		}
		if w == isoWeekday {
			return d
		}
		d = d.AddDays(1)
	}
}
