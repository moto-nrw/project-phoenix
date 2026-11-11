package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog"
	"github.com/moto-nrw/project-phoenix/workflows/traegercatalog/ports"
)

// fakeOwners stands in for every owner port and records the order of the
// transactions and calls.
type fakeOwners struct {
	mutex     sync.Mutex
	exclusive bool
	schools   []int64
	phase     enrollment.TraegerPhase
	offerings []careplan.TraegerOffering
	// stamps per school; a rollout that sees another stamp is stale.
	stamps map[int64]string
	// refuse makes the phase copy of a school fail with err.
	refuse              map[int64]error
	applied             map[int64]enrollment.TraegerPhaseCopy
	prices              []ports.ModulePrices
	recorded            []enrollment.TraegerCopyOutcome
	offered             map[int64][]careplan.TraegerOfferingOutcome
	trail               []string
	inTraeger, inSchool int64
	released            bool
	schoolOwners        map[int64]int64
	billingError        error
	templateError       error
	recordError         error
	recordCalls         int
	releaseDraft        *careplan.TraegerOfferingDraft
	copyAlreadyExists   bool
}

func newFakeOwners() *fakeOwners {
	price := []careplan.TraegerCarePrice{{Weekday: "mon", MonthlyCents: 1600}}
	return &fakeOwners{
		schools: []int64{10, 20},
		phase: enrollment.TraegerPhase{ID: 7, OrganizationID: 1, AllSchools: true, Draft: enrollment.TraegerPhaseFields{
			Name: "Schuljahr", ServiceStartDate: "2027-08-01", ServiceEndDate: "2028-07-31",
		}},
		offerings: []careplan.TraegerOffering{{ID: 70, TraegerPhaseID: 7, Draft: careplan.TraegerOfferingDraft{
			Fields: careplan.TraegerOfferingFields{Name: "Frühbetreuung"}, Prices: price,
		}}},
		stamps: map[int64]string{10: "a", 20: "b"}, refuse: map[int64]error{},
		applied: map[int64]enrollment.TraegerPhaseCopy{}, offered: map[int64][]careplan.TraegerOfferingOutcome{},
	}
}

func (f *fakeOwners) catalog(t *testing.T) *Catalog {
	t.Helper()
	catalog, err := New(ports.Dependencies{
		Schools: fakeSchools{f}, Phases: fakePhases{f}, Offerings: fakeOfferings{f},
		SchoolPhases: fakeSchoolPhases{f}, SchoolOfferings: fakeSchoolOfferings{f},
		Tariffs: func(_ context.Context, _, _ int64, prices []ports.ModulePrices) error {
			if len(prices) > 0 {
				f.trail = append(f.trail, fmt.Sprintf("tariff %d in träger %d", prices[0].SchoolID, f.inTraeger))
			}
			f.prices = append(f.prices, prices...)
			return f.billingError
		},
		ContractTemplates: func(context.Context, int64, int64, int64, []ports.ModuleTemplate) (func(context.Context) error, error) {
			return func(context.Context) error { return f.templateError }, nil
		},
		Runtime:     fakeRuntime{f},
		Today:       func() calendar.Date { return calendar.NewDate(2026, 11, 11) },
		Fingerprint: func(content []byte) string { return fmt.Sprintf("%x", content) },
	})
	require.NoError(t, err)
	return catalog
}

type fakeRuntime struct{ f *fakeOwners }

func (r fakeRuntime) WithinExclusive(ctx context.Context, _ string, fn func(context.Context) error) error {
	r.f.mutex.Lock()
	defer r.f.mutex.Unlock()
	r.f.exclusive = true
	defer func() { r.f.exclusive = false }()
	return fn(ctx)
}

func (r fakeRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	if r.f.inSchool != 0 || r.f.inTraeger != 0 {
		return errors.New("nested transaction")
	}
	r.f.inTraeger = organizationID
	defer func() { r.f.inTraeger = 0 }()
	before := len(r.f.prices)
	err := fn(ctx)
	if err != nil {
		r.f.prices = r.f.prices[:before]
	}
	return err
}

func (r fakeRuntime) WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	if r.f.inSchool != 0 || r.f.inTraeger != 0 {
		return errors.New("nested transaction")
	}
	r.f.inSchool = schoolID
	defer func() { r.f.inSchool = 0 }()
	before, had := r.f.applied[schoolID]
	err := fn(ctx)
	if err != nil { // the transaction rolls back
		delete(r.f.applied, schoolID)
		if had {
			r.f.applied[schoolID] = before
		}
	}
	return err
}

