package timetable

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateRequiredStaffCount(t *testing.T) {
	tests := []struct {
		name string
		row  templateRow
		want int
	}{
		{
			name: "override wins over the derived requirement when an occurrence exists",
			row: templateRow{
				RequiredStaff:           sql.NullInt64{Int64: 7, Valid: true},
				CapacityEnrollmentCount: 12,
				CapacityOccurrenceFound: true,
			},
			want: 7,
		},
		{
			name: "no override derives from the Betreuungsschlüssel",
			row: templateRow{
				CapacityEnrollmentCount: 12,
				CapacityOccurrenceFound: true,
			},
			want: 2,
		},
		{
			name: "override is suppressed when no occurrence exists in the period",
			row: templateRow{
				RequiredStaff:           sql.NullInt64{Int64: 7, Valid: true},
				CapacityOccurrenceFound: false,
			},
			want: 0,
		},
		{
			name: "no override and no occurrence stays zero",
			row: templateRow{
				CapacityOccurrenceFound: false,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, templateRequiredStaffCount(tt.row, 10))
		})
	}
}
