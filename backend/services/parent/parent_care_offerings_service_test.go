package parent

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type careOfferingsChildRepoStub struct {
	parentModels.ChildRepository
	child *parentModels.ChildSummary
	err   error
}

// careOfferingsStudentRepoStub supplies the locked, still-active child that
// CreateOfferingChangeRequest now requires before it delegates the write.
type careOfferingsStudentRepoStub struct {
	usersModels.StudentRepository
}

func (careOfferingsStudentRepoStub) FindByIDForUpdate(
	context.Context, int64,
) (*usersModels.Student, error) {
	return &usersModels.Student{}, nil
}

func (s careOfferingsChildRepoStub) FindForAccount(
	_ context.Context,
	_, _ int64,
) (*parentModels.ChildSummary, error) {
	return s.child, s.err
}

type carePeriodRepoStub struct {
	enrollmentSvc.StudentCarePeriodReader
	periods []*enrollmentSvc.StudentCarePeriod
	err     error
}

func (s carePeriodRepoStub) StudentCarePeriods(
	_ context.Context,
	_ int64,
) ([]*capability.StudentCarePeriod, error) {
	result := make([]*capability.StudentCarePeriod, 0, len(s.periods))
	for _, p := range s.periods {
		result = append(result, &capability.StudentCarePeriod{RequestChildID: p.RequestChildID, RequestID: p.RequestID, PhaseID: p.PhaseID, PhaseName: p.PhaseName, ServiceStartDate: capability.Date(p.ServiceStartDate.String()), ServiceEndDate: capability.Date(p.ServiceEndDate.String())})
	}
	return result, s.err
}

type childOfferingRepoStub struct {
	enrollmentSvc.OfferingHistoryReader
	links []*capability.RequestChildOffering
	err   error
}

type recordingChildOfferingRepoStub struct {
	enrollmentSvc.OfferingHistoryReader
	links        []*capability.RequestChildOffering
	dates        []timezone.Date
	historyCalls int
	err          error
}

func (s *recordingChildOfferingRepoStub) ListByRequestChildIDAtDate(
	_ context.Context,
	_ int64,
	onDate timezone.Date,
) ([]*capability.RequestChildOffering, error) {
	s.dates = append(s.dates, onDate)
	return s.links, s.err
}

func (s *recordingChildOfferingRepoStub) RequestChildOfferingHistory(
	_ context.Context,
	_ int64,
) ([]*capability.RequestChildOffering, error) {
	s.historyCalls++
	return s.links, s.err
}

func (s childOfferingRepoStub) ListByRequestChildID(
	_ context.Context,
	_ int64,
) ([]*capability.RequestChildOffering, error) {
	return s.links, s.err
}

func (s childOfferingRepoStub) ListByRequestChildIDAtDate(
	_ context.Context,
	_ int64,
	_ timezone.Date,
) ([]*capability.RequestChildOffering, error) {
	return s.links, s.err
}

func (s childOfferingRepoStub) RequestChildOfferingHistory(
	_ context.Context,
	_ int64,
) ([]*capability.RequestChildOffering, error) {
	return s.links, s.err
}

type careOfferingRepoStub struct {
	enrollmentModels.CareOfferingRepository
	offerings []*enrollmentModels.CareOffering
	err       error
}

func (s careOfferingRepoStub) ListByIDs(
	_ context.Context,
	_ []int64,
) ([]*enrollmentModels.CareOffering, error) {
	return s.offerings, s.err
}

type offeringChangesStub struct {
	enrollmentSvc.OfferingChangeRequestService
	catalog      *enrollmentSvc.OfferingChangeCatalog
	view         *enrollmentSvc.OfferingChangeView
	earliest     timezone.Date
	catalogErr   error
	viewErr      error
	earliestErr  error
	createErr    error
	withdrawErr  error
	createdInput enrollmentSvc.CreateOfferingChangeInput
	withdrawn    [3]int64
}

func (s *offeringChangesStub) Catalog(
	_ context.Context,
	_ int64,
) (*enrollmentSvc.OfferingChangeCatalog, error) {
	return s.catalog, s.catalogErr
}

