package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func phaseWithClasses(classes []string, require bool) *enrollment.Phase {
	return &enrollment.Phase{
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
		phase     *enrollment.Phase
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
			svc := NewIntake(IntakeDependencies{Settings: schoolClassSettings(tc.collect)})
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
