package enrollment_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
)

// Fixtures of the enrollment flow suites (#3565). The suites seed Care
// Plan's offerings in enrollment rows and the Timetable owner's care
// templates straight in the database, because this adapter's tests may not
// import the Timetable models. Every helper runs on the database a suite
// opened through setupRequestTest, setupRolloverTest or setupDecisionTest.

// careOfferingFixtures seeds and reads Care Plan's offerings in enrollment
// rows through the owner capability, the way the retained offering
// repository did.
type careOfferingFixtures struct{ carePlan careplan.Capability }

func newCareOfferingFixtures(carePlan careplan.Capability) careOfferingFixtures {
	return careOfferingFixtures{carePlan: carePlan}
}

func (r careOfferingFixtures) Create(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil {
		return errors.New("CareOffering cannot be nil or zero value")
	}
	if err := offering.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fields, err := careOfferingFixtureFields(offering)
	if err != nil {
		return fmt.Errorf("encode care offering: %w", err)
	}
	created, err := r.carePlan.CreateCareOffering(ctx, careplan.CreateCareOffering{CareOfferingFields: fields})
	if err != nil {
		return fmt.Errorf("failed to create care offering: %w", err)
	}
	return applyCareOfferingFixture(offering, created)
}

func (r careOfferingFixtures) Update(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil {
		return errors.New("CareOffering cannot be nil or zero value")
	}
	if err := offering.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fields, err := careOfferingFixtureFields(offering)
	if err != nil {
		return fmt.Errorf("encode care offering: %w", err)
	}
	updated, err := r.carePlan.UpdateCareOffering(ctx, careplan.UpdateCareOffering{ID: offering.ID, CareOfferingFields: fields})
	if err != nil {
		return fmt.Errorf("failed to update care offering: %w", err)
	}
	return applyCareOfferingFixture(offering, updated)
}

func (r careOfferingFixtures) ReplaceAutoAddTriggers(ctx context.Context, id int64, triggers []int64) error {
	return r.carePlan.ReplaceAutoAddTriggers(ctx, id, triggers)
}

func (r careOfferingFixtures) CountByPhaseID(ctx context.Context, phaseID int64) (int, error) {
	return r.carePlan.CountCareOfferingsByPhase(ctx, phaseID)
}

func (r careOfferingFixtures) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	return testutil.NewEnrollmentCareOfferingRecords(r.carePlan).ListByPhase(ctx, phaseID)
}

func (r careOfferingFixtures) ListByIDs(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error) {
	return testutil.NewEnrollmentCareOfferingRecords(r.carePlan).ListByIDs(ctx, ids)
}

func (r careOfferingFixtures) ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error) {
	return testutil.NewEnrollmentCareOfferingRecords(r.carePlan).ListByIDsForUpdate(ctx, ids)
}

func (r careOfferingFixtures) ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	return testutil.NewEnrollmentCareOfferingRecords(r.carePlan).ListActiveByPhase(ctx, phaseID)
}

func (r careOfferingFixtures) FindByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	value, err := r.carePlan.FindCareOffering(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find care offering: %w", err)
	}
	offering := new(enrollmentModels.CareOffering)
	return offering, applyCareOfferingFixture(offering, value)
}

// careOfferingReads are the offering reads the flows run through Care
// Plan's offerings in enrollment rows; doubles embed them to fail one read.
type careOfferingReads interface {
	ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error)
	ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error)
	ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error)
}

func careOfferingFixtureFields(offering *enrollmentModels.CareOffering) (careplan.CareOfferingFields, error) {
	availabilityRule, err := json.Marshal(offering.AvailabilityRule)
	if err != nil {
		return careplan.CareOfferingFields{}, err
	}
	return careplan.CareOfferingFields{
		PhaseID: offering.PhaseID, ActivityGroupID: offering.ActivityGroupID, Name: offering.Name, Description: offering.Description,
		DaysOfWeekMode: offering.DaysOfWeekMode, AvailableDays: offering.AvailableDays,
		IncludesHolidayCare: offering.IncludesHolidayCare, IncludesLunch: offering.IncludesLunch,
		Capacity: offering.Capacity, PriceCents: offering.PriceCents, IsActive: offering.IsActive, IsRequired: offering.IsRequired,
		CountsAsCare: offering.CountsAsCare, AutoAddGradeLevels: offering.AutoAddGradeLevels,
		AvailabilityRule: availabilityRule, SortOrder: offering.SortOrder,
		SelectionGroup: offering.SelectionGroup, SelectionRule: offering.SelectionRule, PickupTimes: offering.PickupTimes,
		AutoAddTriggerOfferingIDs: offering.AutoAddTriggerOfferingIDs, Translations: offering.Translations,
	}, nil
}

