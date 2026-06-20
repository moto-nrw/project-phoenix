package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
)

// TestStudentRequestBind_AccompaniedRequiresCompanionNote pins the create-path
// request validation (#1694): a student creation request that allows the
// accompanied ("Mit anderem Kind") departure mode must carry the coupled "mit
// wem" note, so a stale or direct API caller gets a 400 from Bind rather than
// the model invariant surfacing as a 500 on persist.
func TestStudentRequestBind_AccompaniedRequiresCompanionNote(t *testing.T) {
	t.Run("allowed_departure_modes accompanied without note is rejected", func(t *testing.T) {
		modes := users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
		}
		req := &StudentRequest{
			FirstName:             "Max",
			LastName:              "Muster",
			SchoolClass:           "1a",
			AllowedDepartureModes: &modes,
		}
		require.ErrorIs(t, req.Bind(nil), users.ErrDepartureCompanionNoteRequired)
	})

	t.Run("departure_days accompanied with blank note is rejected", func(t *testing.T) {
		days := users.DepartureDays{users.PickupDayMonday: users.DepartureAccompanied}
		blank := "   "
		req := &StudentRequest{
			FirstName:              "Max",
			LastName:               "Muster",
			SchoolClass:            "1a",
			DepartureDays:          &days,
			DepartureCompanionNote: &blank,
		}
		require.ErrorIs(t, req.Bind(nil), users.ErrDepartureCompanionNoteRequired)
	})

	t.Run("accompanied with note is accepted", func(t *testing.T) {
		modes := users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
		}
		note := "Geschwisterkind Lena"
		req := &StudentRequest{
			FirstName:              "Max",
			LastName:               "Muster",
			SchoolClass:            "1a",
			AllowedDepartureModes:  &modes,
			DepartureCompanionNote: &note,
		}
		require.NoError(t, req.Bind(nil))
	})

	t.Run("non-accompanied plan needs no note", func(t *testing.T) {
		modes := users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureBus},
		}
		req := &StudentRequest{
			FirstName:             "Max",
			LastName:              "Muster",
			SchoolClass:           "1a",
			AllowedDepartureModes: &modes,
		}
		require.NoError(t, req.Bind(nil))
	})

	// Validation must follow applyDeparturePlan's persistence precedence:
	// allowed_departure_modes is authoritative and departure_days is ignored when
	// both are present. A client sending a valid new-format allowed payload (no
	// accompanied) while echoing a stale legacy departure_days that carries
	// accompanied without a note must NOT be rejected — the persisted plan would
	// not contain an accompanied day (#1694).
	t.Run("authoritative allowed modes override stale legacy departure_days", func(t *testing.T) {
		modes := users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureBus},
		}
		staleDays := users.DepartureDays{users.PickupDayMonday: users.DepartureAccompanied}
		req := &StudentRequest{
			FirstName:             "Max",
			LastName:              "Muster",
			SchoolClass:           "1a",
			AllowedDepartureModes: &modes,
			DepartureDays:         &staleDays,
		}
		require.NoError(t, req.Bind(nil),
			"blank note must be accepted: the effective (allowed) plan has no accompanied day")
	})

	// The inverse safety case must still reject: allowed modes themselves carry
	// accompanied without a note, even though a legacy departure_days is also
	// present. Precedence does not weaken the accompanied-requires-note rule.
	t.Run("authoritative allowed modes accompanied without note is still rejected", func(t *testing.T) {
		modes := users.AllowedDepartureModes{
			users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
		}
		days := users.DepartureDays{users.PickupDayMonday: users.DepartureBus}
		req := &StudentRequest{
			FirstName:             "Max",
			LastName:              "Muster",
			SchoolClass:           "1a",
			AllowedDepartureModes: &modes,
			DepartureDays:         &days,
		}
		require.ErrorIs(t, req.Bind(nil), users.ErrDepartureCompanionNoteRequired)
	})
}
