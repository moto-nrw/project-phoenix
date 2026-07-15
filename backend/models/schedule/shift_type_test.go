package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestShiftTypeValidate(t *testing.T) {
	tests := []struct {
		name      string
		input     ShiftType
		wantErr   bool
		wantName  string
		wantColor string
	}{
		{
			name:      "trims name and uppercases hex color",
			input:     ShiftType{Name: "  Betreuung  ", Color: "#83cd2d"},
			wantName:  "Betreuung",
			wantColor: "#83CD2D",
		},
		{
			name:      "prepends missing hash",
			input:     ShiftType{Name: "Pause", Color: "6B7280"},
			wantColor: "#6B7280",
		},
		{
			name:      "expands shorthand hex to six digits",
			input:     ShiftType{Name: "Kurz", Color: "#abc"},
			wantColor: "#AABBCC",
		},
		{
			name:      "expands shorthand hex without hash",
			input:     ShiftType{Name: "Kurz2", Color: "abc"},
			wantColor: "#AABBCC",
		},
		{
			name:      "defaults empty color to gray",
			input:     ShiftType{Name: "Sonstiges", Color: ""},
			wantColor: DefaultShiftTypeColor,
		},
		{
			name:    "rejects empty name",
			input:   ShiftType{Name: "   ", Color: "#83CD2D"},
			wantErr: true,
		},
		{
			name:    "rejects invalid color",
			input:   ShiftType{Name: "Vertretung", Color: "not-a-color"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := tt.input
			err := st.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantName != "" && st.Name != tt.wantName {
				t.Errorf("name = %q, want %q", st.Name, tt.wantName)
			}
			if tt.wantColor != "" && st.Color != tt.wantColor {
				t.Errorf("color = %q, want %q", st.Color, tt.wantColor)
			}
		})
	}
}

func TestShiftType_EntityAccessors(t *testing.T) {
	now := time.Now()
	st := &ShiftType{}
	st.ID = 42
	st.CreatedAt = now
	st.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(42), st.GetID())
	assert.Equal(t, now, st.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), st.GetUpdatedAt())
	// Confirms the base accessors surface the same embedded values.
	var _ base.Entity = st
}
