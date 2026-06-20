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
}
