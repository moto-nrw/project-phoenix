package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// GuardianWaker weckt die Sorgeberechtigten eines Kindes, wenn sich dessen
// Anwesenheit geaendert hat.
//
// Bewusst ein schmales Interface hier statt eines Imports von
// services/parentmessaging: der Active-Service soll nichts ueber das
// Eltern-Messaging wissen, und die Factory verdrahtet ohnehin beide.
type GuardianWaker interface {
	// BroadcastChildUpdateToGuardians fanned ein
	// message-unabhaengiges parent_child_updated an alle Sorgeberechtigten
	// des Kindes aus. Best effort, nie blockierend.
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
}

// SetGuardianWaker injiziert den Weckdienst nach der Konstruktion, wie
// SetSettingsService. Bleibt er nil, passiert schlicht nichts.
func (s *service) SetGuardianWaker(waker GuardianWaker) {
	s.guardianWaker = waker
}

// wakeGuardiansAfterCommit meldet den Sorgeberechtigten nach dem Commit, dass
// sich die Anwesenheit ihres Kindes geaendert hat, damit eine offene
// Eltern-App den Tagesstatus (#2252) sofort nachlaedt statt bis zum naechsten
// Fokuswechsel eine veraltete Aussage zu zeigen.
//
// Nach dem Commit, weil ein geweckter Client sonst den Stand von vor der
// Schreiboperation lesen wuerde. Die Tenant-ID wird hier im
// Request-Kontext gelesen, nicht im Hook, denn dort ist der Kontext bereits
// abgeraeumt.
func (s *service) wakeGuardiansAfterCommit(ctx context.Context, studentID int64) {
	if s.guardianWaker == nil || studentID <= 0 {
		return
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return
	}
	waker := s.guardianWaker
	tenant.RegisterAfterCommit(ctx, func() {
		waker.BroadcastChildUpdateToGuardians(tenantID, studentID)
	})
}
