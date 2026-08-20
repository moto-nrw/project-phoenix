package active

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// recordingWaker haelt fest, welche Kinder geweckt wurden.
type recordingWaker struct {
	calls []struct {
		tenantID  int64
		studentID int64
	}
}

func (r *recordingWaker) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	r.calls = append(r.calls, struct {
		tenantID  int64
		studentID int64
	}{tenantID, studentID})
}

// TestWakeGuardiansAfterCommitIsNoOpWithoutWaker stellt sicher, dass ein nicht
// verdrahteter Weckdienst schweigt statt zu panicken. Tests und CLI-Pfade
// bauen den Active-Service ohne ihn.
func TestWakeGuardiansAfterCommitIsNoOpWithoutWaker(t *testing.T) {
	t.Parallel()

	s := &service{}
	ctx := tenant.WithTenantID(context.Background(), 42)

	s.wakeGuardiansAfterCommit(ctx, 4242) // darf nicht panicken
}

// TestWakeGuardiansAfterCommitSkipsWithoutTenant deckt den Fall ab, dass kein
// Tenant im Kontext liegt: dann waere die Weckung nicht zuzuordnen und
// unterbleibt, statt eine falsche Schule zu benachrichtigen.
func TestWakeGuardiansAfterCommitSkipsWithoutTenant(t *testing.T) {
	t.Parallel()

	waker := &recordingWaker{}
	s := &service{}
	s.SetGuardianWaker(waker)

	s.wakeGuardiansAfterCommit(context.Background(), 4242)

	if len(waker.calls) != 0 {
		t.Fatalf("ohne Tenant im Kontext darf nicht geweckt werden, es gab %d Weckungen", len(waker.calls))
	}
}

// TestWakeGuardiansAfterCommitSkipsInvalidStudent haelt fest, dass eine
// unbrauchbare Kind-ID nicht zu einer Weckung fuehrt.
func TestWakeGuardiansAfterCommitSkipsInvalidStudent(t *testing.T) {
	t.Parallel()

	waker := &recordingWaker{}
	s := &service{}
	s.SetGuardianWaker(waker)
	ctx := tenant.WithTenantID(context.Background(), 42)

	s.wakeGuardiansAfterCommit(ctx, 0)

	if len(waker.calls) != 0 {
		t.Fatalf("ohne gueltige Kind-ID darf nicht geweckt werden, es gab %d Weckungen", len(waker.calls))
	}
}
