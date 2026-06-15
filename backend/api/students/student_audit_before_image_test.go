package students

import (
	"testing"

	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

// TestApplyStudentFieldUpdates_BeforeImageStable pins the invariant the
// updateStudent audit diff (#1455) relies on: `before := *fresh` is a shallow
// struct copy, so it is only a safe pre-image as long as applyStudentFieldUpdates
// REPLACES the pointer/map fields we track rather than mutating them in place.
//
// If a future refactor made applyDeparturePlan mutate student.DepartureDays in
// place, `before` would alias the mutated map and the change-history diff would
// silently stop recording departure-plan edits. This test fails loudly in that
// case.
func TestApplyStudentFieldUpdates_BeforeImageStable(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	fresh := &usersModel.Student{
		DepartureDays:   usersModel.DepartureDays{"mon": usersModel.DepartureBus},
		SupervisorNotes: strPtr("ursprüngliche Notiz"),
	}

	// Shallow copy — exactly what updateStudent captures before applying the patch.
	before := *fresh

	req := &UpdateStudentRequest{
		DepartureDays:   &usersModel.DepartureDays{"tue": usersModel.DeparturePickup},
		SupervisorNotes: strPtr("geänderte Notiz"),
	}

	applyStudentFieldUpdates(req, fresh)

	// The before-image must still reflect the original values, proving the patch
	// did not mutate shared backing storage.
	assert.Equal(t, usersModel.DepartureBus, before.DepartureDays.ModeFor("mon"),
		"before-image departure plan must be unchanged")
	assert.Equal(t, usersModel.DepartureAlone, before.DepartureDays.ModeFor("tue"),
		"before-image must not see the patched Tuesday pickup")
	assert.Equal(t, "ursprüngliche Notiz", *before.SupervisorNotes,
		"before-image note pointer must still address the original string")

	// Sanity: the patch did take effect on the live row.
	assert.Equal(t, usersModel.DeparturePickup, fresh.DepartureDays.ModeFor("tue"))
	assert.Equal(t, "geänderte Notiz", *fresh.SupervisorNotes)
}
