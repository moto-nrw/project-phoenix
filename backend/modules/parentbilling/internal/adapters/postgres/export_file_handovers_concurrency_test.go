package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The release of DATEV bookings after a failed handover (ADR 0218 section 6)
// against concurrent transactions: it waits for a running export and runs
// one settlement after the other, so no booking is released that another
// file still holds and none stays on a file that never arrived.

type handoverRace struct {
	t     *testing.T
	ctx   context.Context
	store *Store
	own   int64
	run   int64
	doc   int64
}

func newHandoverRace(t *testing.T, name string) handoverRace {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name,
		Slug: fmt.Sprintf("handover-race-%d", testpkg.UniqueTestTenantID(t))}).ID
	race := handoverRace{t: t, ctx: testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db)),
		store: NewStore(), own: own}
	race.within(func(ctx context.Context) {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own)
		require.NoError(t, err)
		race.run, race.doc = seedHolidayExportRun(t, ctx, tx, own, name, 1000, false)
	})
	return race
}

// within runs fn in a committed Träger transaction of the race's Träger.
func (r handoverRace) within(fn func(ctx context.Context)) {
	r.t.Helper()
	require.NoError(r.t, testpkg.WithinTraeger(r.ctx, r.own, func(ctx context.Context) error {
		fn(ctx)
		return nil
	}))
}

// datevFile records a DATEV file of the run with the booking of the race's
// document, as a download or a handover does.
func (r handoverRace) datevFile(ctx context.Context, all bool) int64 {
	r.t.Helper()
	number, err := r.store.ReserveExportFileNumber(ctx, r.own)
	require.NoError(r.t, err)
	entry, err := r.store.InsertExportFile(ctx, r.own, 0, datevFileRecord(r.run, number, all,
		parentbilling.DatevBookingRef{DocumentID: r.doc}))
	require.NoError(r.t, err)
	return entry.ID
}

func (r handoverRace) bookedOn() []int64 {
	r.t.Helper()
	var files []int64
	r.within(func(ctx context.Context) {
		tx, err := traegerDB(ctx, r.own)
		require.NoError(r.t, err)
		require.NoError(r.t, tx.NewRaw(`SELECT export_file_id FROM parent_billing.datev_booked_documents
			WHERE organization_id = ? AND document_id = ?`, r.own, r.doc).Scan(ctx, &files))
	})
	return files
}

// settleAsync settles a handover as failed in its own transaction and
// reports when it committed.
func (r handoverRace) settleAsync(file int64) chan error {
	done := make(chan error, 1)
	go func() {
		done <- testpkg.WithinTraeger(r.ctx, r.own, func(ctx context.Context) error {
			return r.store.SettleExportFileHandover(ctx, r.own, file, parentbilling.HandoverFailed, "connection_failed")
		})
	}()
	return done
}

// assertWaits checks that a settlement waits for the other transaction.
func assertWaits(t *testing.T, done chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("the settlement did not wait for the other transaction (err %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
}

// An "all bookings" export still running when the handover of the first
// file fails: the settlement waits for it and moves the booking to it.
func TestFailedHandoverWaitsForARunningExport(t *testing.T) {
	t.Parallel()
	race := newHandoverRace(t, "Träger Übergabe Wettlauf Export")
	var first int64
	race.within(func(ctx context.Context) {
		first = race.datevFile(ctx, false)
		require.NoError(t, race.store.InsertExportFileHandover(ctx, race.own, first, kaemmerei))
	})
	locked, release := make(chan struct{}), make(chan struct{})
	exported := make(chan int64, 1)
	go func() {
		require.NoError(t, testpkg.WithinTraeger(race.ctx, race.own, func(ctx context.Context) error {
			exported <- race.datevFile(ctx, true)
			close(locked)
			<-release
			return nil
		}))
	}()
	<-locked
	settled := race.settleAsync(first)
	assertWaits(t, settled)
	close(release)
	require.NoError(t, <-settled)
	assert.Equal(t, []int64{<-exported}, race.bookedOn(), "the booking stays booked, on the export that held it")
}

// Two handovers of the same booking fail at the same time: whichever
// settles second sees the other failed, and the booking is free again.
func TestConcurrentFailedHandoversReleaseTheBooking(t *testing.T) {
	t.Parallel()
	race := newHandoverRace(t, "Träger Übergabe Wettlauf Fehlschlag")
	var first, repeat int64
	race.within(func(ctx context.Context) {
		first = race.datevFile(ctx, false)
		repeat = race.datevFile(ctx, true)
		for _, file := range []int64{first, repeat} {
			require.NoError(t, race.store.InsertExportFileHandover(ctx, race.own, file, kaemmerei))
		}
	})
	locked, release := make(chan struct{}), make(chan struct{})
	repeatDone := make(chan error, 1)
	go func() {
		repeatDone <- testpkg.WithinTraeger(race.ctx, race.own, func(ctx context.Context) error {
			if err := race.store.SettleExportFileHandover(ctx, race.own, repeat, parentbilling.HandoverFailed, "connection_failed"); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	settled := race.settleAsync(first)
	assertWaits(t, settled)
	close(release)
	require.NoError(t, <-repeatDone)
	require.NoError(t, <-settled)
	assert.Empty(t, race.bookedOn(), "no file that arrived holds the booking")
}