func (s *offeringChangesStub) CatalogAt(
	_ context.Context,
	_ int64,
	_ timezone.Date,
) (*enrollmentSvc.OfferingChangeCatalog, error) {
	return s.catalog, s.catalogErr
}

func (s *offeringChangesStub) GetForStudent(
	_ context.Context,
	_ int64,
) (*enrollmentSvc.OfferingChangeView, error) {
	return s.view, s.viewErr
}

func (s *offeringChangesStub) Create(
	_ context.Context,
	input enrollmentSvc.CreateOfferingChangeInput,
) (*enrollmentModels.OfferingChangeRequest, error) {
	s.createdInput = input
	return &enrollmentModels.OfferingChangeRequest{}, s.createErr
}

func (s *offeringChangesStub) Withdraw(
	_ context.Context,
	requestID, accountID, studentID int64,
) error {
	s.withdrawn = [3]int64{requestID, accountID, studentID}
	return s.withdrawErr
}

func (s *offeringChangesStub) EarliestEffectiveFrom(
	_ context.Context,
) (timezone.Date, error) {
	return s.earliest, s.earliestErr
}

type offeringChangeSettingsStub struct {
	configSvc.SettingsService
	enabled              bool
	careOfferingsEnabled *bool
	err                  error
}

func boolPtr(value bool) *bool { return &value }

func (s offeringChangeSettingsStub) ResolveBoolForTenant(
	_ context.Context,
	_ int64,
	key string,
) (bool, error) {
	if key == configModel.KeyEnrollmentCareOfferingsEnabled && s.careOfferingsEnabled != nil {
		return *s.careOfferingsEnabled, s.err
	}
	return s.enabled, s.err
}

func careOfferingsTestDB(t *testing.T) *bun.DB {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	return db
}

func permittedCareOfferingsChild(tb testing.TB) *parentModels.ChildSummary {
	return &parentModels.ChildSummary{
		StudentID: 22,
		TenantID:  testpkg.Tenant(tb),
		GuardianPermissions: map[string]interface{}{
			authorize.GuardianPermissionPortalAccess:  true,
			authorize.GuardianPermissionRequestSubmit: true,
		},
	}
}

func careOfferingsService(
	db *bun.DB,
	child *parentModels.ChildSummary,
	changes *offeringChangesStub,
) *service {
	svc := &service{ServiceConfig: ServiceConfig{
		ChildRepo:   careOfferingsChildRepoStub{child: child},
		StudentRepo: careOfferingsStudentRepoStub{},
		DB:          db,
		Logger:      slog.Default(),
		Now: func() time.Time {
			return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		},
	}}
	if changes != nil {
		svc.OfferingChanges = changes
	}
	return svc
}

