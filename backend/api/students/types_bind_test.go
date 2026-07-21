package students

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
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

// TestUpdateStudentRequestBind_CompanionCount pins the companion-list bound at
// the request boundary. The service enforces the same cap, but only AFTER the
// handler has row-locked every submitted id — a caller could otherwise name
// hundreds of children and hold their rows until the eventual 400.
func TestUpdateStudentRequestBind_CompanionCount(t *testing.T) {
	entries := func(n int) *[]CompanionEntry {
		list := make([]CompanionEntry, 0, n)
		for i := range n {
			list = append(list, CompanionEntry{
				CompanionStudentID: int64(i + 1),
				Weekdays:           []string{"mon"},
			})
		}
		return &list
	}

	// The fingerprint is mandatory alongside a list (see the dedicated test
	// below), so it travels with every fixture that expects acceptance.
	fingerprint := ""

	t.Run("a list at the cap is accepted", func(t *testing.T) {
		req := &UpdateStudentRequest{
			Companions:            entries(usersService.MaxStudentCompanions),
			CompanionsFingerprint: &fingerprint,
		}
		require.NoError(t, req.Bind(nil))
	})

	t.Run("a list over the cap is rejected before any lock", func(t *testing.T) {
		req := &UpdateStudentRequest{Companions: entries(usersService.MaxStudentCompanions + 1)}
		require.ErrorIs(t, req.Bind(nil), usersService.ErrTooManyCompanions)
	})

	t.Run("no companions key is untouched", func(t *testing.T) {
		req := &UpdateStudentRequest{}
		require.NoError(t, req.Bind(nil))
	})
}

// TestUpdateStudentRequestBind_CompanionsFingerprint pins the snapshot claim
// that makes the replacement semantics safe for HTTP callers.
//
// The service treats the fingerprint as optional because system callers derive
// the list from the stored state themselves. A user-built list without one gets
// no lost-update protection at all: two staff members starting from the same
// snapshot would both submit everything they saw, and the later write would
// delete the earlier one's committed links without anything objecting.
func TestUpdateStudentRequestBind_CompanionsFingerprint(t *testing.T) {
	list := []CompanionEntry{{CompanionStudentID: 7, Weekdays: []string{"mon"}}}

	t.Run("a list without a fingerprint is rejected", func(t *testing.T) {
		req := &UpdateStudentRequest{Companions: &list}
		require.ErrorIs(t, req.Bind(nil), errCompanionsFingerprintRequired)
	})

	t.Run("the empty fingerprint of an empty list is a claim, not an omission", func(t *testing.T) {
		empty := ""
		req := &UpdateStudentRequest{Companions: &[]CompanionEntry{}, CompanionsFingerprint: &empty}
		require.NoError(t, req.Bind(nil))
	})

	t.Run("no list needs no fingerprint", func(t *testing.T) {
		require.NoError(t, (&UpdateStudentRequest{}).Bind(nil))
	})
}

// TestUpdateStudentRequestTouchesCompanions pins which requests announce
// student_companions_changed.
//
// The event makes every mounted "läuft mit" view refetch, and an EDITING one
// reacts by discarding or blocking the user's draft — so it must fire for
// exactly the writes that can change the links (a submitted list, or a
// departure-plan write, which trims links the new plan no longer allows) and
// stay silent for everything else.
func TestUpdateStudentRequestTouchesCompanions(t *testing.T) {
	firstName := "Max"
	note := "Nachbarskind"
	sick := true
	busKind := true

	t.Run("silent for writes that cannot touch the links", func(t *testing.T) {
		for name, req := range map[string]*UpdateStudentRequest{
			"nothing":   {},
			"name":      {FirstName: &firstName},
			"sick flag": {Sick: &sick},
		} {
			if req.touchesCompanions() {
				t.Errorf("%s must not announce a companion change", name)
			}
		}
	})

	t.Run("fires for a submitted list and for every plan input", func(t *testing.T) {
		entries := []CompanionEntry{}
		modes := users.AllowedDepartureModes{}
		days := users.DepartureDays{}
		pickupDays := users.PickupDays{}
		busDays := users.BusDays{}
		status := "Geht alleine nach Hause"

		for name, req := range map[string]*UpdateStudentRequest{
			"companions":              {Companions: &entries},
			"allowed_departure_modes": {AllowedDepartureModes: &modes},
			"departure_days":          {DepartureDays: &days},
			"companion note":          {DepartureCompanionNote: &note},
			// The legacy inputs resolve into the same plan, so they can take
			// the accompanied mode — and with it the links — away.
			"pickup_status": {PickupStatus: &status},
			"pickup_days":   {PickupDays: &pickupDays},
			"bus":           {Bus: &busKind},
			"bus_days":      {BusDays: &busDays},
		} {
			if !req.touchesCompanions() {
				t.Errorf("%s must announce a companion change", name)
			}
		}
	})
}

// TestCompanionEntryUnmarshal_AcceptsStringIDs pins the wire contract for the
// companion id: the frontend carries every backend int64 as a string, and
// converting it back to a JS number rounds ids beyond Number.MAX_SAFE_INTEGER
// into a DIFFERENT child's id — so a quoted decimal has to be accepted verbatim
// alongside the plain number older clients send.
func TestCompanionEntryUnmarshal_AcceptsStringIDs(t *testing.T) {
	// Above 2^53: a JS client cannot round-trip this through a number at all.
	const beyondSafeInteger = int64(9007199254740993)

	cases := map[string]struct {
		body string
		want int64
	}{
		"quoted id":  {`{"companion_student_id":"7","weekdays":["mon"]}`, 7},
		"numeric id": {`{"companion_student_id":7,"weekdays":["mon"]}`, 7},
		"quoted id beyond 2^53": {
			`{"companion_student_id":"9007199254740993","weekdays":["mon"]}`,
			beyondSafeInteger,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var entry CompanionEntry
			require.NoError(t, json.Unmarshal([]byte(tc.body), &entry))
			require.Equal(t, tc.want, entry.CompanionStudentID)
			require.Equal(t, []string{"mon"}, entry.Weekdays)
		})
	}

	t.Run("a non-numeric id is refused with the German sentinel", func(t *testing.T) {
		var entry CompanionEntry
		err := json.Unmarshal([]byte(`{"companion_student_id":"abc","weekdays":["mon"]}`), &entry)
		require.ErrorIs(t, err, users.ErrCompanionStudentIDRequired)
	})

	t.Run("a missing id stays zero for Bind to reject", func(t *testing.T) {
		var entry CompanionEntry
		require.NoError(t, json.Unmarshal([]byte(`{"weekdays":["mon"]}`), &entry))
		require.Zero(t, entry.CompanionStudentID)
	})
}
