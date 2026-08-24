package schoolclass

import "testing"

func TestGradePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		class string
		want  string
	}{
		{"single digit grade", "3a", "3"},
		{"double digit grade", "12b", "12"},
		{"no numeric prefix", "Bienen", ""},
		{"empty string", "", ""},
		{"leading whitespace trimmed", "  4c", "4"},
		{"supported class label prefix", "Klasse 3a", "3"},
		{"first numeric run after text", "Stufe 10 Nord", "10"},
		{"digits only, no letter suffix", "1", "1"},
		{"leading-digit boundary: 13a must not match grade 1 via prefix", "13a", "13"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GradePrefix(tt.class); got != tt.want {
				t.Errorf("GradePrefix(%q) = %q, want %q", tt.class, got, tt.want)
			}
		})
	}
}
