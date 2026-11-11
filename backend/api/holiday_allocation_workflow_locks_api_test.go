// Two-connection tests of the holiday allocation workflow (ADR 0206,
// acceptance criterion 3). A holder connection takes one of the locks the
// workflow needs and the test watches in pg_locks and pg_stat_activity what
// the waiting request already holds: the billing lock B of the period before
// the period row P, never P while it waits for B. Every way also meets the
// release of a holiday run, the other ways of the same period, a change of
// the criteria catalog and a decision of another period without a deadlock
// and without a stale contribution in a released run.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// allocationLockHolder is a second connection that holds a lock until it commits.
// It runs on a private actor pool of its test (#5269), which also runs the
// probes, so no lock actor waits for a connection of the shared pool the
// blocked requests hold.
type allocationLockHolder struct {
	db   *bun.DB
	tx   bun.Tx
	pid  int
	done bool
}

func holdAllocationLock(t *testing.T, db *bun.DB, query string, args ...any) *allocationLockHolder {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	h := &allocationLockHolder{db: db, tx: tx}
	t.Cleanup(func() {
		if !h.done {
			_ = tx.Rollback()
		}
	})
	require.NoError(t, tx.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &h.pid))
	_, err = tx.ExecContext(ctx, query, args...)
	require.NoError(t, err)
	return h
}

func (h *allocationLockHolder) commit(t *testing.T) {
	t.Helper()
	h.done = true
	require.NoError(t, h.tx.Commit())
}

// lockActors is the private pool of a test's lock actors: a holder, its
// probes and the polling, never more than three connections, because the
// subtests of a lock test run one after the other.
func lockActors(t *testing.T) *bun.DB {
	t.Helper()
	db := testpkg.SetupClosableTestDB(t)
	db.SetMaxOpenConns(3)
	return db
}

// waitBlocked waits until n backends wait for the holder and returns them.
func (h *allocationLockHolder) waitBlocked(t *testing.T, n int) []int {
	t.Helper()
	db := h.db
	deadline := time.Now().Add(20 * time.Second)
	for {
		var pids []int
		require.NoError(t, db.NewRaw(`SELECT pid FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid)) ORDER BY pid`, h.pid).
			Scan(context.Background(), &pids))
		if len(pids) >= n {
			return pids
		}
		if time.Now().After(deadline) {
			require.Failf(t, "requests did not wait for the holder", "%d of %d waiting", len(pids), n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// awaitedLocks names the locks a backend waits for: an advisory lock by the
// key it was taken with when it is key, every other one by its type and
// relation.
func awaitedLocks(t *testing.T, db *bun.DB, pid int, key string) []string {
	t.Helper()
	var locks []string
	require.NoError(t, db.NewRaw(`SELECT CASE WHEN locktype = 'advisory' AND objsubid = 1
			AND ((classid::bigint << 32) | objid::bigint) = hashtextextended(?, 0) THEN 'advisory:' || ?
		ELSE locktype || ':' || COALESCE(relation::regclass::text, '') END
		FROM pg_locks WHERE pid = ? AND NOT granted`, key, key, pid).Scan(context.Background(), &locks))
	return locks
}

// billingLockKey is Parent Billing's lock B of a holiday period.
func billingLockKey(organizationID, periodID int64) string {
	return fmt.Sprintf("parent_billing.period_billing:%d:holiday:%d", organizationID, periodID)
}

const holdAdvisoryLock = `SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`

// holdsAdvisoryLock reports whether the backend holds the advisory lock of key.
func holdsAdvisoryLock(t *testing.T, db *bun.DB, pid int, key string) bool {
	t.Helper()
	var held bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid = ? AND locktype = 'advisory' AND granted
		AND objsubid = 1 AND ((classid::bigint << 32) | objid::bigint) = hashtextextended(?, 0))`, pid, key).Scan(context.Background(), &held))
	return held
}

// periodRowFree reports whether nobody holds the period row P: a third
// connection takes it without waiting and lets it go again.
func periodRowFree(t *testing.T, db *bun.DB, periodID int64) bool {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SELECT id FROM enrollment.holiday_periods WHERE id = ? FOR UPDATE NOWAIT`, periodID)
	if err != nil {
		require.Contains(t, err.Error(), "55P03", "only a lock conflict may refuse the probe")
		return false
	}
	return true
}

// async runs a request on its own connection.
func asyncRequest(request func() *httptest.ResponseRecorder) <-chan *httptest.ResponseRecorder {
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- request() }()
	return done
}

// refuseDeadlock fails on a server error, which a deadlock (40P01) would be.
func refuseDeadlock(t *testing.T, rec *httptest.ResponseRecorder, what string) {
	t.Helper()
	assert.Less(t, rec.Code, http.StatusInternalServerError, "%s: %s", what, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "40P01", what)
}

// holidayWay is one of the three commands of the workflow on a fresh fixture.
type holidayWay struct {
	name  string
	setup func(t *testing.T) (reviewFixture, func() *httptest.ResponseRecorder)
}

func holidayWays() []holidayWay {
	return []holidayWay{
		{name: "rebooking", setup: func(t *testing.T) (reviewFixture, func() *httptest.ResponseRecorder) {
			f := newReviewFixture(t, "Träger Sperre Umbuchung")
			first, second := f.offer.Weeks[0], f.offer.Weeks[1]
			f.setCapacity(t, first.ID, 2)
			f.submitPlaces(t, "tom.sperre@example.org", map[string][]int{"Tom": {0}})
			ben := f.list(t, "").Registrations[0].Children[1]
			return f, func() *httptest.ResponseRecorder {
				return traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, f.siblings, ben.ID),
					map[string]any{"bookings": []map[string]any{{"week_id": second.ID, "time_model_id": second.TimeModels[0].ID}}})
			}
		}},
		{name: "capacity", setup: func(t *testing.T) (reviewFixture, func() *httptest.ResponseRecorder) {
			f := newReviewFixture(t, "Träger Sperre Kapazität")
			first := f.offer.Weeks[0]
			f.setCapacity(t, first.ID, 2)
			f.submitPlaces(t, "tom.kapazitaet@example.org", map[string][]int{"Tom": {0}})
			return f, func() *httptest.ResponseRecorder {
				return traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayCapacityPath(f.period.ID, first.ID), map[string]any{"capacity": 3})
			}
		}},
		{name: "decision", setup: func(t *testing.T) (reviewFixture, func() *httptest.ResponseRecorder) {
			a := newAllocationFixture(t, "Träger Sperre Entscheidung")
			a.submitCriteria(t, "tom.entscheidung@example.org", true, false)
			candidate := a.board(t).Weeks[0].Pending[0]
			return a.reviewFixture, func() *httptest.ResponseRecorder { return a.decide(t, decision(candidate, "placed", "")) }
		}},
	}
}

