package departure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Moved from models/users with the helpers (#2731).
// TestCompanionLinksFingerprint_OrderIndependent pins that the fingerprint
// describes the SET of links: a re-fetch handing the same links back in another
// order (or with the weekdays shuffled) must not read as a foreign change and
// refuse a legitimate save.
func TestCompanionLinksFingerprint_OrderIndependent(t *testing.T) {
	t.Parallel()

	a := []CompanionLink{
		{CompanionStudentID: 42, Weekdays: []string{PickupDayMonday, PickupDayWednesday}},
		{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}},
	}
	b := []CompanionLink{
		{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}},
		{CompanionStudentID: 42, Weekdays: []string{PickupDayWednesday, PickupDayMonday}},
	}

	if got, want := CompanionLinksFingerprint(a), CompanionLinksFingerprint(b); got != want {
		t.Errorf("fingerprint = %q, want %q", got, want)
	}
}

// TestCompanionLinksFingerprint_MirrorsFrontendFormat pins the exact string
// shape, because the client echoes a fingerprint produced by
// companionsFingerprint() in frontend/src/lib/student-companion-api.ts. A
// silent format change on either side would make every save look stale.
func TestCompanionLinksFingerprint_MirrorsFrontendFormat(t *testing.T) {
	t.Parallel()

	links := []CompanionLink{
		{CompanionStudentID: 42, Weekdays: []string{PickupDayWednesday, PickupDayMonday}},
		{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}},
	}

	if got, want := CompanionLinksFingerprint(links), "42:mon,wed|7:fri"; got != want {
		t.Errorf("fingerprint = %q, want %q", got, want)
	}
	if got := CompanionLinksFingerprint(nil); got != "" {
		t.Errorf("empty fingerprint = %q, want \"\"", got)
	}
}

// TestCompanionLinksFingerprint_DetectsChange pins the case the check exists
// for: a link someone else added must not fingerprint like the list without it.
func TestCompanionLinksFingerprint_DetectsChange(t *testing.T) {
	t.Parallel()

	loaded := []CompanionLink{{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}}}
	stored := []CompanionLink{
		{CompanionStudentID: 7, Weekdays: []string{PickupDayFriday}},
		{CompanionStudentID: 9, Weekdays: []string{PickupDayFriday}},
	}

	if CompanionLinksFingerprint(loaded) == CompanionLinksFingerprint(stored) {
		t.Error("an added link must change the fingerprint")
	}
	// A weekday added to an EXISTING link counts as a change too.
	widened := []CompanionLink{{CompanionStudentID: 7, Weekdays: []string{PickupDayThursday, PickupDayFriday}}}
	if CompanionLinksFingerprint(loaded) == CompanionLinksFingerprint(widened) {
		t.Error("an added weekday must change the fingerprint")
	}
}

// TestFormatCompanionLinks pins the offline-list rendering: names carry the
// weekdays unless the link covers the whole week, so a Monday-only
// Laufgemeinschaft cannot be read off a paper list as a standing arrangement.
func TestFormatCompanionLinks(t *testing.T) {
	t.Parallel()

	got := FormatCompanionLinks([]CompanionLink{
		{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{PickupDayMonday, PickupDayTuesday}},
		{CompanionStudentID: 9, FirstName: "Tom", LastName: "Meier", Weekdays: PickupDayOrder},
	})
	if want := "Mia Schulz (Mo, Di), Tom Meier"; got != want {
		t.Errorf("FormatCompanionLinks = %q, want %q", got, want)
	}

	if got := FormatCompanionLinks(nil); got != "" {
		t.Errorf("empty render = %q, want \"\"", got)
	}

	// A link whose names were not joined in still names the child by id rather
	// than rendering an empty "(Mo)".
	if got, want := FormatCompanionLinks([]CompanionLink{
		{CompanionStudentID: 12, Weekdays: []string{PickupDayMonday}},
	}), "Kind #12 (Mo)"; got != want {
		t.Errorf("nameless render = %q, want %q", got, want)
	}
}

// TestCompanionDaysFromLinks folds the submitted list into the cover set the
// model checks against, which is what lets a caller vouch for edges it is about
// to write in the same transaction.
func TestCompanionDaysFromLinks(t *testing.T) {
	t.Parallel()

	days := CompanionDaysFromLinks([]CompanionLink{
		{CompanionStudentID: 11, Weekdays: []string{PickupDayMonday}},
		{CompanionStudentID: 12, Weekdays: []string{PickupDayMonday, PickupDayThursday}},
	})

	require.Equal(t, map[string]bool{PickupDayMonday: true, PickupDayThursday: true}, days)
	require.Empty(t, CompanionDaysFromLinks(nil))
}
