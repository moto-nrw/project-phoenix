package contracttest_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The enrollment decision fixture the Care Plan suites drive their bookings
// through (moved from services/enrollment, #3565): a tenant with a form
// schema and a source phase, the intake, the decision flow and Care Plan's
// booking materialization, composed the way the root binds them.

// enrollmentKeys names the tenant setting keys the settings doubles answer.
var enrollmentKeys = testutil.EnrollmentSettingKeys

// stubRequestSettings is a minimal settings resolver. Tests set values
// directly via the maps; HasTenantOverride is true for any key present in
// one of them, mirroring the "DB override → registry default" precedence
// without the real settings registry.
type stubRequestSettings struct {
	mu           sync.Mutex
	stringValues map[string]string
	boolValues   map[string]bool
	intValues    map[string]int
}

func newStubRequestSettings() *stubRequestSettings {
	return &stubRequestSettings{
		stringValues: make(map[string]string),
		boolValues:   make(map[string]bool),
		intValues:    make(map[string]int),
	}
}

func (s *stubRequestSettings) HasTenantOverride(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.stringValues[key]; ok {
		return true, nil
	}
	if _, ok := s.boolValues[key]; ok {
		return true, nil
	}
	if _, ok := s.intValues[key]; ok {
		return true, nil
	}
	return false, nil
}

func (s *stubRequestSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.boolValues[key], nil
}

func (s *stubRequestSettings) ResolveString(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stringValues[key], nil
}

func (s *stubRequestSettings) ResolveInt(_ context.Context, key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.intValues[key], nil
}

// stubActivationSettings is a fake decision settings resolver returning a
// fixed enrollment.default_activation_mode, so the immediate/scheduled
// approval paths can be exercised without writing config.setting_values
// rows. Any other key resolves to "" (registry-default behaviour).
type stubActivationSettings struct {
	mode                  string
	notificationMode      string
	careOfferingsEnabled  *bool
	bookingsAuthoritative *bool
}

const decisionTestToday timezone.Date = "2026-08-24"

func (s stubActivationSettings) ResolveString(_ context.Context, key string) (string, error) {
	if key == enrollmentKeys.DefaultActivationMode {
		return s.mode, nil
	}
	if key == enrollmentKeys.NotifyPerDecision {
		if s.notificationMode != "" {
			return s.notificationMode, nil
		}
		return enrollmentKeys.NotifyPerDecisionImmediate, nil
	}
	return "", nil
}

func (s stubActivationSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	if key == enrollmentKeys.CareOfferingsEnabled && s.careOfferingsEnabled != nil {
		return *s.careOfferingsEnabled, nil
	}
	if key == enrollmentKeys.BookingsAuthoritative {
		return s.bookingsAuthoritative != nil && *s.bookingsAuthoritative, nil
	}
	return true, nil
}

// registryDefaultDecisionSettings answers the registry defaults a decision
// flow without settings always applied: care offerings on, bookings not
// authoritative.
type registryDefaultDecisionSettings struct{}

func (registryDefaultDecisionSettings) ResolveString(context.Context, string) (string, error) {
	return "", nil
}

func (registryDefaultDecisionSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	return key == enrollmentKeys.CareOfferingsEnabled, nil
}

// decisionTestEnv bundles the tenant, the source phase and the composed
// flows the suites drive.
type decisionTestEnv struct {
	// tb composes the Care Plan collaborators the suites build on this env
	// over the test tenant.
	tb    testing.TB
	db    *bun.DB
	repos *repositories.Factory
	// requestSvc is the parent-facing intake.
	requestSvc *enrollmentTest.Intake
	// offeringCatalog is the Care Plan catalog over the test database; its
	// link rules feed the decision flows the suites build on this env.
	offeringCatalog careplan.CareOfferingCatalogCapability
	settings        *stubRequestSettings
	sourcePhase     *enrollmentTest.Phase
	creatorID       int64
	// decisions is the composed decision flow; decision hands it out with
	// the public refusal values, the way the root hands it to the routes.
	decisions *enrollmentTest.Decisions
	decision  enrollmentTest.PublicDecisions
	// bookings is Care Plan's booking materialization the decision drives
	// (#3560): the roster resyncs, the offering-source editor and the pickup
	// reset the suites call directly.
	bookings careplan.BookingMaterializationCapability
}

