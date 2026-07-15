package students

import "testing"

func TestMatchesGradeLevel(t *testing.T) {
	tests := []struct {
		name        string
		schoolClass string
		gradeLevel  int
		want        bool
	}{
		{"filter off (gradeLevel 0) matches everything", "3a", 0, true},
		{"exact grade match", "3a", 3, true},
		{"different grade does not match", "4a", 3, false},
		{"leading-digit boundary: 13a must not match grade 1", "13a", 1, false},
		{"double-digit grade matches", "13a", 13, true},
		{"display label with grade matches", "Klasse 3a", 3, true},
		{"class with no numeric prefix never matches a set grade", "Bienen", 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesGradeLevel(tt.schoolClass, tt.gradeLevel); got != tt.want {
				t.Errorf("matchesGradeLevel(%q, %d) = %v, want %v", tt.schoolClass, tt.gradeLevel, got, tt.want)
			}
		})
	}
}