func applyCareOfferingFixture(target *enrollmentModels.CareOffering, value careplan.CareOffering) error {
	target.ID, target.CreatedAt, target.UpdatedAt, target.TenantID = value.ID, value.CreatedAt, value.UpdatedAt, value.TenantID
	target.PhaseID, target.ActivityGroupID = value.PhaseID, value.ActivityGroupID
	target.Name, target.Description = value.Name, value.Description
	target.DaysOfWeekMode, target.AvailableDays = value.DaysOfWeekMode, value.AvailableDays
	target.IncludesHolidayCare, target.IncludesLunch = value.IncludesHolidayCare, value.IncludesLunch
	target.Capacity, target.PriceCents = value.Capacity, value.PriceCents
	target.IsActive, target.IsRequired = value.IsActive, value.IsRequired
	target.CountsAsCare, target.CountsAsCareSet = value.CountsAsCare, true
	target.AutoAddGradeLevels = value.AutoAddGradeLevels
	if len(value.AvailabilityRule) > 0 && string(value.AvailabilityRule) != "null" {
		if err := json.Unmarshal(value.AvailabilityRule, &target.AvailabilityRule); err != nil {
			return fmt.Errorf("decode care offering availability rule: %w", err)
		}
	}
	target.SortOrder, target.SelectionGroup, target.SelectionRule = value.SortOrder, value.SelectionGroup, value.SelectionRule
	target.PickupTimes, target.Translations = value.PickupTimes, value.Translations
	target.AutoAddTriggerOfferingIDs = value.AutoAddTriggerOfferingIDs
	return nil
}

// requestTestBookingCommands records and replaces the intake's bookings
// through Care Plan's effective-booking owner.
func requestTestBookingCommands() testutil.EnrollmentCareBookingCommands {
	return testutil.NewEnrollmentCareBookingCommands(testutil.NewTestOfferingBookings())
}

// recordCareBookingsFunc is a booking writer double.
type recordCareBookingsFunc func(context.Context, int64, []capability.CareBookingInput) error

func (f recordCareBookingsFunc) RecordCareBookings(ctx context.Context, childID int64, bookings []capability.CareBookingInput) error {
	return f(ctx, childID, bookings)
}

func testRepositories(t *testing.T, db *bun.DB) *repositories.EnrollmentFlowTestRepositories {
	t.Helper()
	return flowRepositories(t, db)
}

// flowRepositories composes the repositories the enrollment flow suites run
// on over the suite's database.
func flowRepositories(t testing.TB, db *bun.DB) *repositories.EnrollmentFlowTestRepositories {
	t.Helper()
	repos, err := repositories.NewEnrollmentFlowTestRepositories(db)
	require.NoError(t, err)
	return repos
}

func testGuardianAccess(db *bun.DB) enrollmentAPI.TestDecisionGuardianAccess {
	module, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	if err != nil {
		panic(err)
	}
	return module
}

func testStudentEnrollment(db *bun.DB) enrollmentAPI.TestDecisionStudentEnrollment {
	module, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		panic(err)
	}
	return module
}

// careTemplateGroup is a care template of the Timetable owner as the flow
// suites seed it.
type careTemplateGroup struct {
	bun.BaseModel `bun:"table:activities.groups,alias:grp"`

	ID                    int64   `bun:"id,pk,autoincrement"`
	TenantID              int64   `bun:"tenant_id,notnull"`
	Name                  string  `bun:"name,notnull"`
	MaxParticipants       int     `bun:"max_participants,nullzero"`
	IsOpen                bool    `bun:"is_open,notnull"`
	CategoryID            int64   `bun:"category_id,notnull"`
	PlannedRoomID         *int64  `bun:"planned_room_id"`
	Type                  string  `bun:"type,notnull"`
	IsTemplate            bool    `bun:"is_template,notnull"`
	SeriesRootID          *int64  `bun:"series_root_id"`
	CalendarPeriodID      *int64  `bun:"calendar_period_id"`
	TargetGroupType       string  `bun:"target_group_type,notnull"`
	SourceCareOfferingIDs []int64 `bun:"source_care_offering_ids,type:jsonb,nullzero"`
	SourceGradeLevels     []int   `bun:"source_grade_levels,type:jsonb,nullzero"`
}

