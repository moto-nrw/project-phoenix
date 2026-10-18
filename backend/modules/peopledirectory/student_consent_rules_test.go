package peopledirectory_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// Only a timestamp that appears or disappears is a transition: a grant at
// its own time, a withdrawal at the change.
func TestStudentConsentTransitions(t *testing.T) {
	t.Parallel()

	granted := time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	regranted := granted.Add(time.Hour)
	changedAt := granted.Add(24 * time.Hour)
	before := peopledirectory.StudentConsentSnapshot{AGBAcceptedAt: &granted, EmailContactAcceptedAt: &granted, PhotoConsentGivenAt: &granted}
	after := peopledirectory.StudentConsentSnapshot{AGBAcceptedAt: &regranted, DataProcessingAcceptedAt: &granted, PhotoConsentGivenAt: nil, EmailContactAcceptedAt: &granted}

	got := peopledirectory.StudentConsentTransitions(before, after, changedAt)
	want := []peopledirectory.StudentConsentTransition{
		{Key: peopledirectory.StudentConsentDataProcessing, Granted: true, At: granted},
		{Key: peopledirectory.StudentConsentPhoto, Granted: false, At: changedAt},
	}
	if len(got) != len(want) {
		t.Fatalf("transitions = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("transition %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The photo consent stands withdrawn only while the newest photo event is the
// withdrawal.
func TestPhotoConsentWithdrawnAt(t *testing.T) {
	t.Parallel()

	withdrawn := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	agb := peopledirectory.StudentConsentEvent{Key: peopledirectory.StudentConsentAGB, Withdrawn: true, At: withdrawn.Add(time.Hour)}
	withdrawal := peopledirectory.StudentConsentEvent{Key: peopledirectory.StudentConsentPhoto, Withdrawn: true, At: withdrawn}
	grant := peopledirectory.StudentConsentEvent{Key: peopledirectory.StudentConsentPhoto, At: withdrawn.Add(2 * time.Hour)}

	if at := peopledirectory.PhotoConsentWithdrawnAt([]peopledirectory.StudentConsentEvent{agb, withdrawal, grant}); at == nil || !at.Equal(withdrawn) {
		t.Errorf("withdrawn at = %v, want %v", at, withdrawn)
	}
	if at := peopledirectory.PhotoConsentWithdrawnAt([]peopledirectory.StudentConsentEvent{grant, withdrawal}); at != nil {
		t.Errorf("a later grant stands, got withdrawal at %v", at)
	}
	if at := peopledirectory.PhotoConsentWithdrawnAt(nil); at != nil {
		t.Errorf("no events, got %v", at)
	}
}

// Only an explicit false switches the photos off.
func TestStudentPhotosSwitchedOff(t *testing.T) {
	t.Parallel()

	for value, want := range map[any]bool{false: true, true: false, "false": false, 0: false} {
		if got := peopledirectory.StudentPhotosSwitchedOff(value); got != want {
			t.Errorf("StudentPhotosSwitchedOff(%#v) = %v, want %v", value, got, want)
		}
	}
	if peopledirectory.StudentPhotosSwitchedOff(nil) {
		t.Error("an absent value keeps the photos")
	}
}
