package parent

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type careOfferingsChildRepoStub struct {
	parentModels.ChildRepository
	child *parentModels.ChildSummary
	err   error
}

func (s careOfferingsChildRepoStub) FindForAccount(
	_ context.Context,
	_, _ int64,
) (*parentModels.ChildSummary, error) {
	return s.child, s.err
}

type carePeriodRepoStub struct {
	enrollmentModels.RequestChildRepository
	periods []*enrollmentModels.StudentCarePeriod
	err     error
}

func (s carePeriodRepoStub) ListCarePeriodsByStudentID(
	_ context.Context,
	_ int64,
) ([]*enrollmentModels.StudentCarePeriod, error) {
	return s.periods, s.err
}

type childOfferingRepoStub struct {
	enrollmentModels.RequestChildOfferingRepository
	links []*enrollmentModels.RequestChildOffering
	err   error
}

type recordingChildOfferingRepoStub struct {
	enrollmentModels.RequestChildOfferingRepository
	links        []*enrollmentModels.RequestChildOffering
	dates        []timezone.Date
	historyCalls int
	err          error
}

func (s *recordingChildOfferingRepoStub) ListByRequestChildIDAtDate(
	_ context.Context,
	_ int64,
	onDate timezone.Date,
) ([]*enrollmentModels.RequestChildOffering, error) {
	s.dates = append(s.dates, onDate)
	return s.links, s.err
}

func (s *recordingChildOfferingRepoStub) ListHistoryByRequestChildID(
	_ context.Context,
	_ int64,
) ([]*enrollmentModels.RequestChildOffering, error) {
	s.historyCalls++
	return s.links, s.err
}

func (s childOfferingRepoStub) ListByRequestChildID(
	_ context.Context,
	_ int64,
) ([]*enrollmentModels.RequestChildOffering, error) {
	return s.links, s.err
}

func (s childOfferingRepoStub) ListByRequestChildIDAtDate(
	_ context.Context,
	_ int64,
	_ timezone.Date,
) ([]*enrollmentModels.RequestChildOffering, error) {
	return s.links, s.err
}

func (s childOfferingRepoStub) ListHistoryByRequestChildID(
	_ context.Context,
	_ int64,
) ([]*enrollmentModels.RequestChildOffering, error) {
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
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func permittedCareOfferingsChild() *parentModels.ChildSummary {
	return &parentModels.ChildSummary{
		StudentID: 22,
		TenantID:  1,
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
		ChildRepo: careOfferingsChildRepoStub{child: child},
		DB:        db,
		Logger:    slog.Default(),
	}}
	if changes != nil {
		svc.OfferingChanges = changes
	}
	return svc
}

func TestGetChildCareOfferingsReturnsCompleteSortedView(t *testing.T) {
	db := careOfferingsTestDB(t)
	today := timezone.TodayDate()
	description := "Mit Mittagessen"
	price := 4200
	sourceChildID := int64(301)
	futureStart := today.AddDays(15)
	note := "Ab Februar bitte dienstags"
	createdAt := time.Now().Add(-time.Hour)

	firstOffering := &enrollmentModels.CareOffering{
		Model:         base.Model{ID: 41},
		Name:          "Zweite Sortierung",
		SortOrder:     20,
		IncludesLunch: true,
	}
	secondOffering := &enrollmentModels.CareOffering{
		Model:               base.Model{ID: 42},
		Name:                "Erste Sortierung",
		Description:         &description,
		SortOrder:           10,
		PriceCents:          &price,
		IncludesHolidayCare: true,
	}
	futureOffering := &enrollmentModels.CareOffering{
		Model:     base.Model{ID: 43},
		Name:      "Zukünftige Betreuung",
		SortOrder: 30,
	}
	changes := &offeringChangesStub{
		earliest: today.AddDays(15),
		view: &enrollmentSvc.OfferingChangeView{
			Request: &enrollmentModels.OfferingChangeRequest{
				Model:         base.Model{ID: 61, CreatedAt: createdAt},
				EffectiveFrom: today.AddDays(20),
				ParentNote:    &note,
				SubmittedBy:   11,
			},
			Diff: []enrollmentSvc.OfferingChangeDiffEntry{{
				Label:    "Erste Sortierung",
				OldState: "not_booked",
				NewState: "booked",
				NewDays:  []string{"tue"},
			}},
			LastDecision: &enrollmentSvc.OfferingChangeDecision{ID: 60, Status: "rejected"},
		},
	}
	svc := careOfferingsService(db, permittedCareOfferingsChild(), changes)
	svc.Settings = offeringChangeSettingsStub{enabled: true}
	svc.RequestChildRepo = carePeriodRepoStub{periods: []*enrollmentModels.StudentCarePeriod{{
		RequestChildID:   sourceChildID,
		PhaseName:        "Schuljahr 2026/27",
		ServiceStartDate: today.AddDays(-30),
		ServiceEndDate:   today.AddDays(200),
	}}}
	svc.RequestChildOfferingRepo = childOfferingRepoStub{links: []*enrollmentModels.RequestChildOffering{
		nil,
		{CareOfferingID: 41, SelectedDays: []string{"fri", "mon", "fri", "bad"}},
		{CareOfferingID: 42, SelectedDays: []string{"tue"}},
		{CareOfferingID: 999},
		{CareOfferingID: 43, ValidFrom: &futureStart},
	}}
	svc.CareOfferingRepo = careOfferingRepoStub{offerings: []*enrollmentModels.CareOffering{
		firstOffering, nil, secondOffering, futureOffering,
	}}
	view, err := svc.GetChildCareOfferings(context.Background(), 11, 22)
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
	view, err = svc.GetChildCareOfferings(context.Background(), 11, 22)
	require.NoError(t, err)
	assert.Nil(t, view.PendingRequest)
	assert.Equal(t, changes.view.LastDecision, view.LastDecision)
}

