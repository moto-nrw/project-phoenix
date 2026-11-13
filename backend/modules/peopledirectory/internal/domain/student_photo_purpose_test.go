package domain

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// TestPhotoPurposeEndedOn pins the start of the photo's deletion (#6650):
// the day after the last care day, or the graduation. The nightly run
// re-checks this under the row lock, so a child taken back into care
// between its selection and its lock keeps the photo.
func TestPhotoPurposeEndedOn(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, time.November, 11)
	for _, tc := range []struct {
		name   string
		record StudentRecord
		ended  bool
	}{
		{"last care day yesterday", StudentRecord{Status: "inactive", EnrolledUntil: "2026-11-10"}, true},
		{"last care day today", StudentRecord{Status: "active", EnrolledUntil: "2026-11-11"}, false},
		{"planned last care day", StudentRecord{Status: "active", EnrolledUntil: "2027-07-31"}, false},
		{"no recorded end", StudentRecord{Status: "active"}, false},
		{"returned child without end", StudentRecord{Status: "pending"}, false},
		{"graduate without end", StudentRecord{Status: StudentStatusAlumnus}, true},
		{"graduate with a later end", StudentRecord{Status: StudentStatusAlumnus, EnrolledUntil: "2027-07-31"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.ended, PhotoPurposeEndedOn(tc.record, today))
		})
	}
}

func TestHoldsPhotoData(t *testing.T) {
	t.Parallel()
	path, at, by := "/uploads/student-photos/a.jpg", time.Date(2026, time.September, 1, 8, 0, 0, 0, calendar.Berlin), int64(7)
	assert.False(t, HoldsPhotoData(StudentRecord{}))
	assert.True(t, HoldsPhotoData(StudentRecord{PhotoPath: &path}))
	assert.True(t, HoldsPhotoData(StudentRecord{PhotoConsentGivenAt: &at}))
	assert.True(t, HoldsPhotoData(StudentRecord{PhotoConsentGivenBy: &by}))
}