func TestGetChildCareOfferingsReturnsCompleteSortedView(t *testing.T) {
	t.Parallel()

	db := careOfferingsTestDB(t)
	today := timezone.NewDate(2026, 8, 24)
	description := "Mit Mittagessen"
	price := 4200
	sourceChildID := int64(301)
	futureStart := today.AddDays(15)
	note := "Ab Februar bitte dienstags"
	createdAt := time.Now().Add(-time.Hour)

	firstOffering := &enrollmentModels.CareOffering{
		ID:            41,
		Name:          "Zweite Sortierung",
		SortOrder:     20,
		IncludesLunch: true,
	}
	secondOffering := &enrollmentModels.CareOffering{
		ID:                  42,
		Name:                "Erste Sortierung",
		Description:         &description,
		SortOrder:           10,
		PriceCents:          &price,
		IncludesHolidayCare: true,
	}
	futureOffering := &enrollmentModels.CareOffering{
		ID:        43,
		Name:      "Zukünftige Betreuung",
		SortOrder: 30,
	}
	changes := &offeringChangesStub{
		earliest: today.AddDays(15),
		view: &enrollmentSvc.OfferingChangeView{
			Request: &enrollmentModels.OfferingChangeRequest{
				ID:            61,
				CreatedAt:     createdAt,
				EffectiveFrom: enrollmentModels.OfferingChangeDate(today.AddDays(20)),
				ParentNote:    &note,
				SubmittedBy:   11,
			},
			Diff: []enrollmentSvc.OfferingChangeDiffEntry{{
				Label:    "Erste Sortierung",
				OldState: "not_booked",
				NewState: "booked",
				NewDays:  []string{"tue"},
			}},
			LastDecision: &enrollmentSvc.OfferingChangeDecision{ID: 60, SubmittedBy: 11, Status: "rejected"},
		},
	}
	svc := careOfferingsService(db, permittedCareOfferingsChild(t), changes)
	svc.Settings = offeringChangeSettingsStub{enabled: true}
	svc.CarePeriods = carePeriodRepoStub{periods: []*enrollmentSvc.StudentCarePeriod{{
		RequestChildID:   sourceChildID,
		PhaseName:        "Schuljahr 2026/27",
		ServiceStartDate: today.AddDays(-30),
		ServiceEndDate:   today.AddDays(200),
	}}}
	ownerFutureStart := capability.Date(futureStart)
	svc.OfferingHistory = childOfferingRepoStub{links: []*capability.RequestChildOffering{
		nil,
		{CareOfferingID: 41, SelectedDays: []string{"fri", "mon", "fri", "bad"}},
		{CareOfferingID: 42, SelectedDays: []string{"tue"}},
		{CareOfferingID: 999},
		{CareOfferingID: 43, ValidFrom: &ownerFutureStart},
	}}
	svc.CareOfferingRepo = careOfferingRepoStub{offerings: []*enrollmentModels.CareOffering{
		firstOffering, nil, secondOffering, futureOffering,
	}}
	view, err := svc.GetChildCareOfferings(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22)
	require.NoError(t, err)

	assert.Equal(t, "Schuljahr 2026/27", view.PeriodName)
	require.Len(t, view.Offerings, 3)
	assert.Equal(t, int64(42), view.Offerings[0].OfferingID)
	assert.Equal(t, description, view.Offerings[0].Description)
	assert.Equal(t, []int{1, 5}, view.Offerings[1].Weekdays)
	assert.Equal(t, futureOffering.Name, view.Offerings[2].Name)
	assert.True(t, view.Offerings[2].StartsLater)
	require.NotNil(t, view.Offerings[2].ValidFrom)
	assert.Equal(t, futureStart, *view.Offerings[2].ValidFrom)
	assert.True(t, view.CanRequest)
	assert.Empty(t, view.ChangesDisabledReason)
	assert.Equal(t, changes.earliest, view.EarliestEffectiveFrom)
	require.NotNil(t, view.PendingRequest)
	assert.Equal(t, note, view.PendingRequest.Note)
	assert.True(t, view.PendingRequest.SubmittedBySelf)
	assert.Equal(t, createdAt, view.PendingRequest.CreatedAt)
	assert.Equal(t, changes.view.LastDecision, view.LastDecision)

	// A decided request has no open Request, but its result remains visible for
	// the recency window.
	changes.view.Request = nil
	view, err = svc.GetChildCareOfferings(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22)
	require.NoError(t, err)
	assert.Nil(t, view.PendingRequest)
	assert.Equal(t, changes.view.LastDecision, view.LastDecision)
}

func TestGetChildCareOfferingsWithoutEnrollmentStillReturnsEmptySlices(t *testing.T) {
	t.Parallel()

	db := careOfferingsTestDB(t)
	svc := careOfferingsService(db, permittedCareOfferingsChild(t), nil)
	svc.CarePeriods = carePeriodRepoStub{}

	view, err := svc.GetChildCareOfferings(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22)
	require.NoError(t, err)
	assert.Empty(t, view.Offerings)
	assert.NotNil(t, view.Offerings)
	assert.False(t, view.CanRequest)
	assert.Equal(t, OfferingChangesReasonNoEnrollment, view.ChangesDisabledReason)
}

func TestPendingOfferingChange_HidesAnotherGuardiansRequest(t *testing.T) {
	t.Parallel()

	view := &enrollmentSvc.OfferingChangeView{Request: &enrollmentModels.OfferingChangeRequest{SubmittedBy: 41}}
	assert.Nil(t, pendingOfferingChange(view, 42, false))
}