func (g *careTemplateGroup) SetTenantID(id int64) { g.TenantID = id }

// createCareTemplateGroupRow stores a care template; a template without a
// target group keeps the manually curated roster.
func createCareTemplateGroupRow(ctx context.Context, db bun.IDB, group *careTemplateGroup) error {
	if group.TargetGroupType == "" {
		group.TargetGroupType = timetable.TargetGroupTypeNone
	}
	_, err := db.NewInsert().Model(group).Returning("id").Exec(ctx)
	return err
}

// createCareOfferingTemplateGroup creates a care template with its own
// category and room.
func createCareOfferingTemplateGroup(t *testing.T, db *bun.DB, name string) *careTemplateGroup {
	t.Helper()
	category := testpkg.CreateTestActivityCategory(t, db, "CareTemplate-"+name)
	room := testpkg.CreateTestRoom(t, db, "CareTemplate-"+name)
	group := &careTemplateGroup{
		Name:            uniqueSchemaName(name + "-" + t.Name()),
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		PlannedRoomID:   &room.ID,
		Type:            timetable.GroupTypeCare,
		IsTemplate:      true,
	}
	group.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, createCareTemplateGroupRow(testpkg.Ctx(t), db, group))
	return group
}

// careTemplateSchedule is a weekday schedule of a care template.
type careTemplateSchedule struct {
	bun.BaseModel `bun:"table:activities.schedules,alias:schedule"`

	ID               int64          `bun:"id,pk,autoincrement"`
	TenantID         int64          `bun:"tenant_id,notnull"`
	Weekday          int            `bun:"weekday,notnull"`
	TimeframeID      *int64         `bun:"timeframe_id"`
	ActivityGroupID  int64          `bun:"activity_group_id,notnull"`
	WeekPattern      int            `bun:"week_pattern,notnull"`
	CalendarPeriodID *int64         `bun:"calendar_period_id"`
	ValidUntil       *timezone.Date `bun:"valid_until"`
	ValidFrom        *timezone.Date `bun:"valid_from"`
}

func (s *careTemplateSchedule) SetTenantID(id int64) { s.TenantID = id }

func createCareTemplateScheduleRow(ctx context.Context, db bun.IDB, schedule *careTemplateSchedule) error {
	_, err := db.NewInsert().Model(schedule).Returning("id").Exec(ctx)
	return err
}

func createCareOfferingTemplateSchedule(t *testing.T, db *bun.DB, groupID int64, weekday int, periodID *int64) {
	t.Helper()
	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, testpkg.Tenant(t), "CareTemplate")
	schedule := &careTemplateSchedule{
		Weekday:          weekday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  groupID,
		CalendarPeriodID: periodID,
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, createCareTemplateScheduleRow(testpkg.Ctx(t), db, schedule))
}

// calendarPeriod is a school calendar period.
type calendarPeriod struct {
	bun.BaseModel `bun:"table:schedule.calendar_periods,alias:calendar_period"`

	ID              int64         `bun:"id,pk,autoincrement"`
	TenantID        int64         `bun:"tenant_id,notnull"`
	Name            string        `bun:"name,notnull"`
	PeriodType      string        `bun:"period_type,notnull"`
	StartDate       timezone.Date `bun:"start_date,notnull"`
	EndDate         timezone.Date `bun:"end_date,notnull"`
	WeekCycleLength int           `bun:"week_cycle_length,notnull"`
	IsActive        bool          `bun:"is_active,notnull"`
}

func (p *calendarPeriod) SetTenantID(id int64) { p.TenantID = id }

func createCalendarPeriodRow(ctx context.Context, db bun.IDB, period *calendarPeriod) error {
	_, err := db.NewInsert().Model(period).Returning("id").Exec(ctx)
	return err
}