func setupDecisionTest(t *testing.T) (*decisionTestEnv, func()) {
	// nil Settings exercises the safe default (scheduled).
	return setupDecisionTestWithSettings(t, nil)
}

func setupDecisionTestWithSettings(t *testing.T, settings testutil.EnrollmentDecisionSettings) (*decisionTestEnv, func()) {
	t.Helper()
	return setupEnrollmentTestEnv(t, settings)
}

func setupEnrollmentTestEnv(t *testing.T, decisionSettings testutil.EnrollmentDecisionSettings) (*decisionTestEnv, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))

	timetableDeps := repositories.NewUnobservedTimetableDependencies(db)
	repoFactory := repositories.NewFactory(db, timetableDeps)
	settings := newStubRequestSettings()
	settings.boolValues[enrollmentKeys.Enabled] = true
	settings.boolValues[enrollmentKeys.AllowSubmissionEdit] = true
	settings.boolValues[enrollmentKeys.CollectGradeLevel] = true
	settings.boolValues[enrollmentKeys.CareOfferingsEnabled] = true
	settings.boolValues[enrollmentKeys.WaitlistEnabled] = true
	settings.stringValues[enrollmentKeys.DuplicateHandling] = enrollmentKeys.DuplicateHandlingWarn
	settings.intValues[enrollmentKeys.GradeLevelMax] = 4
	settings.intValues[enrollmentKeys.StatusTokenTTLDays] = 365
	// The shared env submits all four standard consent flags. Configure the
	// matching legal texts so the resolved block contract declares them.
	// Submit persists only consent keys the contract knows, so an
	// unconfigured tenant would silently drop these flags.
	settings.boolValues[enrollmentKeys.LegalTermsEnabled] = true
	settings.boolValues[enrollmentKeys.LegalDSGVOEnabled] = true
	settings.boolValues[enrollmentKeys.LegalEmailContactEnabled] = true
	settings.boolValues[enrollmentKeys.LegalPhotoEnabled] = true
	settings.stringValues[enrollmentKeys.LegalAGBText] = "AGB Text"
	settings.stringValues[enrollmentKeys.LegalDSGVOText] = "DSGVO Text"
	settings.stringValues[enrollmentKeys.LegalEmailContactText] = "E-Mail Text"
	settings.stringValues[enrollmentKeys.LegalPhotoText] = "Foto Text"

	schools := factorySchools{repos: repoFactory}
	offerings := testutil.NewEnrollmentCareOfferingRecords(repoFactory.CarePlan())
	intake := testutil.NewEnrollmentIntake(testutil.EnrollmentIntakeSources{
		Bookings:         testutil.NewEnrollmentCareBookingCommands(carePlanTest.NewOfferingBookings()),
		Requests:         repoFactory.Enrollment(),
		Children:         repoFactory.Enrollment(),
		CareOfferingRepo: offerings,
		Capacity:         testutil.NewEnrollmentOfferingCapacity(offerings, repoFactory.Enrollment(), settings),
		Catalog:          repoFactory.Enrollment(),
		SchoolRepo:       schools,
		Notifications:    testNotifications(repoFactory.Enrollment(), settings, schools),
		RateLimitRepo:    repoFactory.Enrollment(),
		LateInviteRepo:   repoFactory.Enrollment(),
		OutboxEnqueuer:   testpkg.NewCapturingOutbox(),
		Settings:         settings,
		FrontendURL:      "http://localhost:3000",
		Logger:           slog.Default(),
	})

	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Rollover", "Tester")
	schemaSvc := enrollmentTest.NewFormSchemas(repoFactory.Enrollment(), nil, slog.Default())
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Rollover", []enrollmentTest.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentTest.FormFieldText, SortOrder: 0},
	}, account.ID)
	require.NoError(t, err)

	sourcePhase := &enrollmentTest.Phase{
		Name:             "rollover-source-" + t.Name(),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: enrollmentTest.Date(timezone.NewDate(2026, 9, 1)),
		ServiceEndDate:   enrollmentTest.Date(timezone.NewDate(2027, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
		FormSchemaID:     &schema.ID,
	}
	sourcePhase.TenantID = testpkg.Tenant(t)
	require.NoError(t, repoFactory.Enrollment().InsertPhase(ctx, sourcePhase))

	catalog := testCareOfferingCatalog(t, db, testutil.WithCareOfferingSettings(settings))
	bookings := composeBookings(t, db, catalog, decisionSettings, nil, nil)
	decisions := composeDecisions(t, db, repoFactory, bookings, decisionSettings, nil, nil)
	env := &decisionTestEnv{
		tb:              t,
		db:              db,
		repos:           repoFactory,
		requestSvc:      intake,
		offeringCatalog: catalog,
		settings:        settings,
		sourcePhase:     sourcePhase,
		creatorID:       account.ID,
		decisions:       decisions,
		decision:        testutil.PublicEnrollmentDecisions(decisions),
		bookings:        bookings,
	}
	return env, func() { cleanupEnrollmentTestEnv(db, testpkg.Tenant(t), sourcePhase.ID, account.ID) }
}

func cleanupEnrollmentTestEnv(db *bun.DB, tenantID, phaseID, accountID int64) {
	bg := context.Background()
	_, _ = db.NewRaw(`
		WITH phase_scope AS (
			SELECT id
			FROM enrollment.phases
			WHERE tenant_id = ?
			  AND (id = ? OR rollover_source_phase_id = ?)
		)
		DELETE FROM enrollment.care_offering_bookings rco
		USING enrollment.request_children rc, enrollment.requests r, phase_scope ps
		WHERE rco.request_child_id = rc.id
		  AND rc.request_id = r.id
		  AND r.phase_id = ps.id
	`, tenantID, phaseID, phaseID).Exec(bg)
	_, _ = db.NewRaw(`
		WITH phase_scope AS (
			SELECT id
			FROM enrollment.phases
			WHERE tenant_id = ?
			  AND (id = ? OR rollover_source_phase_id = ?)
		)
		DELETE FROM enrollment.request_children rc
		USING enrollment.requests r, phase_scope ps
		WHERE rc.request_id = r.id
		  AND r.phase_id = ps.id
	`, tenantID, phaseID, phaseID).Exec(bg)
	_, _ = db.NewRaw(`
		DELETE FROM enrollment.requests r
		USING enrollment.phases p
		WHERE r.phase_id = p.id
		  AND p.tenant_id = ?
		  AND (p.id = ? OR p.rollover_source_phase_id = ?)
	`, tenantID, phaseID, phaseID).Exec(bg)
	_, _ = db.NewDelete().TableExpr("enrollment.care_offerings").
		Where("phase_id IN (SELECT id FROM enrollment.phases WHERE tenant_id = ? AND (id = ? OR rollover_source_phase_id = ?))",
			tenantID, phaseID, phaseID).Exec(bg)
	_, _ = db.NewDelete().TableExpr("enrollment.submission_rate_limits").
		Where("tenant_id = ? AND key_value LIKE ?", tenantID, "%@example.com").Exec(bg)
	_, _ = db.NewDelete().TableExpr("enrollment.phases").
		Where("tenant_id = ? AND (id = ? OR rollover_source_phase_id = ?)", tenantID, phaseID, phaseID).Exec(bg)
	_, _ = db.NewDelete().TableExpr("enrollment.form_schemas").Where("created_by = ?", accountID).Exec(bg)
	_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", accountID).Exec(bg)
}

// newDecisionServiceForTestWithCareWithdrawal composes a decision flow over
// the env with its own booking materialization, recurrence lock and
// withdrawal follow-up.
func newDecisionServiceForTestWithCareWithdrawal(
	env *decisionTestEnv,
	settings testutil.EnrollmentDecisionSettings,
	lockTemplateRecurrence func(context.Context) error,
	careWithdrawal enrollmentTest.CareWithdrawalReconciler,
) enrollmentTest.PublicDecisions {
	if careWithdrawal == nil {
		careWithdrawal = newTestCareLifecycle(env.db, repositories.CareLifecycleTestConfig{
			BookingsAuthoritative: testBookingsAuthority(settings),
		})
	}
	bookings := composeBookings(env.tb, env.db, env.offeringCatalog, settings, lockTemplateRecurrence, careWithdrawal)
	return testutil.PublicEnrollmentDecisions(composeDecisions(env.tb, env.db, env.repos, bookings, settings, lockTemplateRecurrence, careWithdrawal))
}

// newBookingsForTest composes Care Plan's booking materialization (#3560)
// with the collaborators the decision suites hand the decision flow: the
// env's catalog, the settings, the recurrence lock, the withdrawal follow-up
// and the fixed decision day.
func newBookingsForTest(
	env *decisionTestEnv,
	settings testutil.EnrollmentDecisionSettings,
	lockTemplateRecurrence func(context.Context) error,
	careWithdrawal enrollmentTest.CareWithdrawalReconciler,
) careplan.BookingMaterializationCapability {
	return composeBookings(env.tb, env.db, env.offeringCatalog, settings, lockTemplateRecurrence, careWithdrawal)
}

func composeBookings(
	tb testing.TB,
	db *bun.DB,
	catalog careplan.CareOfferingCatalogCapability,
	settings testutil.EnrollmentDecisionSettings,
	lockTemplateRecurrence func(context.Context) error,
	careWithdrawal enrollmentTest.CareWithdrawalReconciler,
) careplan.BookingMaterializationCapability {
	if careWithdrawal == nil {
		careWithdrawal = newTestCareLifecycle(db, repositories.CareLifecycleTestConfig{
			BookingsAuthoritative: testBookingsAuthority(settings),
		})
	}
	var bookingSettings testutil.EnrollmentDecisionSettings = registryDefaultDecisionSettings{}
	if settings != nil {
		bookingSettings = settings
	}
	options := []testutil.BookingMaterializationOption{
		testutil.WithBookingCatalog(catalog),
		testutil.WithBookingToday(func() timezone.Date { return decisionTestToday }),
		testutil.WithBookingSettings(bookingSettings),
		testutil.WithBookingWithdrawals(careWithdrawal),
	}
	if lockTemplateRecurrence != nil {
		options = append(options, testutil.WithBookingRecurrenceLock(lockTemplateRecurrence))
	}
	return testutil.NewBookingMaterialization(tb, db, options...).Bookings
}

// composeDecisions composes the decision flow the suites drive, the way the
// root binds it.
func composeDecisions(
	tb testing.TB,
	db *bun.DB,
	repoFactory *repositories.Factory,
	bookings enrollmentTest.DecisionBookings,
	settings testutil.EnrollmentDecisionSettings,
	lockTemplateRecurrence func(context.Context) error,
	careWithdrawal enrollmentTest.CareWithdrawalReconciler,
) *enrollmentTest.Decisions {
	if careWithdrawal == nil {
		careWithdrawal = newTestCareLifecycle(db, repositories.CareLifecycleTestConfig{
			BookingsAuthoritative: testBookingsAuthority(settings),
		})
	}
	guardianAccess, err := testutil.NewEnrollmentGuardianAccess(db)
	require.NoError(tb, err)
	students, err := repositories.NewPeopleDirectory(db)
	require.NoError(tb, err)
	return testutil.NewEnrollmentDecisions(testutil.EnrollmentDecisionSources{
		Requests:               repoFactory.Enrollment(),
		Children:               repoFactory.Enrollment(),
		Guardians:              repoFactory.Enrollment(),
		LateInvites:            repoFactory.Enrollment(),
		CareOfferings:          testutil.NewEnrollmentCareOfferingRecords(repoFactory.CarePlan()),
		Phases:                 repoFactory.Enrollment(),
		Schemas:                repoFactory.Enrollment(),
		OfferingAdjustments:    repoFactory.EnrollmentOfferingAdjustment,
		Restorations:           repoFactory.EnrollmentRestorationAudit,
		Notifications:          testNotifications(repoFactory.Enrollment(), settings, nil),
		Persons:                repoFactory.Person,
		Staff:                  repoFactory.Staff,
		Students:               repoFactory.Student,
		StudentGuardians:       repoFactory.StudentGuardian,
		GuardianProfiles:       repoFactory.GuardianProfile,
		GuardianPhones:         repoFactory.GuardianPhoneNumber,
		PickupSchedules:        repositories.NewEnrollmentPickupSchedules(repoFactory.StudentPickupSchedule),
		ArrivalSchedules:       repositories.NewEnrollmentArrivalSchedules(repoFactory.StudentArrivalSchedule),
		CareBookings:           bookings,
		GuardianAccess:         guardianAccess,
		StudentEnrollment:      students,
		Companions:             repositories.NewStudentCompanionRepository(repoFactory.CarePlan()),
		DeleteCompanions:       repoFactory.CarePlan().DeleteCompanionEdges,
		StudentAudit:           usersService.NewStudentAuditService(testpkg.RequestAuditActor, repositories.NewStudentAudit(db)),
		CareWithdrawal:         careWithdrawal,
		FrontendURL:            "http://localhost:3000",
		ParentsURL:             "http://parents.localhost:3000",
		Settings:               settings,
		LockTemplateRecurrence: lockTemplateRecurrence,
		Logger:                 slog.Default(),
		Today:                  func() timezone.Date { return decisionTestToday },
	})
}

func testBookingsAuthority(settings testutil.EnrollmentDecisionSettings) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		if settings == nil {
			return false, nil
		}
		return settings.ResolveBool(ctx, enrollmentKeys.BookingsAuthoritative)
	}
}