type fakeSchools struct{ f *fakeOwners }

func (s fakeSchools) ListTraegerSchoolIDs(context.Context, int64) ([]int64, error) {
	return s.f.schools, nil
}

func (s fakeSchools) FindSchoolForShare(_ context.Context, id int64) (organizationtenancy.School, error) {
	org := office.OrganizationID
	if owner, ok := s.f.schoolOwners[id]; ok {
		org = owner
	}
	return organizationtenancy.School{ID: id, OrganizationID: org}, nil
}

type fakePhases struct{ f *fakeOwners }

func (p fakePhases) TraegerPhase(context.Context, int64, int64) (enrollment.TraegerPhase, error) {
	return p.f.phase, nil
}

func (p fakePhases) LockTraegerPhase(context.Context, int64, int64) (enrollment.TraegerPhase, error) {
	p.f.trail = append(p.f.trail, fmt.Sprintf("lock phase in träger %d", p.f.inTraeger))
	return p.f.phase, nil
}

func (p fakePhases) ReleaseTraegerPhase(context.Context, int64, int64, int64) (int, error) {
	if !p.f.exclusive {
		return 0, errors.New("release outside the rollout mutex")
	}
	p.f.released = true
	p.f.trail = append(p.f.trail, fmt.Sprintf("release in träger %d", p.f.inTraeger))
	return 1, nil
}

func (p fakePhases) RecordTraegerPhaseOutcomes(_ context.Context, _, _ int64, outcomes []enrollment.TraegerCopyOutcome) error {
	p.f.trail = append(p.f.trail, fmt.Sprintf("record in träger %d", p.f.inTraeger))
	if !p.f.exclusive {
		return errors.New("outcome outside the rollout mutex")
	}
	p.f.recordCalls++
	if p.f.recordCalls > 1 && p.f.recordError != nil {
		return p.f.recordError
	}
	p.f.recorded = outcomes
	return nil
}

type fakeOfferings struct{ f *fakeOwners }

func (o fakeOfferings) ListTraegerOfferings(context.Context, int64, int64) ([]careplan.TraegerOffering, error) {
	return o.f.offerings, nil
}

func (o fakeOfferings) ReleaseTraegerOfferings(context.Context, int64, int64, int64) ([]careplan.TraegerOffering, error) {
	released := make([]careplan.TraegerOffering, 0, len(o.f.offerings))
	for _, offering := range o.f.offerings {
		version, draft := 1, offering.Draft
		if o.f.releaseDraft != nil {
			draft = *o.f.releaseDraft
			offering.Draft = draft
		}
		offering.ReleasedVersion, offering.Released = &version, &draft
		released = append(released, offering)
	}
	return released, nil
}

func (o fakeOfferings) RecordTraegerOfferingOutcomes(_ context.Context, _ int64, outcomes map[int64][]careplan.TraegerOfferingOutcome) error {
	o.f.offered = outcomes
	return nil
}

type fakeSchoolPhases struct{ f *fakeOwners }

func (p fakeSchoolPhases) TraegerPhaseCopyState(_ context.Context, _ int64, lock bool) (enrollment.TraegerCopyState, error) {
	if lock {
		p.f.trail = append(p.f.trail, fmt.Sprintf("lock copy in school %d", p.f.inSchool))
	}
	return enrollment.TraegerCopyState{Stamp: p.f.stamps[p.f.inSchool]}, nil
}

func (p fakeSchoolPhases) ApplyTraegerPhase(_ context.Context, copy enrollment.TraegerPhaseCopy) (enrollment.TraegerPhaseApplied, error) {
	if err := p.f.refuse[p.f.inSchool]; err != nil {
		return enrollment.TraegerPhaseApplied{}, err
	}
	p.f.applied[p.f.inSchool] = copy
	return enrollment.TraegerPhaseApplied{CopyID: p.f.inSchool * 100, Created: true}, nil
}

func (p fakeSchoolPhases) WithdrawTraegerPhase(context.Context, int64) (enrollment.TraegerWithdrawal, error) {
	return enrollment.TraegerWithdrawal{Found: true}, nil
}