func TestGetChildCareOfferingsWithoutEnrollmentStillReturnsEmptySlices(t *testing.T) {
	db := careOfferingsTestDB(t)
	svc := careOfferingsService(db, permittedCareOfferingsChild(), nil)
	svc.RequestChildRepo = carePeriodRepoStub{}

	view, err := svc.GetChildCareOfferings(context.Background(), 11, 22)
	require.NoError(t, err)
	assert.Empty(t, view.Offerings)
	assert.NotNil(t, view.Offerings)
	assert.False(t, view.CanRequest)
	assert.Equal(t, OfferingChangesReasonNoEnrollment, view.ChangesDisabledReason)
}

func TestLoadChildCareOfferingsReadsOfferingHistory(t *testing.T) {
	today := timezone.TodayDate()
	period := &enrollmentModels.StudentCarePeriod{
		RequestChildID:   101,
		ServiceStartDate: today.AddDays(-20),
		ServiceEndDate:   today.AddDays(-1),
	}
	periodEndExclusive := today
	links := &recordingChildOfferingRepoStub{links: []*enrollmentModels.RequestChildOffering{{
		CareOfferingID: 1,
		ValidUntil:     &periodEndExclusive,
	}}}
	svc := &service{ServiceConfig: ServiceConfig{
		RequestChildRepo:         carePeriodRepoStub{periods: []*enrollmentModels.StudentCarePeriod{period}},
		RequestChildOfferingRepo: links,
		CareOfferingRepo: careOfferingRepoStub{offerings: []*enrollmentModels.CareOffering{{
			Model: base.Model{ID: 1}, Name: "Nachmittagsbetreuung",
		}}},
	}}
	view := &ChildCareOfferings{Offerings: []CareOfferingSelection{}}

	_, err := svc.loadChildCareOfferings(context.Background(), 22, today, view)
	require.NoError(t, err)
	assert.Equal(t, 1, links.historyCalls)
	assert.Empty(t, links.dates)
	require.Len(t, view.Offerings, 1)
	assert.Equal(t, "Nachmittagsbetreuung", view.Offerings[0].Name)
}