// newTestCareLifecycle composes Care Plan's lifecycle over the test database,
// with its owners bound the way the production graph binds them. The care-end
// history goes to the ordinary student audit unless the config names another.
func newTestCareLifecycle(db *bun.DB, config repositories.CareLifecycleTestConfig) careplan.CareLifecycle {
	repos, err := repositories.NewCareLifecycleTestRepositories(db, nil)
	if err != nil {
		panic(err)
	}
	if config.Audit == nil {
		config.Audit = usersService.NewStudentAuditService(testpkg.RequestAuditActor, repositories.NewStudentAudit(db))
	}
	lifecycle, err := repos.NewCareLifecycle(config)
	if err != nil {
		panic(err)
	}
	return lifecycle
}

// changeRequestApplierForTest applies change requests through the env's
// decision flow, the way the root binds the change requests' applier.
func changeRequestApplierForTest(t *testing.T, env *decisionTestEnv) enrollmentTest.ApprovedChildChanges {
	t.Helper()
	return env.decisions
}

// newChangeRequestServiceWithDecisionForTest composes the change requests
// over the env's intake repositories and decision flow.
func newChangeRequestServiceWithDecisionForTest(t *testing.T, env *decisionTestEnv) enrollmentTest.ChangeRequests {
	t.Helper()
	schools := factorySchools{repos: env.repos}
	offerings := testutil.NewEnrollmentCareOfferingRecords(env.repos.CarePlan())
	return testutil.NewEnrollmentChangeRequests(testutil.EnrollmentChangeRequestSources{
		Bookings:            testutil.NewEnrollmentCareBookingCommands(carePlanTest.NewOfferingBookings()),
		Requests:            env.repos.Enrollment(),
		Children:            env.repos.Enrollment(),
		Guardians:           env.repos.Enrollment(),
		LateInviteRepo:      env.repos.Enrollment(),
		CareOfferingRepo:    offerings,
		Capacity:            testutil.NewEnrollmentOfferingCapacity(offerings, env.repos.Enrollment(), env.settings),
		Catalog:             env.repos.Enrollment(),
		Notifications:       testNotifications(env.repos.Enrollment(), env.settings, schools),
		GuardianProfileRepo: env.repos.GuardianProfile,
		GuardianPhoneRepo:   env.repos.GuardianPhoneNumber,
		StudentRepo:         env.repos.Student,
		GuardianAuthorizer:  env.repos.StudentGuardian,
		Decisions:           env.decisions,
		BookingGates:        env.bookings,
		Settings:            env.settings,
		OutboxEnqueuer:      testpkg.NewCapturingOutbox(),
		FrontendURL:         "http://localhost:3000",
		ParentsURL:          "http://parents.localhost:3000",
		Logger:              slog.Default(),
	})
}

