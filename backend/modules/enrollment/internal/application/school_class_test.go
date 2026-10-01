package application

import "testing"

func classPtr(s string) *string { return &s }

func TestResolveSchoolClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		child *RequestChild
		want  string
	}{
		{
			name:  "concrete class wins over grade",
			child: &RequestChild{TargetGradeLevel: grade(2), TargetSchoolClass: classPtr("2a")},
			want:  "2a",
		},
		{
			name:  "empty concrete falls back to grade number",
			child: &RequestChild{TargetGradeLevel: grade(3), TargetSchoolClass: classPtr("  ")},
			want:  "3",
		},
		{
			name:  "nil concrete falls back to grade number",
			child: &RequestChild{TargetGradeLevel: grade(1)},
			want:  "1",
		},
		{
			name:  "no grade and no class yields neutral placeholder",
			child: &RequestChild{},
			want:  "offen",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveSchoolClass(tc.child); got != tc.want {
				t.Fatalf("resolveSchoolClass = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveRolloverSchoolClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		child         *RequestChild
		existingClass string
		want          string
	}{
		{
			name:          "carried concrete class wins",
			child:         &RequestChild{TargetGradeLevel: grade(3), TargetSchoolClass: classPtr("3a")},
			existingClass: "2a",
			want:          "3a",
		},
		{
			name:          "bare placeholder re-derives to bumped grade",
			child:         &RequestChild{TargetGradeLevel: grade(2)},
			existingClass: "1",
			want:          "2",
		},
		{
			name:          "empty placeholder re-derives to grade",
			child:         &RequestChild{TargetGradeLevel: grade(2)},
			existingClass: "",
			want:          "2",
		},
		{
			name:          "open placeholder re-derives to newly collected grade",
			child:         &RequestChild{TargetGradeLevel: grade(2)},
			existingClass: "offen",
			want:          "2",
		},
		{
			name:          "stale concrete class from old grade falls back to placeholder on grade bump",
			child:         &RequestChild{TargetGradeLevel: grade(3)},
			existingClass: "2a",
			want:          "3",
		},
		{
			name:          "concrete class already matching target grade is kept",
			child:         &RequestChild{TargetGradeLevel: grade(3)},
			existingClass: "3b",
			want:          "3b",
		},
		{
			name:          "concrete class kept on half-year rollover (no grade change)",
			child:         &RequestChild{TargetGradeLevel: grade(2)},
			existingClass: "2a",
			want:          "2a",
		},
		{
			name:          "named class without numeric prefix is left untouched",
			child:         &RequestChild{TargetGradeLevel: grade(3)},
			existingClass: "Bienen",
			want:          "Bienen",
		},
		{
			name:          "nil target grade keeps existing concrete class",
			child:         &RequestChild{},
			existingClass: "2a",
			want:          "2a",
		},
		{
			name:          "zero target grade keeps existing concrete class",
			child:         &RequestChild{TargetGradeLevel: grade(0)},
			existingClass: "2a",
			want:          "2a",
		},
		{
			name:          "nil target grade keeps existing bare placeholder",
			child:         &RequestChild{},
			existingClass: "2",
			want:          "2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveRolloverSchoolClass(tc.child, tc.existingClass); got != tc.want {
				t.Fatalf("resolveRolloverSchoolClass(%q) = %q, want %q", tc.existingClass, got, tc.want)
			}
		})
	}
}

func TestIsBareGradePlaceholderClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{"", true},   // no class assigned yet
		{"  ", true}, // whitespace-only placeholder
		{"1", true},  // bare grade number
		{"12", true}, // still all digits
		{"offen", true},
		{" OFFEN ", true},
		{"2a", false}, // hand-assigned concrete class
		{"Klasse", false},
		{" 2a ", false},
	}
	for _, tc := range tests {
		if got := isBareGradePlaceholderClass(tc.in); got != tc.want {
			t.Fatalf("isBareGradePlaceholderClass(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