func TestGetChildCareOfferingsPropagatesDependencyFailures(t *testing.T) {
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
				svc.RequestChildRepo = carePeriodRepoStub{err: dependencyErr}
			},
		},
		{
			name: "offering links",
			setup: func(svc *service) {
				svc.RequestChildRepo = currentCarePeriodStub()
				svc.RequestChildOfferingRepo = childOfferingRepoStub{err: dependencyErr}
				svc.CareOfferingRepo = careOfferingRepoStub{}
			},
		},
		{
			name: "care offering rows",
			setup: func(svc *service) {
				svc.RequestChildRepo = currentCarePeriodStub()
				svc.RequestChildOfferingRepo = childOfferingRepoStub{links: []*enrollmentModels.RequestChildOffering{{CareOfferingID: 1}}}
				svc.CareOfferingRepo = careOfferingRepoStub{err: dependencyErr}
			},
		},
		{
			name: "request view",
			setup: func(svc *service) {
				svc.RequestChildRepo = carePeriodRepoStub{}
				svc.OfferingChanges = &offeringChangesStub{viewErr: dependencyErr}
			},
		},
		{
			name: "earliest effective date",
			setup: func(svc *service) {
				svc.RequestChildRepo = carePeriodRepoStub{}
				svc.OfferingChanges = &offeringChangesStub{earliestErr: dependencyErr}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := careOfferingsTestDB(t)
			svc := careOfferingsService(db, permittedCareOfferingsChild(), nil)
			tt.setup(svc)

			_, err := svc.GetChildCareOfferings(context.Background(), 11, 22)
			require.Error(t, err)
			assert.ErrorIs(t, err, dependencyErr)
		})
	}
}

func currentCarePeriodStub() carePeriodRepoStub {
	today := timezone.TodayDate()
	return carePeriodRepoStub{periods: []*enrollmentModels.StudentCarePeriod{{
		RequestChildID:   1,
		ServiceStartDate: today.AddDays(-1),
		ServiceEndDate:   today.AddDays(1),
	}}}
}

func TestCurrentCarePeriodSelection(t *testing.T) {
	today := timezone.NewDate(2027, time.January, 15)
	current := &enrollmentModels.StudentCarePeriod{
		RequestChildID:   1,
		ServiceStartDate: today.AddDays(-10),
		ServiceEndDate:   today.AddDays(10),
	}
	upcomingEarly := &enrollmentModels.StudentCarePeriod{
		RequestChildID:   2,
		ServiceStartDate: today.AddDays(20),
		ServiceEndDate:   today.AddDays(40),
	}
	upcomingLate := &enrollmentModels.StudentCarePeriod{
		RequestChildID:   3,
		ServiceStartDate: today.AddDays(50),
		ServiceEndDate:   today.AddDays(70),
	}
	pastRecent := &enrollmentModels.StudentCarePeriod{
		RequestChildID:   4,
		ServiceStartDate: today.AddDays(-40),
		ServiceEndDate:   today.AddDays(-20),
	}
	pastOld := &enrollmentModels.StudentCarePeriod{
		RequestChildID:   5,
		ServiceStartDate: today.AddDays(-80),
		ServiceEndDate:   today.AddDays(-60),
	}

	tests := []struct {
		name    string
		repo    enrollmentModels.RequestChildRepository
		want    *enrollmentModels.StudentCarePeriod
		wantErr bool
	}{
		{name: "repository not wired"},
		{name: "empty", repo: carePeriodRepoStub{}},
		{name: "current", repo: carePeriodRepoStub{periods: []*enrollmentModels.StudentCarePeriod{
			upcomingLate, current, pastRecent,
		}}, want: current},
		{name: "earliest upcoming", repo: carePeriodRepoStub{periods: []*enrollmentModels.StudentCarePeriod{
			upcomingLate, upcomingEarly,
		}}, want: upcomingEarly},
		{name: "most recent past", repo: carePeriodRepoStub{periods: []*enrollmentModels.StudentCarePeriod{
			pastRecent, pastOld,
		}}, want: pastRecent},
		{name: "repository error", repo: carePeriodRepoStub{err: errors.New("periods")}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &service{ServiceConfig: ServiceConfig{RequestChildRepo: tt.repo}}
			got, err := svc.currentCarePeriod(context.Background(), 22, today)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Same(t, tt.want, got)
		})
	}
}

