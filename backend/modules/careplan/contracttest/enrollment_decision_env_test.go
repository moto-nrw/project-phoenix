package contracttest_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The enrollment decision fixture the Care Plan suites drive their bookings
// through (moved from services/enrollment, #3565): a tenant with a form
// schema and a source phase, the intake, the decision flow and Care Plan's
// booking materialization, composed the way the root binds them.

// enrollmentKeys names the tenant setting keys the settings doubles answer.
var enrollmentKeys = api.CarePlanContractKeys

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

const decisionTestToday calendar.Date = "2026-08-24"

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
	tb              testing.TB
	db              *bun.DB
	repos           *api.RepositoryTestFactory
	reposEnrollment *api.EnrollmentSuiteRepositories
	reposPeople     *api.PeopleRepositorySuiteFactory
	reposSchedules  *api.CareScheduleSuiteRepositories
	reposPresence   *api.PresenceSuiteRepositories
	// timetable are the retained Timetable adapters over the same owners.
	timetable owners.TimetableRepositories
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

func setupDecisionTestWithSettings(t *testing.T, settings owners.EnrollmentDecisionSettings) (*decisionTestEnv, func()) {
	t.Helper()
	return setupEnrollmentTestEnv(t, settings)
}

func setupEnrollmentTestEnv(t *testing.T, decisionSettings owners.EnrollmentDecisionSettings) (*decisionTestEnv, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))

	repoFactory := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	repoFactoryEnrollment := api.NewEnrollmentSuiteRepositories(testpkg.NewDatabase(t, db))
	repoFactoryPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	repoFactorySchedules := api.NewCareScheduleSuiteRepositories(testpkg.NewDatabase(t, db))
	repoFactoryPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
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

	schools := newFactorySchools(t, db)
	offerings := enrollmentTest.NewCareOfferingRecords(repoFactory.CarePlan())
	intake := owners.NewEnrollmentIntake(owners.EnrollmentIntakeSources{
		Bookings:         api.NewEnrollmentCareBookingCommands(carePlanTest.NewOfferingBookings()),
		Requests:         repoFactoryEnrollment.Enrollment(),
		Children:         repoFactoryEnrollment.Enrollment(),
		CareOfferingRepo: offerings,
		Capacity:         owners.NewEnrollmentOfferingCapacity(offerings, repoFactoryEnrollment.Enrollment(), settings),
		Catalog:          repoFactoryEnrollment.Enrollment(),
		SchoolRepo:       schools,
		Notifications:    testNotifications(repoFactoryEnrollment.Enrollment(), settings, schools),
		RateLimitRepo:    repoFactoryEnrollment.Enrollment(),
		LateInviteRepo:   repoFactoryEnrollment.Enrollment(),
		OutboxEnqueuer:   &recordingMailOutbox{},
		Settings:         settings,
		FrontendURL:      "http://localhost:3000",
		Logger:           slog.Default(),
	})

	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Rollover", "Tester")
	schemaSvc := enrollmentTest.NewFormSchemas(repoFactoryEnrollment.Enrollment(), nil, slog.Default())
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Rollover", []enrollmentTest.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentTest.FormFieldText, SortOrder: 0},
	}, account.ID)
	require.NoError(t, err)

	sourcePhase := &enrollmentTest.Phase{
		Name:             "rollover-source-" + t.Name(),
		Kind:             enrollmentTest.PhaseKindSchoolYear,
		ServiceStartDate: enrollmentTest.Date(calendar.NewDate(2026, 9, 1)),
		ServiceEndDate:   enrollmentTest.Date(calendar.NewDate(2027, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentTest.PhaseCareOverflowWaitlist,
		FormSchemaID:     &schema.ID,
	}
	sourcePhase.TenantID = testpkg.Tenant(t)
	require.NoError(t, repoFactoryEnrollment.Enrollment().InsertPhase(ctx, sourcePhase))

	catalog := testCareOfferingCatalog(t, db, api.WithCareOfferingSettings(settings))
	bookings := composeBookings(t, db, catalog, decisionSettings, nil, nil)
	decisions := composeDecisions(t, db, repoFactoryEnrollment, repoFactory, repoFactoryPeople, repoFactorySchedules, bookings, decisionSettings, nil, nil)
	env := &decisionTestEnv{
		tb:              t,
		db:              db,
		repos:           repoFactory,
		reposEnrollment: repoFactoryEnrollment,
		reposPeople:     repoFactoryPeople,
		reposSchedules:  repoFactorySchedules,
		reposPresence:   repoFactoryPresence,
		timetable:       api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(db)),
		requestSvc:      intake,
		offeringCatalog: catalog,
		settings:        settings,
		sourcePhase:     sourcePhase,
		creatorID:       account.ID,
		decisions:       decisions,
		decision:        enrollmentTest.NewPublicDecisions(decisions),
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
	settings owners.EnrollmentDecisionSettings,
	lockTemplateRecurrence func(context.Context) error,
	careWithdrawal enrollmentTest.CareWithdrawalReconciler,
) enrollmentTest.PublicDecisions {
	if careWithdrawal == nil {
		careWithdrawal = newTestCareLifecycle(env.db, api.CareLifecycleTestConfig{
			BookingsAuthoritative: testBookingsAuthority(settings),
		})
	}
	bookings := composeBookings(env.tb, env.db, env.offeringCatalog, settings, lockTemplateRecurrence, careWithdrawal)
	return enrollmentTest.NewPublicDecisions(composeDecisions(env.tb, env.db, env.reposEnrollment, env.repos, env.reposPeople, env.reposSchedules, bookings, settings, lockTemplateRecurrence, careWithdrawal))
}

// newBookingsForTest composes Care Plan's booking materialization (#3560)
// with the collaborators the decision suites hand the decision flow: the
// env's catalog, the settings, the recurrence lock, the withdrawal follow-up
// and the fixed decision day.
func newBookingsForTest(
	env *decisionTestEnv,
	settings owners.EnrollmentDecisionSettings,
	lockTemplateRecurrence func(context.Context) error,
	careWithdrawal enrollmentTest.CareWithdrawalReconciler,
) careplan.BookingMaterializationCapability {
	return composeBookings(env.tb, env.db, env.offeringCatalog, settings, lockTemplateRecurrence, careWithdrawal)
}

func composeBookings(
	tb testing.TB,
	db *bun.DB,
	catalog careplan.CareOfferingCatalogCapability,
	settings owners.EnrollmentDecisionSettings,
	lockTemplateRecurrence func(context.Context) error,
	careWithdrawal enrollmentTest.CareWithdrawalReconciler,
) careplan.BookingMaterializationCapability {
	if careWithdrawal == nil {
		careWithdrawal = newTestCareLifecycle(db, api.CareLifecycleTestConfig{
			BookingsAuthoritative: testBookingsAuthority(settings),
		})
	}
	var bookingSettings owners.EnrollmentDecisionSettings = registryDefaultDecisionSettings{}
	if settings != nil {
		bookingSettings = settings
	}
	options := []api.BookingMaterializationOption{
		api.WithBookingCatalog(catalog),
		api.WithBookingToday(func() calendar.Date { return decisionTestToday }),
		api.WithBookingSettings(bookingSettings),
		api.WithBookingWithdrawals(careWithdrawal),
	}
	if lockTemplateRecurrence != nil {
		options = append(options, api.WithBookingRecurrenceLock(lockTemplateRecurrence))
	}
	return newBookingMaterialization(tb, db, options...).Bookings
}

// composeDecisions composes the decision flow the suites drive, the way the
// root binds it.
func composeDecisions(
	tb testing.TB,
	db *bun.DB,
	repoFactory *api.EnrollmentSuiteRepositories, repoFactoryCare *api.RepositoryTestFactory, repoFactoryPeople *api.PeopleRepositorySuiteFactory, repoFactorySchedules *api.CareScheduleSuiteRepositories,
	bookings enrollmentTest.DecisionBookings,
	settings owners.EnrollmentDecisionSettings,
	lockTemplateRecurrence func(context.Context) error,
	careWithdrawal enrollmentTest.CareWithdrawalReconciler,
) *enrollmentTest.Decisions {
	if careWithdrawal == nil {
		careWithdrawal = newTestCareLifecycle(db, api.CareLifecycleTestConfig{
			BookingsAuthoritative: testBookingsAuthority(settings),
		})
	}
	guardianAccess, err := api.NewCarePlanContractGuardianAccess(testpkg.NewDatabase(tb, db))
	require.NoError(tb, err)
	students, err := owners.NewPeopleDirectory(testpkg.MustDatabase(db))
	require.NoError(tb, err)
	audits := owners.NewTestAuditPersistence(testpkg.NewDatabase(tb, db), nil)
	return owners.NewEnrollmentDecisions(api.EnrollmentDecisionDependencies{
		Requests:       repoFactory.Enrollment(),
		Children:       repoFactory.Enrollment(),
		Guardians:      repoFactory.Enrollment(),
		LateInvites:    repoFactory.Enrollment(),
		Phases:         repoFactory.Enrollment(),
		Schemas:        repoFactory.Enrollment(),
		Offerings:      enrollmentTest.NewCareOfferingRecords(repoFactoryCare.CarePlan()),
		Notifications:  testNotifications(repoFactory.Enrollment(), settings, nil),
		Bookings:       bookings,
		GuardianAccess: guardianAccess,
		People: owners.EnrollmentDecisionPeopleDirectory(
			repoFactoryPeople.Person, repoFactoryPeople.Staff, repoFactoryPeople.Student, repoFactoryPeople.StudentGuardian, repoFactoryPeople.GuardianProfile, repoFactoryPeople.GuardianPhoneNumber, students,
		),
		StudentEnrollment:      students,
		Companions:             owners.EnrollmentDecisionCompanions(owners.NewStudentCompanionRepository(repoFactoryCare.CarePlan()), repoFactoryCare.CarePlan().DeleteCompanionEdges),
		PickupSchedules:        timetableCompose.NewEnrollmentPickupSchedules(repoFactorySchedules.StudentPickupSchedule),
		ArrivalSchedules:       timetableCompose.NewEnrollmentArrivalSchedules(repoFactorySchedules.StudentArrivalSchedule),
		Adjustments:            owners.EnrollmentDecisionAdjustments(audits.EnrollmentOfferingAdjustment),
		Restorations:           owners.EnrollmentDecisionRestorations(audits.EnrollmentRestorationAudit),
		StudentAudit:           owners.EnrollmentDecisionStudentAudit(api.NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, api.NewStudentAudit(testpkg.NewDatabase(tb, db)))),
		CareWithdrawal:         careWithdrawal,
		ParentsURL:             owners.EnrollmentDecisionParentsURL("http://parents.localhost:3000", "http://localhost:3000"),
		Settings:               owners.EnrollmentDecisionSettingValues(settings),
		LockTemplateRecurrence: lockTemplateRecurrence,
		Logger:                 slog.Default(),
		Today:                  func() calendar.Date { return decisionTestToday },
	})
}

