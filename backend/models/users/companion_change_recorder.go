package users

import "context"

// CompanionChangeRecorder answers "did this write actually change a child's
// 'läuft mit' links?" for a caller that does not write the links itself.
//
// Why this exists (#1694): a departure-plan write TRIMS the links on every
// weekday the new plan no longer allows, and those links are rows on ANOTHER
// child's card too — so the write must announce student_companions_changed.
// The reverse is the expensive half: a client reacting to that event discards
// or blocks an in-progress "läuft mit" edit, so announcing it for a write that
// touched no link costs somebody their draft. Callers that reach the shared
// write path through StudentRepository.Update (care-request approval,
// master-data review) cannot tell the two apart from their own payload — a
// payload that carries departure modes is a NECESSARY, not a sufficient,
// condition, because merging the same modes back changes nothing.
//
// Only the write path sees the difference, so it records it here. The HTTP
// student flow needs no recorder: it reconciles the links itself and compares
// the before/after fingerprints (api/students.applyCompanionUpdate).
//
// Scope is one transaction — the recorder lives in the context of the write and
// is never shared across requests or goroutines.
type CompanionChangeRecorder struct {
	changed bool
}

type companionChangeRecorderKey struct{}

// ContextWithCompanionChangeRecorder opens a recording scope and returns both
// the derived context to run the write with and the recorder to read afterwards.
// Writes made with any other context are not recorded.
func ContextWithCompanionChangeRecorder(ctx context.Context) (context.Context, *CompanionChangeRecorder) {
	recorder := &CompanionChangeRecorder{}
	return context.WithValue(ctx, companionChangeRecorderKey{}, recorder), recorder
}

// RecordCompanionChange marks the open recording scope as having changed the
// links. A no-op when the write runs outside a scope, so the write path can
// call it unconditionally.
func RecordCompanionChange(ctx context.Context) {
	if recorder, ok := ctx.Value(companionChangeRecorderKey{}).(*CompanionChangeRecorder); ok && recorder != nil {
		recorder.changed = true
	}
}

// Changed reports whether a write in this scope removed or rewrote a link.
func (r *CompanionChangeRecorder) Changed() bool {
	return r != nil && r.changed
}