// TestHolidayAllocationWorkflowTakesTheBillingLockBeforeThePeriod observes
// the order B → P of every way: waiting for B it holds no P, waiting for P
// it already holds B.
func TestHolidayAllocationWorkflowTakesTheBillingLockBeforeThePeriod(t *testing.T) {
	t.Parallel()
	actors := lockActors(t)
	for _, way := range holidayWays() {
		t.Run(way.name+"/waits for B without P", func(t *testing.T) {
			f, request := way.setup(t)
			holder := holdAllocationLock(t, actors, holdAdvisoryLock, billingLockKey(f.office.organization, f.period.ID))
			done := asyncRequest(request)
			holder.waitBlocked(t, 1)
			assert.True(t, periodRowFree(t, holder.db, f.period.ID), "a request waiting for B holds no period row")
			holder.commit(t)
			rec := <-done
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		})
		t.Run(way.name+"/waits for P holding B", func(t *testing.T) {
			f, request := way.setup(t)
			holder := holdAllocationLock(t, actors, `SELECT id FROM enrollment.holiday_periods WHERE id = ? FOR UPDATE`, f.period.ID)
			done := asyncRequest(request)
			waiting := holder.waitBlocked(t, 1)
			assert.True(t, holdsAdvisoryLock(t, holder.db, waiting[0], billingLockKey(f.office.organization, f.period.ID)),
				"a request waiting for the period already holds the billing lock")
			holder.commit(t)
			rec := <-done
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		})
	}
}

// releaseWay prepares a confirmed registration with an issued-to-be invoice
// whose contribution the way changes, and a billing run previewed before.
type releaseWay struct {
	name  string
	setup func(t *testing.T) (f reviewFixture, billing string, affected int64, request func() *httptest.ResponseRecorder)
}

