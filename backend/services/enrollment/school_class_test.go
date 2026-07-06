package enrollment

import (
	"context"
	"testing"

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

func phaseWithClasses(classes []string, require bool) *enrollmentModels.Phase {
	return &enrollmentModels.Phase{
		AvailableSchoolClasses: classes,
		RequireSchoolClass:     require,
	}
}

func TestValidateAndNormalizeSchoolClasses(t *testing.T) {
	classes := []string{"2a", "2b", "3a"}

	tests := []struct {
		name      string
		collect   bool
		phase     *enrollmentModels.Phase
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
			name:      "grade 1 never keeps a class even when required",
			collect:   true,
			phase:     phaseWithClasses(classes, true),
			child:     SubmitChild{TargetGradeLevel: grade(1), TargetSchoolClass: classPtr("2a")},
			wantClass: nil,
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &requestService{settings: schoolClassSettingsStub{collect: tc.collect}}
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
			name:  "no grade and no class yields empty",
			child: &enrollmentModels.RequestChild{},
			want:  "",
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
