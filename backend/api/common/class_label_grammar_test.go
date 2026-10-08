package common

import "testing"

func TestParseLeadingGradeLevel_ClassLabelGrammar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		class string
		want  int
	}{
		{"single digit grade", "3a", 3},
		{"double digit grade", "12b", 12},
		{"no numeric prefix", "Bienen", 0},
		{"empty string", "", 0},
		{"leading whitespace trimmed", "  4c", 4},
		{"supported class label prefix", "Klasse 3a", 3},
		{"first numeric run after text", "Stufe 10 Nord", 10},
		{"later digit run ignored", "Klasse 12b3", 12},
		{"non-ASCII digits ignored", "٢a", 0},
		{"integer overflow has no prefill", "999999999999999999999999999999a", 0},
		{"digits only, no letter suffix", "1", 1},
		{"leading-digit boundary: 13a must not match grade 1 via prefix", "13a", 13},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLeadingGradeLevel(tt.class); got != tt.want {
				t.Errorf("ParseLeadingGradeLevel(%q) = %d, want %d", tt.class, got, tt.want)
			}
		})
	}
}
