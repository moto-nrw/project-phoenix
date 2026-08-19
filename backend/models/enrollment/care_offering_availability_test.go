package enrollment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCareOfferingAvailabilityRuleMatchesGradeLevel(t *testing.T) {
	t.Parallel()

	grade := func(value int16) *int16 { return &value }
	tests := []struct {
		name  string
		rule  *CareOfferingAvailabilityRule
		grade *int16
		want  bool
	}{
		{name: "no rule", want: true},
		{name: "empty rule", rule: &CareOfferingAvailabilityRule{}, want: true},
		{name: "missing grade never satisfies positive", rule: gradeRule(AvailabilityMatchAll, AvailabilityOperatorIn, 1, 2), want: false},
		{name: "missing grade never satisfies negative", rule: gradeRule(AvailabilityMatchAll, AvailabilityOperatorNotIn, 3, 4), want: false},
		{name: "in matches", rule: gradeRule(AvailabilityMatchAll, AvailabilityOperatorIn, 1, 2), grade: grade(2), want: true},
		{name: "in rejects", rule: gradeRule(AvailabilityMatchAll, AvailabilityOperatorIn, 1, 2), grade: grade(3), want: false},
		{name: "not in matches", rule: gradeRule(AvailabilityMatchAll, AvailabilityOperatorNotIn, 3, 4), grade: grade(2), want: true},
		{name: "not in rejects", rule: gradeRule(AvailabilityMatchAll, AvailabilityOperatorNotIn, 3, 4), grade: grade(3), want: false},
		{
			name: "all requires every condition",
			rule: &CareOfferingAvailabilityRule{Match: AvailabilityMatchAll, Conditions: []CareOfferingAvailabilityCondition{
				{Source: AvailabilitySourceGradeLevel, Operator: AvailabilityOperatorIn, Value: []int{1, 2}},
				{Source: AvailabilitySourceGradeLevel, Operator: AvailabilityOperatorNotIn, Value: []int{2}},
			}},
			grade: grade(2),
			want:  false,
		},
		{
			name: "any accepts one condition",
			rule: &CareOfferingAvailabilityRule{Match: AvailabilityMatchAny, Conditions: []CareOfferingAvailabilityCondition{
				{Source: AvailabilitySourceGradeLevel, Operator: AvailabilityOperatorIn, Value: []int{1}},
				{Source: AvailabilitySourceGradeLevel, Operator: AvailabilityOperatorIn, Value: []int{2}},
			}},
			grade: grade(2),
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.rule.MatchesGradeLevel(tt.grade)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCareOfferingAvailabilityRuleValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rule CareOfferingAvailabilityRule
	}{
		{name: "unknown match", rule: CareOfferingAvailabilityRule{Match: "none", Conditions: []CareOfferingAvailabilityCondition{{Source: AvailabilitySourceGradeLevel, Operator: AvailabilityOperatorIn, Value: []int{1}}}}},
		{name: "unknown source", rule: CareOfferingAvailabilityRule{Match: AvailabilityMatchAll, Conditions: []CareOfferingAvailabilityCondition{{Source: "age", Operator: AvailabilityOperatorIn, Value: []int{1}}}}},
		{name: "unknown operator", rule: CareOfferingAvailabilityRule{Match: AvailabilityMatchAll, Conditions: []CareOfferingAvailabilityCondition{{Source: AvailabilitySourceGradeLevel, Operator: "equals", Value: []int{1}}}}},
		{name: "empty values", rule: CareOfferingAvailabilityRule{Match: AvailabilityMatchAll, Conditions: []CareOfferingAvailabilityCondition{{Source: AvailabilitySourceGradeLevel, Operator: AvailabilityOperatorIn}}}},
		{name: "grade below range", rule: *gradeRule(AvailabilityMatchAll, AvailabilityOperatorIn, 0)},
		{name: "grade above range", rule: *gradeRule(AvailabilityMatchAll, AvailabilityOperatorIn, 14)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.rule.NormalizeAndValidate())
		})
	}
}

func TestCareOfferingAvailabilityRuleNormalizesValues(t *testing.T) {
	t.Parallel()

	rule := gradeRule(AvailabilityMatchAll, AvailabilityOperatorIn, 3, 1, 3, 2)
	require.NoError(t, rule.NormalizeAndValidate())
	require.Equal(t, []int{1, 2, 3}, rule.Conditions[0].Value)
}

func gradeRule(match, operator string, values ...int) *CareOfferingAvailabilityRule {
	return &CareOfferingAvailabilityRule{
		Match: match,
		Conditions: []CareOfferingAvailabilityCondition{{
			Source: AvailabilitySourceGradeLevel, Operator: operator, Value: values,
		}},
	}
}