// factorySchools reads the seeded schools through the repository factory's
// Organisation & Tenancy capability, the owner the serving root binds.
type factorySchools struct{ repos *repositories.Factory }

func (s factorySchools) FindSchool(ctx context.Context, id int64) (*enrollmentTest.School, error) {
	school, err := s.repos.School.FindSchool(ctx, id)
	if err != nil {
		return nil, err
	}
	return &enrollmentTest.School{
		Name: school.Name, Subdomain: school.Subdomain, Settings: school.Settings, Deleted: school.IsDeleted(),
	}, nil
}

// recordingMailOutbox records Enrollment's parent mails.
type recordingMailOutbox struct {
	mu    sync.Mutex
	mails []enrollmentTest.Mail
}

func (o *recordingMailOutbox) EnqueueMail(_ context.Context, mail enrollmentTest.Mail) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.mails = append(o.mails, mail)
	return nil
}

// notificationModeSettings reads the decision notification mode from a
// suite's settings double.
type notificationModeSettings struct {
	settings interface {
		ResolveString(ctx context.Context, key string) (string, error)
	}
}

func (s notificationModeSettings) NotifyPerDecision(ctx context.Context) (string, error) {
	if s.settings == nil {
		return "", nil
	}
	return s.settings.ResolveString(ctx, enrollmentKeys.NotifyPerDecision)
}

