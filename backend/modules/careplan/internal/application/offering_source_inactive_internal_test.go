package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// sourceRefusal is the refusal error of sourceRefusalRules, so the tests can
// tell a refusal from a propagated failure.
type sourceRefusal struct{ ports.OfferingSourceRefusal }

func (r sourceRefusal) Error() string { return r.Reason }

type sourceRefusalRules struct{}

func (sourceRefusalRules) MaxSourcesPerTemplate() int { return 10 }
func (sourceRefusalRules) Reject(reason string) error {
	return sourceRefusal{ports.OfferingSourceRefusal{Reason: reason}}
}
func (sourceRefusalRules) RejectWith(refusal ports.OfferingSourceRefusal) error {
	return sourceRefusal{refusal}
}
func (sourceRefusalRules) IsRejection(err error) bool {
	var refusal sourceRefusal
	return errors.As(err, &refusal)
}

type sourcePeriodStub struct {
	ports.CatalogCalendar
	period careplan.LinkedPeriod
	err    error
}

func (s sourcePeriodStub) FindPeriod(context.Context, int64) (careplan.LinkedPeriod, error) {
	return s.period, s.err
}

// inactiveSourceCatalog loads offerings 1 (inactive), 2 and 3 (active, phase
// 1 unless overridden) with the given phase and period lookups.
type inactiveSourceCatalog struct {
	offerings []careplan.CareOffering
	listErr   error
	phases    catalogPhasesStub
	period    sourcePeriodStub
}

func newInactiveSourceCatalog() inactiveSourceCatalog {
	return inactiveSourceCatalog{
		offerings: []careplan.CareOffering{
			{ID: 1, Name: "A", PhaseID: 1},
			{ID: 2, Name: "B", PhaseID: 1, IsActive: true},
			{ID: 3, Name: "C", PhaseID: 1, IsActive: true},
		},
		phases: catalogPhasesStub{phase: careplan.OfferingPhase{
			ID: 1, Name: "Schuljahr", ServiceStart: calendar.NewDate(2026, 9, 1), ServiceEnd: calendar.NewDate(2027, 7, 31),
		}},
		period: sourcePeriodStub{period: careplan.LinkedPeriod{
			ID: 9, StartDate: calendar.NewDate(2026, 8, 1), EndDate: calendar.NewDate(2027, 8, 31), IsActive: true,
		}},
	}
}

func (c inactiveSourceCatalog) load(t *testing.T, ids []int64, periodID *int64, drift sourceDrift) (careplan.OfferingSources, error) {
	t.Helper()
	catalog := &CareOfferingCatalog{deps: CareOfferingCatalogDependencies{
		Records:     catalogRecordsStub{offerings: c.offerings, err: c.listErr},
		Phases:      c.phases,
		Calendar:    c.period,
		SourceRules: sourceRefusalRules{},
	}}
	return catalog.LoadOfferingSources(t.Context(), ids, periodID, drift)
}

func requireRefusal(t *testing.T, err error, kind ports.OfferingSourceRefusalKind) {
	t.Helper()
	var refusal sourceRefusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, kind, refusal.Kind)
}

// An inactive stored source contributes nothing while another stays valid
// (#5305); the remaining sources keep their order and phase.
func TestLoadOfferingSources_SkipsInactiveSourceNextToValidOne(t *testing.T) {
	t.Parallel()
	period := int64(9)
	for _, ids := range [][]int64{{1, 2, 3}, {2, 1, 3}, {2, 3, 1}} {
		sources, err := newInactiveSourceCatalog().load(t, ids, &period, skipInactiveSources)
		require.NoError(t, err, "sources %v", ids)
		require.Len(t, sources.Offerings, 2, "sources %v", ids)
		assert.Equal(t, int64(2), sources.Offerings[0].ID)
		assert.Equal(t, int64(3), sources.Offerings[1].ID)
		assert.Equal(t, []int64{1}, sources.Inactive)
		require.NotNil(t, sources.Phase)
		assert.Equal(t, int64(1), sources.Phase.ID)
	}
}