type fakeSchoolOfferings struct{ f *fakeOwners }

func (o fakeSchoolOfferings) TraegerOfferingCopyStates(context.Context, []int64, bool) ([]careplan.TraegerOfferingCopyState, error) {
	return nil, nil
}

func (o fakeSchoolOfferings) ApplyTraegerOffering(_ context.Context, copy careplan.TraegerOfferingCopy) (careplan.TraegerOfferingApplied, error) {
	return careplan.TraegerOfferingApplied{SourceID: copy.SourceID, CopyID: copy.PhaseID + 1, Created: !o.f.copyAlreadyExists}, nil
}

func (o fakeSchoolOfferings) WithdrawTraegerOffering(context.Context, int64) (careplan.TraegerOfferingWithdrawal, error) {
	return careplan.TraegerOfferingWithdrawal{Found: true}, nil
}

var office = traegercatalog.Actor{OrganizationID: 1, AccountID: 5}

// TestRolloutFollowsThePreview pins ADR 0195 section 5: a rollout writes in
// every school the preview showed, releases under the lock of the Träger
// phase before any school transaction, writes the tariff after the school's
// commit and records the outcomes last, and never nests two transactions.
func TestRolloutFollowsThePreview(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	catalog := f.catalog(t)
	preview, err := catalog.Preview(context.Background(), office, 7)
	require.NoError(t, err)
	require.Len(t, preview.Schools, 2)
	assert.False(t, f.released, "a preview releases nothing")
	assert.Empty(t, f.applied, "a preview keeps no copy")
	require.NotNil(t, preview.Schools[0].Offerings[0].PricesFrom)
	assert.Equal(t, calendar.NewDate(2027, 8, 1), *preview.Schools[0].Offerings[0].PricesFrom, "a new copy is priced from the service start")

	assert.Empty(t, f.prices, "preview prices are rolled back")
	f.trail = nil
	rollout, err := catalog.Rollout(context.Background(), office, 7, preview.Fingerprint)
	require.NoError(t, err)
	assert.Equal(t, []traegercatalog.SchoolResult{{SchoolID: 10, Outcome: "applied"}, {SchoolID: 20, Outcome: "applied"}}, rollout.Schools)
	assert.Equal(t, []string{
		"lock phase in träger 1", "release in träger 1", "record in träger 1",
		"lock copy in school 10", "tariff 10 in träger 1", "record in träger 1",
		"lock copy in school 20", "tariff 20 in träger 1", "record in träger 1",
		"record in träger 1",
	}, f.trail, "release, then each school and its atomic financial outcome, then all outcomes")
	require.Len(t, f.prices, 2)
	assert.Equal(t, int64(1001), f.prices[0].CareOfferingID, "the tariff names the school's copy")
	require.Len(t, f.offered[70], 2)
	assert.Equal(t, 1, *f.offered[70][0].AppliedVersion)
}

// TestRolloutRefusesAChangedDraft pins the fingerprint: a draft or a school
// list that changed since the preview releases nothing.
func TestRolloutRefusesAChangedDraft(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	catalog := f.catalog(t)
	preview, err := catalog.Preview(context.Background(), office, 7)
	require.NoError(t, err)
	f.phase.Draft.Name = "Schuljahr 27/28"
	_, err = catalog.Rollout(context.Background(), office, 7, preview.Fingerprint)
	assert.ErrorIs(t, err, traegercatalog.ErrPreviewStale)
	assert.False(t, f.released, "a stale preview releases no version")

	f.phase.Draft.Name = "Schuljahr"
	f.schools = []int64{10}
	_, err = catalog.Rollout(context.Background(), office, 7, preview.Fingerprint)
	assert.ErrorIs(t, err, traegercatalog.ErrPreviewStale, "a school that left the Träger changes the plan")
	_, err = catalog.Rollout(context.Background(), office, 7, "not a fingerprint")
	assert.ErrorIs(t, err, traegercatalog.ErrPreviewStale)
}