// testNotifications composes Enrollment's parent mails for a flow under
// test: the owner pins the mode, the suite's settings choose it, and a
// recording outbox takes the mails.
func testNotifications(modes enrollmentTest.NotificationModePin, settings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}, schools interface {
	FindSchool(ctx context.Context, id int64) (*enrollmentTest.School, error)
}) enrollmentTest.Notifications {
	deps := enrollmentTest.NotificationDependencies{
		Modes: modes, Settings: notificationModeSettings{settings: settings}, Outbox: &recordingMailOutbox{},
	}
	if schools != nil {
		deps.Schools = schools
	}
	return enrollmentTest.NewNotifications(deps)
}

// updateSourcePhase writes the env's source phase back through the owner.
func updateSourcePhase(t *testing.T, env *decisionTestEnv) {
	t.Helper()
	phase := *env.sourcePhase
	require.NoError(t, env.repos.Enrollment().UpdatePhase(testpkg.Ctx(t), &phase))
}

func setSourcePhaseServiceStartDate(t *testing.T, env *decisionTestEnv, serviceStartDate timezone.Date) {
	t.Helper()
	env.sourcePhase.ServiceStartDate = enrollmentTest.Date(serviceStartDate)
	if !timezone.Date(env.sourcePhase.ServiceEndDate).After(serviceStartDate) {
		env.sourcePhase.ServiceEndDate = enrollmentTest.Date(timezone.NewDate(serviceStartDate.Year(), serviceStartDate.Month()+10, serviceStartDate.Day()))
	}
	updateSourcePhase(t, env)
}

