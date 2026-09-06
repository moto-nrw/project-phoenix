package enrollment

import (
	"context"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// schoolClassSettingsStub is a minimal RequestSettingsResolver that only
// answers the one key the concrete-class flow reads. Everything else
// returns the zero/registry-default equivalent.
type schoolClassSettingsStub struct {
	collect bool
}

func (s schoolClassSettingsStub) HasTenantOverride(context.Context, string) (bool, error) {
	return false, nil
}

func (s schoolClassSettingsStub) ResolveBool(_ context.Context, key string) (bool, error) {
	if key == configModel.KeyEnrollmentCollectGradeLevel || key == configModel.KeyEnrollmentCareOfferingsEnabled {
		return true, nil
	}
	if key == configModel.KeyEnrollmentCollectSchoolClass {
		return s.collect, nil
	}
	return false, nil
}

func (s schoolClassSettingsStub) ResolveString(context.Context, string) (string, error) {
	return "", nil
}

func (s schoolClassSettingsStub) ResolveInt(context.Context, string) (int, error) {
	return 0, nil
}

// grade(int16) *int16 is provided by rollover_helpers_test.go.

func classPtr(s string) *string { return &s }

func phaseWithClasses(classes []string, require bool) *capability.Phase {
	return &capability.Phase{
		AvailableSchoolClasses: classes,
		RequireSchoolClass:     require,
	}
}

func TestValidateAndNormalizeSchoolClasses(t *testing.T) {
	t.Parallel()

	classes := []string{"2a", "2b", "3a"}

	tests := []struct {
		name      string
		collect   bool
		phase     *capability.Phase
		child     SubmitChild
		wantErr   bool
		wantClass *string // expected normalized TargetSchoolClass
	}{
		{
			name:      "setting off drops any submitted class",
			collect:   false,
			phase:     phaseWithClasses(classes, true),
			child:     SubmitChild{TargetGradeLevel: grade(3), TargetSchoolClass: classPtr("3a")},
			wantClass: nil,
		},
		{
			name:      "grade 1 stays classless when the phase offers no grade-1 class",
			collect:   true,
			phase:     phaseWithClasses(classes, true),
			child:     SubmitChild{TargetGradeLevel: grade(1), TargetSchoolClass: classPtr("2a")},
			wantClass: nil,
		},
		{
			// #1663: when the phase offers a grade-1 class, grade 1 collects
			// and keeps its concrete class just like grade >= 2.
			name:      "grade 1 keeps its class when the phase offers a grade-1 class",
			collect:   true,
			phase:     phaseWithClasses([]string{"1a", "2a"}, true),
			child:     SubmitChild{TargetGradeLevel: grade(1), TargetSchoolClass: classPtr("  1a  ")},
			wantClass: classPtr("1a"),
		},
		{
			// The grade-1 pick is mandatory once a grade-1 class is offered
			// and the phase requires a class (#1663).
			name:    "grade 1 required with a grade-1 class offered rejects empty",
			collect: true,
			phase:   phaseWithClasses([]string{"1a"}, true),
			child:   SubmitChild{TargetGradeLevel: grade(1), TargetSchoolClass: nil},
			wantErr: true,
		},
		{
			// A grade-1 phase offers grade-1 classes; a mismatched grade-2
			// class must still be rejected by the prefix check (#1663).
			name:    "grade 1 rejects a class from another grade",
			collect: true,
			phase:   phaseWithClasses([]string{"1a", "2a"}, true),
			child:   SubmitChild{TargetGradeLevel: grade(1), TargetSchoolClass: classPtr("2a")},
			wantErr: true,
		},
		{
			name:      "grade 2 optional, empty allowed (Klasse offen)",
			collect:   true,
			phase:     phaseWithClasses(classes, false),
			child:     SubmitChild{TargetGradeLevel: grade(2), TargetSchoolClass: nil},
			wantClass: nil,
		},
		{
			name:    "grade 2 required, empty rejected",
			collect: true,
			phase:   phaseWithClasses(classes, true),
			child:   SubmitChild{TargetGradeLevel: grade(2), TargetSchoolClass: nil},
			wantErr: true,
		},
		{
			name:      "grade 2 valid class from list is kept and trimmed",
			collect:   true,
			phase:     phaseWithClasses(classes, true),
			child:     SubmitChild{TargetGradeLevel: grade(2), TargetSchoolClass: classPtr("  2b  ")},
			wantClass: classPtr("2b"),
		},
		{
			name:    "grade 2 class not offered by phase is rejected",
			collect: true,
			phase:   phaseWithClasses(classes, true),
			child:   SubmitChild{TargetGradeLevel: grade(2), TargetSchoolClass: classPtr("9z")},
			wantErr: true,
		},
		{
			name:      "grade 2 whitespace-only collapses to Klasse offen when optional",
			collect:   true,
			phase:     phaseWithClasses(classes, false),
			child:     SubmitChild{TargetGradeLevel: grade(2), TargetSchoolClass: classPtr("   ")},
			wantClass: nil,
		},
		{
			// The class is in the phase list but belongs to another
			// grade; must be rejected, not written to the student (#1833).
			name:    "grade 2 offered class from a different grade is rejected",
			collect: true,
			phase:   phaseWithClasses(classes, true),
			child:   SubmitChild{TargetGradeLevel: grade(2), TargetSchoolClass: classPtr("3a")},
			wantErr: true,
		},
		{
			// The phase requires a class but only offers grade-3 classes;
			// grade 2 has no matching option, so the required pick is
			// unsatisfiable and must fall back to Klasse offen (nil) rather
			// than reject an otherwise valid submission (#1833).
			name:      "grade 2 required but no matching class collapses to Klasse offen",
			collect:   true,
			phase:     phaseWithClasses([]string{"3a", "3b"}, true),
			child:     SubmitChild{TargetGradeLevel: grade(2), TargetSchoolClass: nil},
			wantClass: nil,
		},
		{
			name:      "grade 3 keeps its own class from a mixed list",
			collect:   true,
			phase:     phaseWithClasses(classes, true),
			child:     SubmitChild{TargetGradeLevel: grade(3), TargetSchoolClass: classPtr("3a")},
			wantClass: classPtr("3a"),
		},
		{
			// A class without a numeric prefix carries no derivable
			// grade, so the plain list check governs and it is kept.
			name:      "non-numeric class name is kept when offered",
			collect:   true,
			phase:     phaseWithClasses([]string{"Bienen", "2a"}, true),
			child:     SubmitChild{TargetGradeLevel: grade(2), TargetSchoolClass: classPtr("Bienen")},
			wantClass: classPtr("Bienen"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &requestService{RequestServiceConfig: RequestServiceConfig{Settings: schoolClassSettingsStub{collect: tc.collect}}}
			children := []SubmitChild{tc.child}
			err := svc.validateAndNormalizeSchoolClasses(context.Background(), tc.phase, children)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (class=%v)", children[0].TargetSchoolClass)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := children[0].TargetSchoolClass
			switch {
			case tc.wantClass == nil && got != nil:
				t.Fatalf("expected nil class, got %q", *got)
			case tc.wantClass != nil && got == nil:
				t.Fatalf("expected class %q, got nil", *tc.wantClass)
			case tc.wantClass != nil && got != nil && *got != *tc.wantClass:
				t.Fatalf("expected class %q, got %q", *tc.wantClass, *got)
			}
		})
	}
}

func TestDecisionService_ResolveSchoolClass(t *testing.T) {
	t.Parallel()

	s := &decisionService{}

	tests := []struct {
		name  string
		child *enrollmentModels.RequestChild
		want  string
	}{
		{
			name:  "concrete class wins over grade",
			child: &enrollmentModels.RequestChild{TargetGradeLevel: grade(2), TargetSchoolClass: classPtr("2a")},
			want:  "2a",
		},
		{
			name:  "empty concrete falls back to grade number",
			child: &enrollmentModels.RequestChild{TargetGradeLevel: grade(3), TargetSchoolClass: classPtr("  ")},
			want:  "3",
		},
		{
			name:  "nil concrete falls back to grade number",
			child: &enrollmentModels.RequestChild{TargetGradeLevel: grade(1)},
			want:  "1",
		},
		{
			name:  "no grade and no class yields neutral placeholder",
			child: &enrollmentModels.RequestChild{},
			want:  "offen",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.resolveSchoolClass(tc.child); got != tc.want {
				t.Fatalf("resolveSchoolClass = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDecisionService_ResolveRolloverSchoolClass(t *testing.T) {
	t.Parallel()

	s := &decisionService{}

	tests := []struct {
		name          string
		child         *enrollmentModels.RequestChild
		existingClass string
		want          string
	}{
		{
			name:          "carried concrete class wins",
			child:         &enrollmentModels.RequestChild{TargetGradeLevel: grade(3), TargetSchoolClass: classPtr("3a")},
			existingClass: "2a",
			want:          "3a",
		},
		{
			name:          "bare placeholder re-derives to bumped grade",
			child:         &enrollmentModels.RequestChild{TargetGradeLevel: grade(2)},
			existingClass: "1",
			want:          "2",
		},
		{
			name:          "empty placeholder re-derives to grade",
			child:         &enrollmentModels.RequestChild{TargetGradeLevel: grade(2)},
			existingClass: "",
			want:          "2",
		},
		{
			name:          "open placeholder re-derives to newly collected grade",
			child:         &enrollmentModels.RequestChild{TargetGradeLevel: grade(2)},
			existingClass: "offen",
			want:          "2",
		},
		{
			name:          "stale concrete class from old grade falls back to placeholder on grade bump",
			child:         &enrollmentModels.RequestChild{TargetGradeLevel: grade(3)},
			existingClass: "2a",
			want:          "3",
		},
		{
			name:          "concrete class already matching target grade is kept",
			child:         &enrollmentModels.RequestChild{TargetGradeLevel: grade(3)},
			existingClass: "3b",
			want:          "3b",
		},
		{
			name:          "concrete class kept on half-year rollover (no grade change)",
			child:         &enrollmentModels.RequestChild{TargetGradeLevel: grade(2)},
			existingClass: "2a",
			want:          "2a",
		},
		{
			name:          "named class without numeric prefix is left untouched",
			child:         &enrollmentModels.RequestChild{TargetGradeLevel: grade(3)},
			existingClass: "Bienen",
			want:          "Bienen",
		},
		{
			name:          "nil target grade keeps existing concrete class",
			child:         &enrollmentModels.RequestChild{},
			existingClass: "2a",
			want:          "2a",
		},
		{
			name:          "zero target grade keeps existing concrete class",
			child:         &enrollmentModels.RequestChild{TargetGradeLevel: grade(0)},
			existingClass: "2a",
			want:          "2a",
		},
		{
			name:          "nil target grade keeps existing bare placeholder",
			child:         &enrollmentModels.RequestChild{},
			existingClass: "2",
			want:          "2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.resolveRolloverSchoolClass(tc.child, tc.existingClass); got != tc.want {
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