func releaseWays() []releaseWay {
	placesSetup := func(t *testing.T, name string) (reviewFixture, string, int64) {
		f := newReviewFixture(t, name)
		billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
		setNumberRange(t, f, billing, "FW-{NNNN}")
		f.setCapacity(t, f.offer.Weeks[0].ID, 2)
		tom := f.submitPlaces(t, "tom.lauf@example.org", map[string][]int{"Tom": {0, 1}})
		require.Equal(t, http.StatusOK, f.confirmWith(t, tom.ID, nil).Code)
		return f, billing, tom.ID
	}
	return []releaseWay{
		{name: "rebooking", setup: func(t *testing.T) (reviewFixture, string, int64, func() *httptest.ResponseRecorder) {
			f, billing, tom := placesSetup(t, "Träger Lauf Umbuchung")
			second := f.offer.Weeks[1]
			ben := f.list(t, "").Registrations[0].Children[1]
			return f, billing, tom, func() *httptest.ResponseRecorder {
				return traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, f.siblings, ben.ID),
					map[string]any{"bookings": []map[string]any{{"week_id": second.ID, "time_model_id": second.TimeModels[0].ID}}})
			}
		}},
		{name: "capacity", setup: func(t *testing.T) (reviewFixture, string, int64, func() *httptest.ResponseRecorder) {
			f, billing, tom := placesSetup(t, "Träger Lauf Kapazität")
			return f, billing, tom, func() *httptest.ResponseRecorder {
				return traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayCapacityPath(f.period.ID, f.offer.Weeks[0].ID), map[string]any{"capacity": 3})
			}
		}},
		{name: "decision", setup: func(t *testing.T) (reviewFixture, string, int64, func() *httptest.ResponseRecorder) {
			a := newAllocationFixture(t, "Träger Lauf Entscheidung")
			billing := traegerColleague(t, a.reviewFixture, identityaccess.TraegerRoleBilling)
			setNumberRange(t, a.reviewFixture, billing, "FE-{NNNN}")
			createContributionRule(t, a.reviewFixture, billing, siblingHalf())
			first := a.submitCriteria(t, "erstes.lauf@example.org", false, false)
			require.Equal(t, http.StatusOK, a.decide(t, decision(a.board(t).Weeks[0].Pending[0], "placed", "")).Code)
			require.Equal(t, http.StatusOK, a.act(t, first.ID, "confirm").Code)
			body := placesRegistration(a.reviewFixture, "zweites.lauf@example.org", map[string][]int{"Ida": {0}})
			body["criteria"] = map[string]bool{fmt.Sprint(a.working): true}
			child := body["children"].([]map[string]any)[0]
			child["criteria"] = map[string]bool{fmt.Sprint(a.sibling): true}
			child["bookings"].([]map[string]any)[0]["time_model_id"] = a.offer.Weeks[0].TimeModels[1].ID
			rec := publicJSON(t, a.public, http.MethodPost, registrationPath(a.period.PublicToken), body)
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			second := decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes())
			require.Equal(t, http.StatusOK, a.merge(t, a.office.token, first.ID, second.ID).Code)
			a.setCapacity(t, a.offer.Weeks[0].ID, 2)
			candidate := a.board(t).Weeks[0].Pending[0]
			return a.reviewFixture, billing, first.ID, func() *httptest.ResponseRecorder { return a.decide(t, decision(candidate, "placed", "")) }
		}},
	}
}