func TestOfferingChangeAvailabilityReasonsAndSettingFailures(t *testing.T) {
	today := timezone.TodayDate()
	activePeriod := &enrollmentModels.StudentCarePeriod{ServiceEndDate: today.AddDays(1)}
	endedPeriod := &enrollmentModels.StudentCarePeriod{ServiceEndDate: today.AddDays(-1)}
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
		period     *enrollmentModels.StudentCarePeriod
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
			got, reason := svc.resolveOfferingChangeAvailability(
				context.Background(),
				tt.child,
				tt.period,
				today,
			)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestOfferingChangeCommandsAuthorizeDelegateAndRefresh(t *testing.T) {
	db := careOfferingsTestDB(t)
	today := timezone.TodayDate()
	child := permittedCareOfferingsChild()
	changes := &offeringChangesStub{
		catalog:  &enrollmentSvc.OfferingChangeCatalog{PhaseID: 99},
		earliest: today.AddDays(15),
	}
	svc := careOfferingsService(db, child, changes)
	svc.RequestChildRepo = carePeriodRepoStub{}

	catalog, err := svc.GetChildOfferingCatalog(context.Background(), 11, 22)
	require.NoError(t, err)
	assert.Equal(t, int64(99), catalog.PhaseID)

	selections := []enrollmentSvc.OfferingChangeSelection{{OfferingID: 41, SelectedDays: []string{"mon"}}}
	view, err := svc.CreateOfferingChangeRequest(context.Background(), 11, 22, selections, today.AddDays(20), "Bitte")
	require.NoError(t, err)
	assert.NotNil(t, view)
	assert.Equal(t, int64(22), changes.createdInput.StudentID)
	assert.Equal(t, int64(11), changes.createdInput.AccountID)
	assert.Equal(t, selections, changes.createdInput.Selections)
	assert.Equal(t, "Bitte", changes.createdInput.Note)

	view, err = svc.WithdrawOfferingChangeRequest(context.Background(), 11, 22, 77)
	require.NoError(t, err)
	assert.NotNil(t, view)
	assert.Equal(t, [3]int64{77, 11, 22}, changes.withdrawn)
}

func TestWithdrawOfferingChangeRequestAllowsOwnerWithoutSubmitPermission(t *testing.T) {
	db := careOfferingsTestDB(t)
	child := permittedCareOfferingsChild()
	child.GuardianPermissions = map[string]interface{}{
		authorize.GuardianPermissionPortalAccess: true,
	}
	changes := &offeringChangesStub{}
	svc := careOfferingsService(db, child, changes)
	svc.RequestChildRepo = carePeriodRepoStub{}

	_, err := svc.WithdrawOfferingChangeRequest(context.Background(), 11, 22, 77)
	require.NoError(t, err)
	assert.Equal(t, [3]int64{77, 11, 22}, changes.withdrawn)
}

func TestOfferingChangeCommandsRejectMissingDependencyPermissionAndDelegateErrors(t *testing.T) {
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
			child: permittedCareOfferingsChild(),
			call: func(svc *service) error {
				_, err := svc.GetChildOfferingCatalog(context.Background(), 11, 22)
				return err
			},
			want: enrollmentSvc.ErrOfferingChangeDisabled,
		},
		{
			name:  "create no service",
			child: permittedCareOfferingsChild(),
			call: func(svc *service) error {
				_, err := svc.CreateOfferingChangeRequest(context.Background(), 11, 22, nil, timezone.TodayDate(), "")
				return err
			},
			want: enrollmentSvc.ErrOfferingChangeDisabled,
		},
		{
			name:  "withdraw no service",
			child: permittedCareOfferingsChild(),
			call: func(svc *service) error {
				_, err := svc.WithdrawOfferingChangeRequest(context.Background(), 11, 22, 1)
				return err
			},
			want: enrollmentSvc.ErrOfferingChangeDisabled,
		},
		{
			name:   "catalog permission denied",
			child:  &parentModels.ChildSummary{StudentID: 22, TenantID: 1},
			change: &offeringChangesStub{},
			call: func(svc *service) error {
				_, err := svc.GetChildOfferingCatalog(context.Background(), 11, 22)
				return err
			},
			want: ErrGuardianPermissionDenied,
		},
		{
			name:   "catalog delegate",
			child:  permittedCareOfferingsChild(),
			change: &offeringChangesStub{catalogErr: delegateErr},
			call: func(svc *service) error {
				_, err := svc.GetChildOfferingCatalog(context.Background(), 11, 22)
				return err
			},
			want: delegateErr,
		},
		{
			name:   "create delegate",
			child:  permittedCareOfferingsChild(),
			change: &offeringChangesStub{createErr: delegateErr},
			call: func(svc *service) error {
				_, err := svc.CreateOfferingChangeRequest(context.Background(), 11, 22, nil, timezone.TodayDate(), "")
				return err
			},
			want: delegateErr,
		},
		{
			name:   "withdraw delegate",
			child:  permittedCareOfferingsChild(),
			change: &offeringChangesStub{withdrawErr: delegateErr},
			call: func(svc *service) error {
				_, err := svc.WithdrawOfferingChangeRequest(context.Background(), 11, 22, 1)
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