// consentFlags are the four standard consent flags the env's legal texts
// declare.
func consentFlags(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"agb":             true,
		"data_processing": true,
		"email_contact":   true,
		"photo":           true,
	})
	require.NoError(t, err)
	return raw
}

// submitOneChild submits a single-child enrollment in the source phase and
// returns the resulting request + child ids so tests can drive Decide
// against a known row.
func submitOneChild(t *testing.T, env *decisionTestEnv, guardianEmail, childFirst, childLast string) (requestID, childID int64) {
	t.Helper()
	grade := int16(2)
	res, err := env.requestSvc.Submit(testpkg.Ctx(t), enrollmentTest.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Test",
		GuardianEmail:     guardianEmail,
		ConsentFlags:      consentFlags(t),
		Children: []enrollmentTest.SubmitChild{{
			FirstName:        childFirst,
			LastName:         childLast,
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
		}},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)
	return res.Request.ID, res.Children[0].ID
}

// matchChildToExistingStudent pins matched_student_id on a submitted child, the
// way an existing_students phase does at submission time, so approval takes the
// re-enrollment branch instead of creating a second Person + Student.
func matchChildToExistingStudent(t *testing.T, env *decisionTestEnv, childID, studentID int64) {
	t.Helper()
	require.NoError(t, env.repos.Enrollment().UpdateMatchedStudent(testpkg.Ctx(t), childID, &studentID))
}

