package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestApplyDueClassSwitchesLocksGatesBeforeReadingPlans(t *testing.T) {
	t.Parallel()

	var calls []string
	decisions := NewDecisions(DecisionDependencies{
		Children: classSwitchOrderChildren{calls: &calls},
		StudentEnrollment: classSwitchOrderStudentEnrollment{
			calls: &calls,
		},
		LockTemplateRecurrence: func(context.Context) error {
			calls = append(calls, "recurrence")
			return nil
		},
	})

	applied, err := decisions.ApplyDueClassSwitches(t.Context(), calendar.NewDate(2030, 8, 1))

	require.NoError(t, err)
	assert.Zero(t, applied)
	assert.Equal(t, []string{"class-writes", "recurrence", "due-plans"}, calls)
}

type classSwitchOrderChildren struct {
	DecisionChildren
	calls *[]string
}

func (c classSwitchOrderChildren) DueClassSwitches(context.Context, enrollment.Date) ([]enrollment.DueClassSwitch, error) {
	*c.calls = append(*c.calls, "due-plans")
	return nil, nil
}

type classSwitchOrderStudentEnrollment struct {
	DecisionStudentEnrollment
	calls *[]string
}

func (s classSwitchOrderStudentEnrollment) LockEnrollmentClassWrites(context.Context) error {
	*s.calls = append(*s.calls, "class-writes")
	return nil
}