func createCareOfferingTestPeriod(t *testing.T, db *bun.DB, name string, start, end timezone.Date) *calendarPeriod {
	t.Helper()
	period := &calendarPeriod{
		Name:            uniqueSchemaName(name + "-" + t.Name()),
		PeriodType:      schoolcalendar.PeriodTypeCustom,
		StartDate:       start,
		EndDate:         end,
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, createCalendarPeriodRow(testpkg.Ctx(t), db, period))
	return period
}

// studentEnrollment is a roster row of an activity group.
type studentEnrollment struct {
	bun.BaseModel `bun:"table:activities.student_enrollments,alias:student_enrollment"`

	ID                       int64          `bun:"id,pk,autoincrement"`
	CreatedAt                time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID                 int64          `bun:"tenant_id,notnull"`
	StudentID                int64          `bun:"student_id,notnull"`
	ActivityGroupID          int64          `bun:"activity_group_id,notnull"`
	ValidFrom                timezone.Date  `bun:"valid_from,notnull"`
	ValidUntil               *timezone.Date `bun:"valid_until"`
	CalendarPeriodID         *int64         `bun:"calendar_period_id"`
	EnrollmentRequestChildID *int64         `bun:"enrollment_request_child_id"`
	SelectedWeekdays         []int          `bun:"selected_weekdays,type:jsonb,nullzero"`
	AttendanceStatus         *string        `bun:"attendance_status"`
	Weekday                  *int           `bun:"weekday"`
}

func (e *studentEnrollment) SetTenantID(id int64) { e.TenantID = id }

// Weekdays of the care templates (ISO: Monday is 1).
const (
	weekdayMonday   = timetable.WeekdayMonday
	weekdayTuesday  = timetable.WeekdayTuesday
	weekdayThursday = timetable.WeekdayThursday
)

// newPickupBaselineService binds native records in the fixture's legacy
// booking mode.
func newPickupBaselineService(records careplan.Capability, links careplan.ApprovedBookingReader) careplan.PickupBaselineReader {
	return testutil.NewStoredPickupBaselines(records, links)
}

// guardianLinkHasPermission reports whether a relationship grants the
// parents-portal permission, the way the authorization helper reads it.
func guardianLinkHasPermission(link *usersModels.StudentGuardian, permission string) bool {
	if link == nil {
		return false
	}
	_, _, _, _, _, permissions := link.GuardianAuthorizationData()
	value, exists := permissions[permission]
	if !exists {
		return false
	}
	if granted, ok := value.(bool); ok {
		return granted
	}
	return value != nil
}

func nextWeekday(from timezone.Date, weekday time.Weekday) timezone.Date {
	date := from
	for date.Weekday() != weekday {
		date = date.AddDays(1)
	}
	return date
}

// registerSourcedInstanceCleanup drops an occurrence's attendance rows.
func registerSourcedInstanceCleanup(t *testing.T, env *decisionTestEnv, instanceIDs ...int64) {
	t.Helper()
	t.Cleanup(func() {
		for _, instanceID := range instanceIDs {
			_, _ = env.db.NewDelete().
				TableExpr("schedule.instance_students").
				Where("instance_id = ?", instanceID).
				Exec(context.Background())
		}
	})
}

func pendingWithdrawalForStudent(
	ctx context.Context,
	repo usersModels.CareWithdrawalCompletionRepository,
	studentID int64,
) (*usersModels.CareWithdrawalCompletion, error) {
	rows, _, err := repo.ListPending(ctx, usersModels.CareWithdrawalCompletionFilter{StudentID: studentID, Page: 1, PageSize: 1})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func carePickupTimes(days ...string) map[string]string {
	times := make(map[string]string, len(days))
	for _, day := range days {
		times[day] = "14:30"
	}
	return times
}

// createSourcedTemplate creates a live template that declares the offering as
// its roster source, with one Monday schedule pinned to the period (#2137).
func createSourcedTemplate(
	t *testing.T,
	env *decisionTestEnv,
	name string,
	offeringID int64,
	gradeLevels []int,
	period *calendarPeriod,
) *careTemplateGroup {
	t.Helper()
	group := createCareOfferingTemplateGroup(t, env.db, name)
	group.TargetGroupType = timetable.TargetGroupTypeOffering
	group.SourceCareOfferingIDs = []int64{offeringID}
	group.SourceGradeLevels = gradeLevels
	group.CalendarPeriodID = &period.ID
	_, err := env.db.NewUpdate().Model(group).
		Column("target_group_type", "source_care_offering_ids", "source_grade_levels", "calendar_period_id").
		WherePK().Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, weekdayMonday, &period.ID)
	return group
}