// TestHolidayAllocationWorkflowMeetsARunRelease lets every way and the
// release of a holiday run previewed before it wait for the same billing
// lock. Whichever comes first, neither deadlocks, and the released run
// never bills a contribution the way already replaced: the release either
// refuses the changed run, or it issues the old contribution and the new
// one shows the correction needed.
func TestHolidayAllocationWorkflowMeetsARunRelease(t *testing.T) {
	t.Parallel()
	actors := lockActors(t)
	for _, way := range releaseWays() {
		t.Run(way.name, func(t *testing.T) {
			f, billing, affected, request := way.setup(t)
			before := f.cancelledContribution(t, affected).TotalCents
			run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
			holder := holdAllocationLock(t, actors, holdAdvisoryLock, billingLockKey(f.office.organization, f.period.ID))
			changed := asyncRequest(request)
			released := asyncRequest(func() *httptest.ResponseRecorder {
				return traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
			})
			holder.waitBlocked(t, 2)
			assert.True(t, periodRowFree(t, holder.db, f.period.ID), "neither holds the period while it waits for the billing lock")
			holder.commit(t)
			change, release := <-changed, <-released
			refuseDeadlock(t, change, "the way")
			refuseDeadlock(t, release, "the release")
			require.Equal(t, http.StatusOK, change.Code, change.Body.String())
			after := f.cancelledContribution(t, affected)
			require.NotEqual(t, before, after.TotalCents, "the way changes the contribution")
			if release.Code == http.StatusOK {
				var billed int64
				for _, document := range correctedRun(t, f, billing, run.ID).Documents {
					if document.Kind == "invoice" && document.RegistrationID == affected {
						billed = document.TotalCents
					}
				}
				assert.Equal(t, before, billed, "the run released first bills the contribution it previewed")
				assert.True(t, after.CorrectionNeeded, "the later change shows the difference to the issued invoice")
				return
			}
			require.Equal(t, http.StatusConflict, release.Code, release.Body.String())
			code, _ := registrationRefusalOf(t, release)
			assert.Equal(t, "parent_billing.billing_run_changed", code, "the release refuses a run the way changed")
			assert.NotEqual(t, "released", billingRun(t, f, billing, run.ID).Status)
		})
	}
}

// TestHolidayAllocationWorkflowWaysSerializeInOnePeriod lets a rebooking and
// a new capacity of the same period, and a decision with a new capacity of a
// period by criteria, wait for the same billing lock: both finish without a
// deadlock and the waiting child moves up once.
func TestHolidayAllocationWorkflowWaysSerializeInOnePeriod(t *testing.T) {
	t.Parallel()
	actors := lockActors(t)
	t.Run("rebooking and capacity", func(t *testing.T) {
		f := newReviewFixture(t, "Träger Wege Gleichzeitig")
		first, second := f.offer.Weeks[0], f.offer.Weeks[1]
		f.setCapacity(t, first.ID, 2)
		tom := f.submitPlaces(t, "tom.wege@example.org", map[string][]int{"Tom": {0}})
		ben := f.list(t, "").Registrations[0].Children[1]
		holder := holdAllocationLock(t, actors, holdAdvisoryLock, billingLockKey(f.office.organization, f.period.ID))
		rebooked := asyncRequest(func() *httptest.ResponseRecorder {
			return traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, f.siblings, ben.ID),
				map[string]any{"bookings": []map[string]any{{"week_id": second.ID, "time_model_id": second.TimeModels[0].ID}}})
		})
		widened := asyncRequest(func() *httptest.ResponseRecorder {
			return traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayCapacityPath(f.period.ID, first.ID), map[string]any{"capacity": 3})
		})
		holder.waitBlocked(t, 2)
		assert.True(t, periodRowFree(t, holder.db, f.period.ID))
		holder.commit(t)
		for _, rec := range []*httptest.ResponseRecorder{<-rebooked, <-widened} {
			refuseDeadlock(t, rec, "a way of the same period")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		}
		assert.Empty(t, f.places(t)[first.ID].Waitlist)
		assert.Equal(t, 1, f.mails(t, "holiday_place_granted", tom.ID), "Tom moves up once")
	})
	t.Run("decision and capacity", func(t *testing.T) {
		a := newAllocationFixture(t, "Träger Entscheidung Gleichzeitig")
		registration := a.submitCriteria(t, "tom.gleichzeitig@example.org", true, false)
		candidate := a.board(t).Weeks[0].Pending[0]
		holder := holdAllocationLock(t, actors, holdAdvisoryLock, billingLockKey(a.office.organization, a.period.ID))
		decided := asyncRequest(func() *httptest.ResponseRecorder { return a.decide(t, decision(candidate, "placed", "")) })
		widened := asyncRequest(func() *httptest.ResponseRecorder {
			return traegerJSON(t, a.traeger, a.office.token, http.MethodPut, holidayCapacityPath(a.period.ID, a.offer.Weeks[0].ID), map[string]any{"capacity": 2})
		})
		holder.waitBlocked(t, 2)
		holder.commit(t)
		for _, rec := range []*httptest.ResponseRecorder{<-decided, <-widened} {
			refuseDeadlock(t, rec, "a way of the same period")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		}
		assert.Empty(t, a.board(t).Weeks[0].Pending)
		assert.Equal(t, 1, a.mails(t, "holiday_place_granted", registration.ID))
	})
}

