package timetable

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The per-occurrence pin wins over the template override; a NULL pin inherits
// the template value; both NULL means "derive from the Betreuungsschlüssel"
// downstream (#1839).
func TestInstanceRequiredStaffOverride(t *testing.T) {
	pin := 5
	tmpl := 3

	tests := []struct {
		name             string
		pin              *int
		templateOverride *int
		want             *int
	}{
		{"pin wins over template override", &pin, &tmpl, &pin},
		{"nil pin inherits template override", nil, &tmpl, &tmpl},
		{"pin without template override", &pin, nil, &pin},
		{"both nil derives downstream", nil, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, instanceRequiredStaffOverride(tt.pin, tt.templateOverride))
		})
	}
}