func createSourceOffering(t *testing.T, env *decisionTestEnv, name string, activityGroupID *int64) *enrollmentModels.CareOffering {
	t.Helper()
	ctx := testpkg.Ctx(t)
	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: activityGroupID,
		Name:            uniqueSchemaName(name + "-" + t.Name()),
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon"},
		PickupTimes:     carePickupTimes("mon"),
		IsActive:        true,
	}
	offering.TenantID = testpkg.Tenant(t)
	require.NoError(t, newCareOfferingFixtures(env.repos.CarePlan()).Create(ctx, offering))
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("id = ?", offering.ID).
			Exec(context.Background())
	})
	return offering
}

func offeringSourcePeriod(t *testing.T, env *decisionTestEnv) *calendarPeriod {
	t.Helper()
	return createCareOfferingTestPeriod(t, env.db, "offering-source",
		timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(-31),
		timezone.Date(env.sourcePhase.ServiceEndDate).AddDays(31))
}

// submitApprovedAdjustmentChild submits and approves a one-child request for
// the given offerings and returns the request, child and created student.
func submitApprovedAdjustmentChild(
	t *testing.T,
	env *decisionTestEnv,
	email, lastName string,
	offerings []*enrollmentModels.CareOffering,
) (requestID, childID, studentID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)

	offeringIDs := make([]int64, 0, len(offerings))
	offeringDays := make([]enrollmentAPI.SubmitOfferingDays, 0, len(offerings))
	for _, offering := range offerings {
		offeringIDs = append(offeringIDs, offering.ID)
		offeringDays = append(offeringDays, enrollmentAPI.SubmitOfferingDays{
			OfferingID:   offering.ID,
			SelectedDays: []string{"mon"},
		})
	}

	submitted, err := env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  lastName,
		GuardianEmail:     email,
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentAPI.SubmitChild{
			{
				FirstName:        "Kind",
				LastName:         lastName,
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      offeringIDs,
				OfferingDays:     offeringDays,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)

	outcome, err := env.decision.Decide(ctx, enrollmentAPI.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     capability.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	return submitted.Request.ID, submitted.Children[0].ID, *outcome.Child.CreatedStudentID
}

// offeringChangeFixture sets up an approved child booked into oldOffering,
// plus a second offering it could switch to.
type offeringChangeFixture struct {
	requestID    int64
	childID      int64
	studentID    int64
	oldOffering  *enrollmentModels.CareOffering
	newOffering  *enrollmentModels.CareOffering
	oldGroupID   int64
	newGroupID   int64
	switchDate   timezone.Date
	pastSwitchAt timezone.Date
}

func setupOfferingChangeFixture(
	t *testing.T,
	env *decisionTestEnv,
	label string,
) *offeringChangeFixture {
	t.Helper()
	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "OfferChangeOld"+label)
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "OfferChangeNew"+label)
	oldOffering := createAdjustmentCareOfferingWith(t, env, "Bisher "+label, func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &oldGroup.ID
		o.SortOrder = 201
	})
	newOffering := createAdjustmentCareOfferingWith(t, env, "Neu "+label, func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &newGroup.ID
		o.SortOrder = 202
	})
	requestID, childID, studentID := submitApprovedAdjustmentChild(
		t, env, "offer-change-"+label+"@example.com", "OfferChange"+label,
		[]*enrollmentModels.CareOffering{oldOffering},
	)
	return &offeringChangeFixture{
		requestID:    requestID,
		childID:      childID,
		studentID:    studentID,
		oldOffering:  oldOffering,
		newOffering:  newOffering,
		oldGroupID:   oldGroup.ID,
		newGroupID:   newGroup.ID,
		switchDate:   timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(150),
		pastSwitchAt: timezone.NewDate(2026, 8, 24).AddDays(-1),
	}
}

// testCareOfferingCatalog composes the Care Plan catalog over the test
// database, as the server binds it.
func testCareOfferingCatalog(t *testing.T, db *bun.DB, options ...testutil.CareOfferingCatalogOption) careplan.CareOfferingCatalogCapability {
	t.Helper()
	return testutil.NewCareOfferingCatalog(t, db, options...).Catalog
}