// TestRolloutIsAtomicPerSchool pins ADR 0195 section 5 points 4 and 6: a
// school whose state moved since the preview is stale, a school that
// refuses keeps its version with the code, and the other schools take the
// version.
func TestRolloutIsAtomicPerSchool(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	f.schools = []int64{10, 20, 30}
	f.stamps[30] = "c"
	catalog := f.catalog(t)
	preview, err := catalog.Preview(context.Background(), office, 7)
	require.NoError(t, err)
	f.stamps[10] = "a moved"
	f.refuse[20] = fmt.Errorf("refused: %w", enrollment.ErrPhaseDuplicateName)

	rollout, err := catalog.Rollout(context.Background(), office, 7, preview.Fingerprint)
	require.NoError(t, err)
	assert.Equal(t, []traegercatalog.SchoolResult{
		{SchoolID: 10, Outcome: "stale"},
		{SchoolID: 20, Outcome: "rejected", ErrorCode: "enrollment.phase_name_exists"},
		{SchoolID: 30, Outcome: "applied"},
	}, rollout.Schools)
	assert.NotContains(t, f.applied, int64(10), "a stale school writes nothing")
	require.Len(t, f.recorded, 3)
	assert.Nil(t, f.recorded[1].AppliedVersion, "a refusal keeps the version applied before")
	require.Len(t, f.prices, 1, "only the applied school gets its tariff")
	assert.Equal(t, int64(30), f.prices[0].SchoolID)

	f.refuse[20] = errors.New("database gone")
	f.stamps[10] = "a"
	preview, err = catalog.Preview(context.Background(), office, 7)
	assert.ErrorContains(t, err, "database gone", "a failure of the rollout itself is no school's refusal")
	assert.Empty(t, preview.Fingerprint)
}

// TestTargetsAreSchoolsOfTheTraeger pins ADR 0195 section 2 point 5: a
// selected school that no longer belongs to the Träger is skipped, a school
// that left the selection withdraws its copy, and a withdrawn phase
// withdraws everywhere.
func TestTargetsAreSchoolsOfTheTraeger(t *testing.T) {
	t.Parallel()
	applied := 1
	snap := snapshot{schools: []int64{10, 20}, phase: enrollment.TraegerPhase{Schools: []enrollment.TraegerCatalogSchool{
		{SchoolID: 10, Selected: true},
		{SchoolID: 20, Selected: false, AppliedVersion: &applied, Outcome: "applied"},
		{SchoolID: 99, Selected: true, AppliedVersion: &applied, Outcome: "applied"},
	}}}
	assert.Equal(t, []target{{schoolID: 10, action: "update"}, {schoolID: 20, action: "withdraw"}}, snap.targets())
	withdrawn := calendar.NewDate(2026, 11, 11).BerlinMidnight()
	snap.phase.WithdrawnAt = &withdrawn
	assert.Equal(t, []target{{schoolID: 20, action: "withdraw"}}, snap.targets(), "a withdrawn phase withdraws only existing copies")
}

// TestBillingFollowsTheRecordedVersion pins ADR 0195 section 6: a school one
// version behind gets a new tariff from the next month only when the prices
// changed; a school that holds the version gets nothing.
func TestBillingFollowsTheRecordedVersion(t *testing.T) {
	t.Parallel()
	one := 1
	copyID := newFakeOwners().schools[0]*100 + 1
	before := careplan.TraegerOfferingDraft{Prices: []careplan.TraegerCarePrice{{Weekday: "mon", MonthlyCents: 1600}}}
	offering := careplan.TraegerOffering{ReleasedVersion: &one, Released: &before, Schools: []careplan.TraegerOfferingSchool{
		{SchoolID: 10, AppliedVersion: &one, AppliedCopyID: &copyID, Applied: &before, Outcome: "applied"},
	}}
	today := calendar.NewDate(2026, 11, 11)
	phase := enrollment.TraegerPhaseFields{ServiceStartDate: "2026-08-01"}
	same := billingFor(offering, offeringRelease{version: 2, draft: before}, 10, phase, today, careplan.TraegerOfferingApplied{CopyID: copyID})
	assert.Nil(t, same.pricesFrom, "unchanged prices write no tariff")
	dearer := careplan.TraegerOfferingDraft{Prices: []careplan.TraegerCarePrice{{Weekday: "mon", MonthlyCents: 1700}}}
	changed := billingFor(offering, offeringRelease{version: 2, draft: dearer}, 10, phase, today, careplan.TraegerOfferingApplied{CopyID: copyID})
	require.NotNil(t, changed.pricesFrom)
	assert.Equal(t, calendar.NewDate(2026, 12, 1), *changed.pricesFrom, "a changed price starts the next month")
	held := billingFor(offering, offeringRelease{version: 1, draft: dearer}, 10, phase, today, careplan.TraegerOfferingApplied{CopyID: copyID})
	assert.Nil(t, held.pricesFrom, "a school that holds the version gets nothing")
	fresh := billingFor(offering, offeringRelease{version: 2, draft: dearer}, 20, phase, today, careplan.TraegerOfferingApplied{CopyID: copyID})
	require.NotNil(t, fresh.pricesFrom)
	assert.Equal(t, calendar.NewDate(2026, 11, 1), *fresh.pricesFrom, "a new copy is priced from the current month at the earliest")
}