func testBookingsAuthority(settings owners.EnrollmentDecisionSettings) func(context.Context) (bool, error) {
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
func newTestCareLifecycle(db *bun.DB, config api.CareLifecycleTestConfig) careplan.CareLifecycle {
	repos, err := api.NewCareLifecycleTestRepositories(testpkg.MustDatabase(db), nil)
	if err != nil {
		panic(err)
	}
	if config.Audit == nil {
		config.Audit = api.NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, api.NewStudentAudit(testpkg.MustDatabase(db)))
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
	schools := newFactorySchools(t, env.db)
	offerings := enrollmentTest.NewCareOfferingRecords(env.repos.CarePlan())
	return owners.NewEnrollmentChangeRequests(owners.EnrollmentChangeRequestSources{
		Bookings:            api.NewEnrollmentCareBookingCommands(carePlanTest.NewOfferingBookings()),
		Requests:            env.reposEnrollment.Enrollment(),
		Children:            env.reposEnrollment.Enrollment(),
		Guardians:           env.reposEnrollment.Enrollment(),
		LateInviteRepo:      env.reposEnrollment.Enrollment(),
		CareOfferingRepo:    offerings,
		Capacity:            owners.NewEnrollmentOfferingCapacity(offerings, env.reposEnrollment.Enrollment(), env.settings),
		Catalog:             env.reposEnrollment.Enrollment(),
		Notifications:       testNotifications(env.reposEnrollment.Enrollment(), env.settings, schools),
		GuardianProfileRepo: env.reposPeople.GuardianProfile,
		GuardianPhoneRepo:   env.reposPeople.GuardianPhoneNumber,
		StudentRepo:         env.reposPeople.Student,
		GuardianAuthorizer:  env.reposPeople.StudentGuardian,
		Decisions:           env.decisions,
		BookingGates:        env.bookings,
		Settings:            env.settings,
		OutboxEnqueuer:      &recordingMailOutbox{},
		FrontendURL:         "http://localhost:3000",
		ParentsURL:          "http://parents.localhost:3000",
		Logger:              slog.Default(),
	})
}

// factorySchools reads the seeded schools through an Organisation & Tenancy
// capability, the owner the serving root binds.
type factorySchools struct {
	find func(context.Context, int64) (*enrollmentTest.School, error)
}

func newFactorySchools(t *testing.T, db *bun.DB) factorySchools {
	t.Helper()
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return factorySchools{find: func(ctx context.Context, id int64) (*enrollmentTest.School, error) {
		school, err := organizations.FindSchool(ctx, id)
		if err != nil {
			return nil, err
		}
		return &enrollmentTest.School{
			Name: school.Name, Subdomain: school.Subdomain, Settings: school.Settings, Deleted: school.IsDeleted(),
		}, nil
	}}
}

func (s factorySchools) FindSchool(ctx context.Context, id int64) (*enrollmentTest.School, error) {
	return s.find(ctx, id)
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
	deps := enrollmentTest.NotificationDependencies{Branding: owners.EnrollmentMailBranding,
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
	require.NoError(t, env.reposEnrollment.Enrollment().UpdatePhase(testpkg.Ctx(t), &phase))
}

func setSourcePhaseServiceStartDate(t *testing.T, env *decisionTestEnv, serviceStartDate calendar.Date) {
	t.Helper()
	env.sourcePhase.ServiceStartDate = enrollmentTest.Date(serviceStartDate)
	if !calendar.Date(env.sourcePhase.ServiceEndDate).After(serviceStartDate) {
		env.sourcePhase.ServiceEndDate = enrollmentTest.Date(calendar.NewDate(serviceStartDate.Year(), serviceStartDate.Month()+10, serviceStartDate.Day()))
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
			DateOfBirth:      calendar.NewDate(2018, 4, 15),
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
	require.NoError(t, env.reposEnrollment.Enrollment().UpdateMatchedStudent(testpkg.Ctx(t), childID, &studentID))
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
	children, err := env.reposEnrollment.Enrollment().ChildrenForRequest(ctx, requestID, false)
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

func listStudentEnrollmentRowsForDecisionTest(t *testing.T, env *decisionTestEnv, studentID int64) []timetable.StudentEnrollment {
	t.Helper()
	var rows []timetable.StudentEnrollment
	require.NoError(t, env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"student_enrollment".student_id = ?`, studentID).
		OrderExpr(`"student_enrollment".id`).
		Scan(testpkg.Ctx(t)))
	return rows
}

func createAdjustmentCareOfferingWith(t *testing.T, env *decisionTestEnv, name string, mutate func(*offeringContract.CareOfferingRecord)) *offeringContract.CareOfferingRecord {
	t.Helper()
	offering := &offeringContract.CareOfferingRecord{
		PhaseID:        env.sourcePhase.ID,
		Name:           name,
		DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice,
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

type offeringStudentTestDirectory struct {
	students interface {
		FindByIDs(context.Context, []int64) (map[int64]*peopleValues.StudentState, error)
	}
}

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

func approvedOfferingTestProjection(repos *api.EnrollmentSuiteRepositories, reposPeople *api.PeopleRepositorySuiteFactory) *enrollmentTest.ApprovedOfferingProjection {
	return enrollmentTest.NewApprovedOfferingProjection(repos.Enrollment(), offeringStudentTestDirectory{reposPeople.Student})
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