func TestLoadChildCareOfferingsReadsOfferingHistory(t *testing.T) {
	t.Parallel()

	today := timezone.TodayDate()
	period := &enrollmentSvc.StudentCarePeriod{
		RequestChildID:   101,
		ServiceStartDate: today.AddDays(-20),
		ServiceEndDate:   today.AddDays(-1),
	}
	periodEndExclusive := capability.Date(today)
	links := &recordingChildOfferingRepoStub{links: []*capability.RequestChildOffering{{
		CareOfferingID: 1,
		ValidUntil:     &periodEndExclusive,
	}}}
	svc := &service{ServiceConfig: ServiceConfig{
		CarePeriods:     carePeriodRepoStub{periods: []*enrollmentSvc.StudentCarePeriod{period}},
		OfferingHistory: links,
		CareOfferingRepo: careOfferingRepoStub{offerings: []*enrollmentModels.CareOffering{{
			ID: 1, Name: "Nachmittagsbetreuung",
		}}},
	}}
	view := &ChildCareOfferings{Offerings: []CareOfferingSelection{}}

	_, err := svc.loadChildCareOfferings(testpkg.WithPackageTenantRuntime(context.Background()), 22, today, view)
	require.NoError(t, err)
	assert.Equal(t, 1, links.historyCalls)
	assert.Empty(t, links.dates)
	require.Len(t, view.Offerings, 1)
	assert.Equal(t, "Nachmittagsbetreuung", view.Offerings[0].Name)
}

func TestGetChildCareOfferingsPropagatesDependencyFailures(t *testing.T) {
	t.Parallel()

	dependencyErr := errors.New("dependency failed")
	tests := []struct {
		name  string
		setup func(*service)
	}{
		{
			name: "ownership lookup",
			setup: func(svc *service) {
				svc.ChildRepo = careOfferingsChildRepoStub{err: dependencyErr}
			},
		},
		{
			name: "care periods",
			setup: func(svc *service) {
				svc.CarePeriods = carePeriodRepoStub{err: dependencyErr}
			},
		},
		{
			name: "offering links",
			setup: func(svc *service) {
				svc.CarePeriods = currentCarePeriodStub()
				svc.OfferingHistory = childOfferingRepoStub{err: dependencyErr}
				svc.CareOfferingRepo = careOfferingRepoStub{}
			},
		},
		{
			name: "care offering rows",
			setup: func(svc *service) {
				svc.CarePeriods = currentCarePeriodStub()
				svc.OfferingHistory = childOfferingRepoStub{links: []*capability.RequestChildOffering{{CareOfferingID: 1}}}
				svc.CareOfferingRepo = careOfferingRepoStub{err: dependencyErr}
			},
		},
		{
			name: "request view",
			setup: func(svc *service) {
				svc.CarePeriods = carePeriodRepoStub{}
				svc.OfferingChanges = &offeringChangesStub{viewErr: dependencyErr}
			},
		},
		{
			name: "earliest effective date",
			setup: func(svc *service) {
				svc.CarePeriods = carePeriodRepoStub{}
				svc.OfferingChanges = &offeringChangesStub{earliestErr: dependencyErr}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := careOfferingsTestDB(t)
			svc := careOfferingsService(db, permittedCareOfferingsChild(t), nil)
			tt.setup(svc)

			_, err := svc.GetChildCareOfferings(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22)
			require.Error(t, err)
			assert.ErrorIs(t, err, dependencyErr)
		})
	}
}

func currentCarePeriodStub() carePeriodRepoStub {
	today := timezone.TodayDate()
	return carePeriodRepoStub{periods: []*enrollmentSvc.StudentCarePeriod{{
		RequestChildID:   1,
		ServiceStartDate: today.AddDays(-1),
		ServiceEndDate:   today.AddDays(1),
	}}}
}