func TestBillingInitializesPreviouslyUnpricedCopy(t *testing.T) {
	t.Parallel()
	one, copyID := 1, int64(1001)
	before := careplan.TraegerOfferingDraft{Prices: []careplan.TraegerCarePrice{}}
	offering := careplan.TraegerOffering{Schools: []careplan.TraegerOfferingSchool{
		{SchoolID: 10, AppliedVersion: &one, AppliedCopyID: &copyID, Applied: &before, Outcome: "applied"},
	}}
	next := offeringRelease{version: 2, draft: careplan.TraegerOfferingDraft{Prices: []careplan.TraegerCarePrice{{Weekday: "mon", MonthlyCents: 1600}}}}
	for _, start := range []calendar.Date{"2026-08-01", "2027-02-15"} {
		plan := billingFor(offering, next, 10, enrollment.TraegerPhaseFields{ServiceStartDate: enrollment.Date(start)}, calendar.NewDate(2026, 11, 11), careplan.TraegerOfferingApplied{CopyID: copyID})
		require.NotNil(t, plan.pricesFrom)
		want := calendar.NewDate(2026, 11, 1)
		if start.After(want) {
			want = monthStart(start)
		}
		assert.Equal(t, want, *plan.pricesFrom)
	}
}

func TestRolloutRevalidatesSchoolOwnership(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	catalog := f.catalog(t)
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	f.schoolOwners = map[int64]int64{10: 2}
	result, err := catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	require.NoError(t, err)
	assert.Equal(t, enrollment.TraegerOutcomeStale, result.Schools[0].Outcome)
	assert.NotContains(t, f.applied, int64(10), "the previous Träger writes no copy after a school moves")
}

func TestRolloutRejectsUnconfirmedRelease(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	catalog := f.catalog(t)
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	changed := f.offerings[0].Draft
	changed.Fields.Name = "Changed after confirmation"
	f.releaseDraft = &changed
	_, err = catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	assert.ErrorIs(t, err, traegercatalog.ErrPreviewStale)
	assert.Empty(t, f.applied)
}

func TestRolloutBillingRefusalRejectsSchool(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"tariff", "template"} {
		t.Run(kind, func(t *testing.T) {
			f := newFakeOwners()
			f.offerings[0].Draft.ContractTemplate = &careplan.TraegerContractTemplate{TermKind: "school_year"}
			catalog := f.catalog(t)
			preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
			require.NoError(t, err)
			code := "parent_billing.care_tariff_month_billed"
			if kind == "tariff" {
				f.billingError = &ports.Refusal{Code: code, Err: errors.New("month billed")}
			} else {
				code = "care_contract.template_invalid"
				f.templateError = &ports.Refusal{Code: code, Err: errors.New("invalid template")}
			}
			result, err := catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
			require.NoError(t, err)
			for _, school := range result.Schools {
				assert.Equal(t, enrollment.TraegerOutcomeRejected, school.Outcome)
				assert.Equal(t, code, school.ErrorCode)
			}
			for _, outcome := range f.recorded {
				assert.Nil(t, outcome.AppliedVersion)
			}
			for _, outcome := range f.offered[70] {
				assert.Nil(t, outcome.AppliedVersion)
				assert.Equal(t, code, outcome.ErrorCode)
			}
		})
	}
}

func TestRolloutRetainsRecoveryTargetsWhenRecordingFails(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	catalog := f.catalog(t)
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	f.recordError = errors.New("outcomes unavailable")
	_, err = catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	require.ErrorIs(t, err, f.recordError)
	require.Len(t, f.applied, 1, "the first school copy committed before its financial outcome failed")
	assert.Empty(t, f.prices, "a failed financial outcome rolls tariffs back")
	require.Len(t, f.recorded, 2, "the earlier recovery targets survive")
	for _, outcome := range f.recorded {
		assert.Equal(t, enrollment.TraegerOutcomeStale, outcome.Outcome)
		assert.Nil(t, outcome.AppliedVersion)
	}
}