// TestHolidayAllocationWorkflowDecisionMeetsTheCatalogAndAnotherPeriod
// holds the criteria catalog as its writer does and swaps the priority of
// the two criteria. A decision of the period waits there with its own B
// and P and is refused for the proposal it no longer matches; a decision of
// another period of the Träger waits at the same catalog, not at the first
// period, and succeeds.
func TestHolidayAllocationWorkflowDecisionMeetsTheCatalogAndAnotherPeriod(t *testing.T) {
	t.Parallel()
	actors := lockActors(t)
	a := newAllocationFixture(t, "Träger Katalog Gleichzeitig")
	working := a.submitCriteria(t, "berufstaetig.katalog@example.org", true, false)
	a.submitCriteria(t, "geschwister.katalog@example.org", false, true)
	pending := a.board(t).Weeks[0].Pending
	require.Len(t, pending, 2)
	require.Equal(t, working.ID, pending[0].RegistrationID)
	require.Equal(t, "placed", pending[0].Proposal)
	require.Equal(t, "waitlisted", pending[1].Proposal)
	other := otherCriteriaPeriod(t, a)
	other.submitCriteria(t, "anderer.zeitraum@example.org", false, false)
	otherCandidate := other.board(t).Weeks[0].Pending[0]

	holder := holdAllocationLock(t, actors, holdAdvisoryLock, fmt.Sprintf("enrollment.holiday_allocation_criteria:%d", a.office.organization))
	_, err := holder.tx.ExecContext(context.Background(), `UPDATE enrollment.holiday_allocation_criteria
		SET position = CASE id WHEN ? THEN 2 ELSE 1 END WHERE id IN (?, ?)`, a.working, a.working, a.sibling)
	require.NoError(t, err)
	stale := asyncRequest(func() *httptest.ResponseRecorder {
		return a.decide(t, decision(pending[0], "placed", ""), decision(pending[1], "waitlisted", ""))
	})
	otherDecided := asyncRequest(func() *httptest.ResponseRecorder { return other.decide(t, decision(otherCandidate, "placed", "")) })
	waiting := holder.waitBlocked(t, 2)
	catalog := fmt.Sprintf("enrollment.holiday_allocation_criteria:%d", a.office.organization)
	for _, pid := range waiting {
		assert.Equal(t, []string{"advisory:" + catalog}, awaitedLocks(t, holder.db, pid, catalog),
			"both decisions wait for the shared catalog only, never for the other period")
	}
	held := map[int64]int{}
	for _, pid := range waiting {
		for _, period := range []int64{a.period.ID, other.period.ID} {
			if holdsAdvisoryLock(t, holder.db, pid, billingLockKey(a.office.organization, period)) {
				held[period]++
			}
		}
	}
	assert.Equal(t, map[int64]int{a.period.ID: 1, other.period.ID: 1}, held, "each decision holds the billing lock of its own period only")
	assert.False(t, periodRowFree(t, holder.db, a.period.ID), "the decision holds its period while it waits for the catalog")
	holder.commit(t)

	rec := <-stale
	refuseDeadlock(t, rec, "the stale decision")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.holiday_allocation_changed", a.errorCode(t, rec), "a proposal changed by the catalog is refused")
	assert.Len(t, a.board(t).Weeks[0].Pending, 2, "the refused decision writes nothing")
	rec = <-otherDecided
	refuseDeadlock(t, rec, "the other period")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// otherCriteriaPeriod publishes a second period by criteria of the same
// Träger, in the Easter holidays.
func otherCriteriaPeriod(t *testing.T, a allocationFixture) allocationFixture {
	t.Helper()
	body := januaryHolidays(a.office)
	body.Name = "Osterferien 2027"
	body.RegistrationOpensOn, body.RegistrationClosesOn = "2020-01-01", "2099-12-31"
	body.AllocationMode = "criteria"
	for i := range body.Weeks {
		body.Weeks[i].StartsOn = strings.Replace(body.Weeks[i].StartsOn, "2027-01-", "2027-04-", 1)
		body.Weeks[i].EndsOn = strings.Replace(body.Weeks[i].EndsOn, "2027-01-", "2027-04-", 1)
	}
	created := createHolidayPeriod(t, a.traeger, a.office, body)
	rec := traegerJSON(t, a.traeger, a.office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	other := a
	other.period = decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	other.offer = holidayOffer(t, a.holidayRegistrationFixture, other.period.PublicToken)
	return other
}
