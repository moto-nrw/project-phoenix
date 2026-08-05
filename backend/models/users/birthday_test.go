package users

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// A birthday row is rendered by its name, so a half-filled person must not
// produce a stray leading or trailing space on the card or in the export.
func TestBirthdayEntryFullName(t *testing.T) {
	tests := []struct {
		name  string
		entry BirthdayEntry
		want  string
	}{
		{
			name:  "both names",
			entry: BirthdayEntry{FirstName: "Lina", LastName: "Adler"},
			want:  "Lina Adler",
		},
		{
			name:  "first name missing",
			entry: BirthdayEntry{LastName: "Adler"},
			want:  "Adler",
		},
		{
			name:  "last name missing",
			entry: BirthdayEntry{FirstName: "Lina"},
			want:  "Lina",
		},
		{
			name:  "nothing at all",
			entry: BirthdayEntry{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.entry.FullName())
		})
	}
}

// A birthday recurs annually, so the lookup key drops the year.
func TestMonthDayOf(t *testing.T) {
	assert.Equal(t,
		MonthDay{Month: time.February, Day: 29},
		MonthDayOf(timezone.NewDate(2020, time.February, 29)),
	)
	assert.Equal(t,
		MonthDay{Month: time.August, Day: 5},
		MonthDayOf(timezone.NewDate(1979, time.August, 5)),
	)
}