// Every guard but the inactive one still applies: declarations and catalog
// edits refuse an inactive source, a template without a valid source keeps
// its refusal, and the remaining sources are checked for mixed phases, the
// period and their phase lookup.
func TestLoadOfferingSources_SkipInactiveKeepsEveryOtherRefusal(t *testing.T) {
	t.Parallel()
	period := int64(9)

	_, err := newInactiveSourceCatalog().load(t, []int64{1, 2}, &period, rejectDriftedSources)
	requireRefusal(t, err, ports.OfferingSourceInactive)

	allInactive := newInactiveSourceCatalog()
	allInactive.offerings[1].IsActive = false
	_, err = allInactive.load(t, []int64{1}, &period, skipInactiveSources)
	requireRefusal(t, err, ports.OfferingSourceInactive)
	_, err = allInactive.load(t, []int64{2, 1}, &period, skipInactiveSources)
	requireRefusal(t, err, ports.OfferingSourceInactive)
	_, err = allInactive.load(t, []int64{1, 4}, &period, skipInactiveSources)
	requireRefusal(t, err, ports.OfferingSourceInactive)

	mixed := newInactiveSourceCatalog()
	mixed.offerings[2].PhaseID = 2
	_, err = mixed.load(t, []int64{1, 2, 3}, &period, skipInactiveSources)
	requireRefusal(t, err, ports.OfferingSourceMixedPhases)

	outside := newInactiveSourceCatalog()
	outside.period.period.EndDate = calendar.NewDate(2026, 8, 31)
	_, err = outside.load(t, []int64{1, 2}, &period, skipInactiveSources)
	requireRefusal(t, err, ports.OfferingSourceOutsidePeriod)

	missingPeriod := newInactiveSourceCatalog()
	missingPeriod.period.err = ports.ErrCatalogRowNotFound
	_, err = missingPeriod.load(t, []int64{1, 2}, &period, skipInactiveSources)
	require.True(t, sourceRefusalRules{}.IsRejection(err), "a vanished period is refused: %v", err)

	missingPhase := newInactiveSourceCatalog()
	missingPhase.phases.err = ports.ErrCatalogRowNotFound
	_, err = missingPhase.load(t, []int64{1, 2}, &period, skipInactiveSources)
	require.True(t, sourceRefusalRules{}.IsRejection(err), "a vanished phase is refused: %v", err)
}

// Technical failures are no refusal: they propagate unchanged, so no caller
// can skip a template over them.
func TestLoadOfferingSources_SkipInactivePropagatesLoadFailures(t *testing.T) {
	t.Parallel()
	period := int64(9)
	dbErr := errors.New("connection reset")

	listFails := newInactiveSourceCatalog()
	listFails.listErr = dbErr
	periodFails := newInactiveSourceCatalog()
	periodFails.period.err = dbErr
	phaseFails := newInactiveSourceCatalog()
	phaseFails.phases.err = dbErr
	for name, catalog := range map[string]inactiveSourceCatalog{"offerings": listFails, "period": periodFails, "phase": phaseFails} {
		_, err := catalog.load(t, []int64{1, 2}, &period, skipInactiveSources)
		require.ErrorIs(t, err, dbErr, "%s lookup", name)
		assert.False(t, sourceRefusalRules{}.IsRejection(err), "%s lookup failure is no refusal", name)
	}
}

// sourceDriftOf maps the resync input onto the loader's policy; the detach
// fallback's tolerance wins over the inactive exception.
func TestSourceDriftOf(t *testing.T) {
	t.Parallel()
	assert.Equal(t, rejectDriftedSources, sourceDriftOf(careplan.OfferingRosterResync{}))
	assert.Equal(t, skipInactiveSources, sourceDriftOf(careplan.OfferingRosterResync{SkipInactiveSources: true}))
	assert.Equal(t, tolerateDriftedSources, sourceDriftOf(careplan.OfferingRosterResync{TolerateDriftedSources: true, SkipInactiveSources: true}))
}