func TestTargetsRecoverUnfinishedCopies(t *testing.T) {
	t.Parallel()
	snap := snapshot{schools: []int64{10}, phase: enrollment.TraegerPhase{Schools: []enrollment.TraegerCatalogSchool{
		{SchoolID: 10, Selected: false, Outcome: enrollment.TraegerOutcomeStale},
	}}}
	assert.Equal(t, []target{{schoolID: 10, action: traegercatalog.ActionWithdraw}}, snap.targets(), "pending work remains a withdrawal target without an applied version")
}

func TestBillingPreservesTermsAcrossMissedReleases(t *testing.T) {
	t.Parallel()
	one, three := 1, 3
	copyID := newFakeOwners().schools[0]*100 + 1
	terms := careplan.TraegerOfferingDraft{
		Prices:           []careplan.TraegerCarePrice{{Weekday: "mon", MonthlyCents: 1600}},
		ContractTemplate: &careplan.TraegerContractTemplate{TermKind: "open_ended"},
	}
	offering := careplan.TraegerOffering{ReleasedVersion: &three, Released: &terms, Schools: []careplan.TraegerOfferingSchool{
		{SchoolID: 10, AppliedVersion: &one, AppliedCopyID: &copyID, Applied: &terms, Outcome: enrollment.TraegerOutcomeApplied},
	}}
	plan := billingFor(offering, offeringRelease{version: 4, draft: terms}, 10,
		enrollment.TraegerPhaseFields{ServiceStartDate: "2026-08-01"}, calendar.NewDate(2026, 11, 11), careplan.TraegerOfferingApplied{CopyID: copyID})
	assert.Nil(t, plan.pricesFrom, "a capacity-only catch-up preserves the school's tariff")
	assert.False(t, plan.template, "a capacity-only catch-up preserves the school's contract template")
}

func TestRolloutDoesNotKeepTariffsAfterTemplateRefusal(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	f.offerings[0].Draft.ContractTemplate = &careplan.TraegerContractTemplate{TermKind: "school_year"}
	catalog := f.catalog(t)
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	f.templateError = &ports.Refusal{Code: "care_contract.template_invalid", Err: errors.New("template refused")}
	_, err = catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	require.NoError(t, err)
	assert.Empty(t, f.prices, "a template refusal rolls back the tariff in the same financial transaction")
}

func TestRolloutRejectsPreviewFromAnotherBillingMonth(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	catalog := f.catalog(t)
	today := calendar.NewDate(2026, 11, 30)
	catalog.deps.Today = func() calendar.Date { return today }
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	today = calendar.NewDate(2026, 12, 1)
	_, err = catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	assert.ErrorIs(t, err, traegercatalog.ErrPreviewStale)
	assert.Empty(t, f.applied)
}

func (o fakeSchoolOfferings) ApplyTraegerOfferings(ctx context.Context, copies []careplan.TraegerOfferingCopy, withdraw []int64) (careplan.TraegerOfferingBatchApplied, error) {
	result := careplan.TraegerOfferingBatchApplied{Withdrawn: map[int64]careplan.TraegerOfferingWithdrawal{}}
	for _, copy := range copies {
		applied, err := o.ApplyTraegerOffering(ctx, copy)
		if err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, applied)
	}
	for _, id := range withdraw {
		result.Withdrawn[id] = careplan.TraegerOfferingWithdrawal{Found: true}
	}
	return result, nil
}

func TestRolloutRejectsPreviewWithChangedBillingBaseline(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	catalog := f.catalog(t)
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	version := 1
	applied := f.offerings[0].Draft
	f.offerings[0].Schools = []careplan.TraegerOfferingSchool{{SchoolID: 10, AppliedVersion: &version, Applied: &applied, Outcome: "applied"}}
	_, err = catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	assert.ErrorIs(t, err, traegercatalog.ErrPreviewStale)
	assert.Empty(t, f.applied, "a concurrent financial commit needs a new preview even when copy stamps stayed unchanged")
}