// liftTakeoverStamp clears created_student_id on the request's children and
// returns the restore func. ADR 0003 blocks NEW change requests for a child
// that is already taken over into care; rows that were created before the
// takeover still have to be decidable. The tests build exactly that state:
// the change request is created while the stamp is lifted, then the stamp
// goes back on before the decision runs.
func liftTakeoverStamp(t *testing.T, env *decisionTestEnv, requestID int64) func() {
	t.Helper()
	ctx := testpkg.TenantContext(env.sourcePhase.TenantID)
	children, err := env.repos.Enrollment().ChildrenForRequest(ctx, requestID, false)
	require.NoError(t, err)
	stamps := make(map[int64]int64, len(children))
	for _, child := range children {
		if child.CreatedStudentID == nil {
			continue
		}
		stamps[child.ID] = *child.CreatedStudentID
	}
	if len(stamps) > 0 {
		_, err = env.db.NewUpdate().
			TableExpr("enrollment.request_children").
			Set("created_student_id = NULL").
			Where("request_id = ?", requestID).
			Exec(ctx)
		require.NoError(t, err)
	}
	return func() {
		for childID, studentID := range stamps {
			_, err := env.db.NewUpdate().
				TableExpr("enrollment.request_children").
				Set("created_student_id = ?", studentID).
				Where("id = ?", childID).
				Exec(ctx)
			require.NoError(t, err)
		}
	}
}

func listStudentEnrollmentRowsForDecisionTest(t *testing.T, env *decisionTestEnv, studentID int64) []activitiesModels.StudentEnrollment {
	t.Helper()
	var rows []activitiesModels.StudentEnrollment
	require.NoError(t, env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"student_enrollment".student_id = ?`, studentID).
		OrderExpr(`"student_enrollment".id`).
		Scan(testpkg.Ctx(t)))
	return rows
}

func createAdjustmentCareOfferingWith(t *testing.T, env *decisionTestEnv, name string, mutate func(*enrollmentModels.CareOffering)) *enrollmentModels.CareOffering {
	t.Helper()
	offering := &enrollmentModels.CareOffering{
		PhaseID:        env.sourcePhase.ID,
		Name:           name,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
		CountsAsCare:   false,
		SortOrder:      100,
	}
	if mutate != nil {
		mutate(offering)
	}
	require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Create(testpkg.Ctx(t), offering))
	return offering
}

type offeringStudentTestDirectory struct{ students usersModels.StudentRepository }

func (d offeringStudentTestDirectory) ListOfferingStudents(ctx context.Context, ids []int64) ([]enrollmentTest.OfferingStudent, error) {
	students, err := d.students.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]enrollmentTest.OfferingStudent, 0, len(students))
	for _, student := range students {
		rows = append(rows, enrollmentTest.OfferingStudent{ID: student.ID, SchoolClass: student.SchoolClass, Alumnus: student.IsAlumnus()})
	}
	return rows, nil
}

func approvedOfferingTestProjection(repos *repositories.Factory) *enrollmentTest.ApprovedOfferingProjection {
	return enrollmentTest.NewApprovedOfferingProjection(repos.Enrollment(), offeringStudentTestDirectory{repos.Student})
}

func wipePhases(db *bun.DB, tenantID int64, names ...string) {
	if len(names) == 0 {
		// No-op when no names — protects against accidental tenant-wide
		// wipes in shared-tenant tests.
		return
	}
	_, _ = db.NewDelete().
		TableExpr("enrollment.phases").
		Where("tenant_id = ? AND name IN (?)", tenantID, bun.List(names)).
		Exec(context.Background())
}

func wipeOfferings(db *bun.DB, tenantID, phaseID int64) {
	_, _ = db.NewDelete().
		TableExpr("enrollment.care_offerings").
		Where("tenant_id = ? AND phase_id = ?", tenantID, phaseID).
		Exec(context.Background())
}

func runInTenantTx(t *testing.T, db *bun.DB, tenantID int64, fn func(ctx context.Context) error) error {
	t.Helper()
	return tenant.WithTenantTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
}