// A template save skips an inactive source only when it was already stored
// (#5313); a newly selected inactive one is refused under its own name, in
// request order, and a save left without a valid source keeps its refusal.
func TestLoadOfferingSources_SkipsOnlyStoredInactiveSources(t *testing.T) {
	t.Parallel()
	period := int64(9)
	catalog := newInactiveSourceCatalog()

	sources, err := catalog.loadStored(t, []int64{1, 2}, &period, []int64{1, 2})
	require.NoError(t, err, "a stored inactive source next to a valid one is skipped")
	assert.Equal(t, []int64{1}, sources.Inactive)
	require.Len(t, sources.Offerings, 1)
	assert.Equal(t, int64(2), sources.Offerings[0].ID)

	_, err = catalog.loadStored(t, []int64{1, 2}, &period, []int64{2})
	requireRefusalNaming(t, err, ports.OfferingSourceInactive, "A")
	_, err = catalog.loadStored(t, []int64{1, 2}, &period, nil)
	requireRefusalNaming(t, err, ports.OfferingSourceInactive, "A")

	newInactive := newInactiveSourceCatalog()
	newInactive.offerings = append(newInactive.offerings, careplan.CareOffering{ID: 4, Name: "D", PhaseID: 1})
	_, err = newInactive.loadStored(t, []int64{1, 2, 4}, &period, []int64{1, 2})
	requireRefusalNaming(t, err, ports.OfferingSourceInactive, "D")

	allInactive := newInactiveSourceCatalog()
	allInactive.offerings[1].IsActive = false
	_, err = allInactive.loadStored(t, []int64{1, 2}, &period, []int64{1, 2})
	requireRefusal(t, err, ports.OfferingSourceInactive)

	mixed := newInactiveSourceCatalog()
	mixed.offerings[2].PhaseID = 2
	_, err = mixed.loadStored(t, []int64{1, 2, 3}, &period, []int64{1, 2, 3})
	requireRefusal(t, err, ports.OfferingSourceMixedPhases)
}

func (c inactiveSourceCatalog) loadStored(t *testing.T, ids []int64, periodID *int64, stored []int64) (careplan.OfferingSources, error) {
	t.Helper()
	catalog := &CareOfferingCatalog{deps: CareOfferingCatalogDependencies{
		Records:     catalogRecordsStub{offerings: c.offerings, err: c.listErr},
		Phases:      c.phases,
		Calendar:    c.period,
		SourceRules: sourceRefusalRules{},
	}}
	return catalog.loadValidatedOfferingSources(t.Context(), ids, periodID, sourceDriftOf(careplan.OfferingRosterResync{StoredOfferingIDs: stored}), stored)
}

func requireRefusalNaming(t *testing.T, err error, kind ports.OfferingSourceRefusalKind, offering string) {
	t.Helper()
	var refusal sourceRefusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, kind, refusal.Kind)
	assert.Equal(t, offering, refusal.Offering)
}

// The stored ids select the stored-only skip; the resync and detach flags
// keep precedence.
func TestSourceDriftOf_StoredOfferingIDs(t *testing.T) {
	t.Parallel()
	assert.Equal(t, skipStoredInactiveSources, sourceDriftOf(careplan.OfferingRosterResync{StoredOfferingIDs: []int64{1}}))
	assert.Equal(t, skipInactiveSources, sourceDriftOf(careplan.OfferingRosterResync{SkipInactiveSources: true, StoredOfferingIDs: []int64{1}}))
	assert.Equal(t, tolerateDriftedSources, sourceDriftOf(careplan.OfferingRosterResync{TolerateDriftedSources: true, StoredOfferingIDs: []int64{1}}))
}

// The catalog learns from a refusal why the template refused its sources.
func TestSourceRefusalProblem(t *testing.T) {
	t.Parallel()
	period := int64(9)
	allInactive := newInactiveSourceCatalog()
	allInactive.offerings[1].IsActive = false
	_, err := allInactive.load(t, []int64{1, 2}, &period, skipInactiveSources)
	assert.Equal(t, careplan.SourcedTemplateNoActiveSource, sourceRefusalProblem(fmt.Errorf("resync: %w", err)))

	mixed := newInactiveSourceCatalog()
	mixed.offerings[2].PhaseID = 2
	_, err = mixed.load(t, []int64{2, 3}, &period, skipInactiveSources)
	assert.Equal(t, careplan.SourcedTemplateMixedPhases, sourceRefusalProblem(err))

	outside := newInactiveSourceCatalog()
	outside.period.period.EndDate = calendar.NewDate(2026, 8, 31)
	_, err = outside.load(t, []int64{2}, &period, skipInactiveSources)
	assert.Equal(t, careplan.SourcedTemplateOutsidePeriod, sourceRefusalProblem(err))

	missingPhase := newInactiveSourceCatalog()
	missingPhase.phases.err = ports.ErrCatalogRowNotFound
	_, err = missingPhase.load(t, []int64{2}, &period, skipInactiveSources)
	assert.Equal(t, careplan.SourcedTemplateSourceInvalid, sourceRefusalProblem(err))
}