func TestRolloutInitializesRecreatedCopiesDespiteHistoricalAppliedVersions(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	one := 1
	f.offerings[0].Draft.ContractTemplate = &careplan.TraegerContractTemplate{TermKind: "school_year"}
	terms := f.offerings[0].Draft
	f.offerings[0].ReleasedVersion, f.offerings[0].Released = &one, &terms
	f.offerings[0].Schools = []careplan.TraegerOfferingSchool{{SchoolID: 10, AppliedVersion: &one, Applied: &terms, Outcome: "applied"}}
	catalog := f.catalog(t)
	templates := map[int64][]ports.ModuleTemplate{}
	catalog.deps.ContractTemplates = func(_ context.Context, _, _, schoolID int64, inputs []ports.ModuleTemplate) (func(context.Context) error, error) {
		templates[schoolID] = inputs
		return func(context.Context) error { return nil }, nil
	}
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	require.NotNil(t, preview.Schools[0].Offerings[0].PricesFrom, "a recreated copy needs its own tariff")
	assert.True(t, preview.Schools[0].Offerings[0].ContractTemplate)
	_, err = catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	require.NoError(t, err)
	require.Len(t, f.prices, 2, "both newly created school copies get prices")
	require.Len(t, templates[10], 1)
	assert.Equal(t, int64(1001), templates[10][0].CareOfferingID)
}

func TestRolloutRetriesFinancialInitializationForTheCurrentCopy(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	one, oldCopy := 1, int64(900)
	terms := f.offerings[0].Draft
	terms.ContractTemplate = &careplan.TraegerContractTemplate{TermKind: "school_year"}
	f.offerings[0].Draft = terms
	f.offerings[0].ReleasedVersion, f.offerings[0].Released = &one, &terms
	f.offerings[0].Schools = []careplan.TraegerOfferingSchool{{SchoolID: 10, AppliedVersion: &one, AppliedCopyID: &oldCopy, Applied: &terms, Outcome: "applied"}}
	catalog := f.catalog(t)
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	f.billingError = &ports.Refusal{Code: "parent_billing.care_tariff_month_billed", Err: errors.New("billing refused")}
	_, err = catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	require.NoError(t, err)
	assert.Empty(t, f.prices)
	f.copyAlreadyExists, f.billingError = true, nil
	preview, err = catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	require.NotNil(t, preview.Schools[0].Offerings[0].PricesFrom, "the old copy's financial baseline cannot initialize the recreated copy")
	assert.True(t, preview.Schools[0].Offerings[0].ContractTemplate)
	_, err = catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	require.NoError(t, err)
	require.Len(t, f.prices, 2)
	require.NotNil(t, f.offered[70][0].AppliedCopyID)
	assert.Equal(t, int64(1001), *f.offered[70][0].AppliedCopyID)
}

func TestRolloutRejectsPreviewWhenOnlyTheFinancialCopyChanges(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	f.copyAlreadyExists = true
	one, previousCopy, currentCopy := 1, int64(900), int64(1001)
	terms := f.offerings[0].Draft
	f.offerings[0].Schools = []careplan.TraegerOfferingSchool{{SchoolID: 10, AppliedVersion: &one, AppliedCopyID: &previousCopy, Applied: &terms, Outcome: "applied"}}
	catalog := f.catalog(t)
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	f.offerings[0].Schools[0].AppliedCopyID = &currentCopy
	_, err = catalog.Rollout(t.Context(), office, f.phase.ID, preview.Fingerprint)
	assert.ErrorIs(t, err, traegercatalog.ErrPreviewStale, "a changed copy baseline changes the confirmed financial plan")
	assert.Empty(t, f.applied)
}

func TestPreviewRejectsFinancialRefusalsWithoutKeepingWrites(t *testing.T) {
	t.Parallel()
	f := newFakeOwners()
	catalog := f.catalog(t)
	f.billingError = &ports.Refusal{Code: "parent_billing.care_tariff_month_billed", Err: errors.New("month billed")}
	preview, err := catalog.Preview(t.Context(), office, f.phase.ID)
	require.NoError(t, err)
	for _, school := range preview.Schools {
		assert.Equal(t, traegercatalog.PreviewRejected, school.Outcome)
		assert.Equal(t, "parent_billing.care_tariff_month_billed", school.ErrorCode)
	}
	assert.Empty(t, f.prices)
	assert.Empty(t, f.applied)
	assert.False(t, f.released)
}