func TestCurrentCarePeriodSelection(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2027, time.January, 15)
	current := &enrollmentSvc.StudentCarePeriod{
		RequestChildID:   1,
		ServiceStartDate: today.AddDays(-10),
		ServiceEndDate:   today.AddDays(10),
	}
	upcomingEarly := &enrollmentSvc.StudentCarePeriod{
		RequestChildID:   2,
		ServiceStartDate: today.AddDays(20),
		ServiceEndDate:   today.AddDays(40),
	}
	upcomingLate := &enrollmentSvc.StudentCarePeriod{
		RequestChildID:   3,
		ServiceStartDate: today.AddDays(50),
		ServiceEndDate:   today.AddDays(70),
	}
	pastRecent := &enrollmentSvc.StudentCarePeriod{
		RequestChildID:   4,
		ServiceStartDate: today.AddDays(-40),
		ServiceEndDate:   today.AddDays(-20),
	}
	pastOld := &enrollmentSvc.StudentCarePeriod{
		RequestChildID:   5,
		ServiceStartDate: today.AddDays(-80),
		ServiceEndDate:   today.AddDays(-60),
	}

	tests := []struct {
		name    string
		repo    enrollmentSvc.StudentCarePeriodReader
		want    *enrollmentSvc.StudentCarePeriod
		wantErr bool
	}{
		{name: "repository not wired"},
		{name: "empty", repo: carePeriodRepoStub{}},
		{name: "current", repo: carePeriodRepoStub{periods: []*enrollmentSvc.StudentCarePeriod{
			upcomingLate, current, pastRecent,
		}}, want: current},
		{name: "earliest upcoming", repo: carePeriodRepoStub{periods: []*enrollmentSvc.StudentCarePeriod{
			upcomingLate, upcomingEarly,
		}}, want: upcomingEarly},
		{name: "most recent past", repo: carePeriodRepoStub{periods: []*enrollmentSvc.StudentCarePeriod{
			pastRecent, pastOld,
		}}, want: pastRecent},
		{name: "repository error", repo: carePeriodRepoStub{err: errors.New("periods")}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &service{ServiceConfig: ServiceConfig{CarePeriods: tt.repo}}
			got, err := svc.currentCarePeriod(testpkg.WithPackageTenantRuntime(context.Background()), 22, today)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOfferingChangeAvailabilityReasonsAndSettingFailures(t *testing.T) {
	t.Parallel()

	today := timezone.TodayDate()
	activePeriod := &enrollmentSvc.StudentCarePeriod{ServiceEndDate: today.AddDays(1)}
	endedPeriod := &enrollmentSvc.StudentCarePeriod{ServiceEndDate: today.AddDays(-1)}
	permitted := &parentChild{
		tenantID: 1,
		guardianPermissions: map[string]interface{}{
			authorize.GuardianPermissionRequestSubmit: true,
		},
	}
	notPermitted := &parentChild{tenantID: 1}

	tests := []struct {
		name       string
		settings   configSvc.SettingsService
		child      *parentChild
		period     *enrollmentSvc.StudentCarePeriod
		want       bool
		wantReason string
	}{
		{name: "no enrollment", child: permitted, wantReason: OfferingChangesReasonNoEnrollment},
		{name: "no permission", child: notPermitted, period: activePeriod, wantReason: OfferingChangesReasonNoPermission},
		{name: "period over", child: permitted, period: endedPeriod, wantReason: OfferingChangesReasonPeriodOver},
		{name: "settings not wired", child: permitted, period: activePeriod, wantReason: OfferingChangesReasonSchoolOff},
		{
			name:       "setting lookup fails closed",
			settings:   offeringChangeSettingsStub{err: errors.New("settings")},
			child:      permitted,
			period:     activePeriod,
			wantReason: OfferingChangesReasonSchoolOff,
		},
		{
			name:       "school disabled",
			settings:   offeringChangeSettingsStub{},
			child:      permitted,
			period:     activePeriod,
			wantReason: OfferingChangesReasonSchoolOff,
		},
		{
			name: "care offerings disabled",
			settings: offeringChangeSettingsStub{
				enabled:              true,
				careOfferingsEnabled: boolPtr(false),
			},
			child:      permitted,
			period:     activePeriod,
			wantReason: OfferingChangesReasonSchoolOff,
		},
		{
			name:     "available",
			settings: offeringChangeSettingsStub{enabled: true},
			child:    permitted,
			period:   activePeriod,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &service{ServiceConfig: ServiceConfig{Settings: tt.settings, Logger: slog.Default()}}
			got, reason := svc.resolveOfferingChangeAvailabilityForStudent(
				testpkg.WithPackageTenantRuntime(context.Background()),
				tt.child,
				0,
				tt.period,
				today,
			)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestOfferingChangeCommandsAuthorizeDelegateAndRefresh(t *testing.T) {
	t.Parallel()

	db := careOfferingsTestDB(t)
	today := timezone.TodayDate()
	child := permittedCareOfferingsChild(t)
	changes := &offeringChangesStub{
		catalog:  &enrollmentSvc.OfferingChangeCatalog{PhaseID: 99},
		earliest: today.AddDays(15),
	}
	svc := careOfferingsService(db, child, changes)
	svc.CarePeriods = carePeriodRepoStub{}

	catalog, err := svc.GetChildOfferingCatalog(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22)
	require.NoError(t, err)
	assert.Equal(t, int64(99), catalog.PhaseID)

	selections := []enrollmentSvc.OfferingChangeSelection{{OfferingID: 41, SelectedDays: []string{"mon"}}}
	view, err := svc.CreateOfferingChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22, selections, today.AddDays(20), "Bitte", false, nil)
	require.NoError(t, err)
	assert.NotNil(t, view)
	assert.Equal(t, int64(22), changes.createdInput.StudentID)
	assert.Equal(t, int64(11), changes.createdInput.AccountID)
	assert.Equal(t, selections, changes.createdInput.Selections)
	assert.Equal(t, "Bitte", changes.createdInput.Note)

}

func TestOfferingChangeCommandsRejectMissingDependencyPermissionAndDelegateErrors(t *testing.T) {
	t.Parallel()

	delegateErr := errors.New("delegate failed")
	tests := []struct {
		name   string
		child  *parentModels.ChildSummary
		change *offeringChangesStub
		call   func(*service) error
		want   error
	}{
		{
			name:  "catalog no service",
			child: permittedCareOfferingsChild(t),
			call: func(svc *service) error {
				_, err := svc.GetChildOfferingCatalog(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22)
				return err
			},
			want: enrollmentSvc.ErrOfferingChangeDisabled,
		},
		{
			name:  "create no service",
			child: permittedCareOfferingsChild(t),
			call: func(svc *service) error {
				_, err := svc.CreateOfferingChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22, nil, timezone.TodayDate(), "", false, nil)
				return err
			},
			want: enrollmentSvc.ErrOfferingChangeDisabled,
		},
		{
			name:   "catalog permission denied",
			child:  &parentModels.ChildSummary{StudentID: 22, TenantID: testpkg.Tenant(t)},
			change: &offeringChangesStub{},
			call: func(svc *service) error {
				_, err := svc.GetChildOfferingCatalog(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22)
				return err
			},
			want: ErrGuardianPermissionDenied,
		},
		{
			name:   "catalog delegate",
			child:  permittedCareOfferingsChild(t),
			change: &offeringChangesStub{catalogErr: delegateErr},
			call: func(svc *service) error {
				_, err := svc.GetChildOfferingCatalog(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22)
				return err
			},
			want: delegateErr,
		},
		{
			name:   "create delegate",
			child:  permittedCareOfferingsChild(t),
			change: &offeringChangesStub{createErr: delegateErr},
			call: func(svc *service) error {
				// #2267: reason policy defaults to "both" — an empty note is
				// now refused before the delegate is reached, so this case
				// carries one to keep testing the delegate's error.
				_, err := svc.CreateOfferingChangeRequest(testpkg.WithPackageTenantRuntime(context.Background()), 11, 22, nil, timezone.TodayDate(), "Bitte", false, nil)
				return err
			},
			want: delegateErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := careOfferingsTestDB(t)
			svc := careOfferingsService(db, tt.child, tt.change)
			err := tt.call(svc)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}
