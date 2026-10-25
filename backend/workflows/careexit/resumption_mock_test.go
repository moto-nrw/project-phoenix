package careexit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type resumptionOwners struct {
	today   calendar.Date
	ended   calendar.Date
	err     error
	resumed bool
	deleted bool
	unit    bool
}

func (s *resumptionOwners) LockResumptionStudent(context.Context, int64) (careexit.ResumptionStudent, bool, error) {
	if !s.unit {
		return careexit.ResumptionStudent{}, false, errors.New("student lock outside unit")
	}
	return careexit.ResumptionStudent{Status: careplan.StudentStatusInactive, EnrolledUntil: &s.ended}, true, nil
}

func (s *resumptionOwners) ResumeCare(_ context.Context, _ int64, _ calendar.Date, _ string, on calendar.Date) (bool, error) {
	if on != s.today {
		return false, errors.New("membership command did not receive frozen day")
	}
	return s.resumed, s.err
}

func (*resumptionOwners) FindCareExits(_ context.Context, ids []int64) (map[int64]careplan.CareExit, error) {
	return map[int64]careplan.CareExit{ids[0]: {StudentID: ids[0]}}, nil
}

func (s *resumptionOwners) DeleteCareExits(context.Context, []int64) error {
	s.deleted = true
	return nil
}

func (s *resumptionOwners) DiscardCareExitRemovals(context.Context, []int64) error {
	s.deleted = true
	return nil
}

func (*resumptionOwners) RecordCareEndChange(context.Context, int64, *calendar.Date, *calendar.Date, int64) error {
	return nil
}

func TestResumptionPreservesLedgerWhenMembershipRefuses(t *testing.T) {
	t.Parallel()
	for _, commandErr := range []error{nil, errors.New("membership unavailable")} {
		t.Run(map[bool]string{true: "conditional guard", false: "command error"}[commandErr == nil], func(t *testing.T) {
			today := calendar.NewDate(2026, 10, 23)
			owners := &resumptionOwners{today: today, ended: today.AddDays(-1), err: commandErr}
			workflow := careexit.NewResumption(owners, owners, owners, func(ctx context.Context, fn func(context.Context) error) error {
				owners.unit = true
				return fn(ctx)
			}, func() calendar.Date { return today }, nil)
			err := workflow.Resume(context.Background(), careplan.CareResumeInput{StudentID: 42, NewStart: today, Checked: true})
			if commandErr == nil {
				require.ErrorIs(t, err, careplan.ErrCareResumeNotEnded)
			} else {
				require.ErrorIs(t, err, commandErr)
			}
			assert.False(t, owners.deleted)
		})
	}
}

func TestResumptionValidatesBeforeOpeningUnit(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 10, 23)
	for _, tc := range []struct {
		name  string
		input careplan.CareResumeInput
		want  error
	}{
		{"missing child", careplan.CareResumeInput{NewStart: today, Checked: true}, careplan.ErrCareExitNoStudents},
		{"unchecked", careplan.CareResumeInput{StudentID: 42, NewStart: today}, careplan.ErrCareResumeNotChecked},
		{"past start", careplan.CareResumeInput{StudentID: 42, NewStart: today.AddDays(-1), Checked: true}, careplan.ErrCareResumeStartInPast},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owners := &resumptionOwners{}
			workflow := careexit.NewResumption(owners, owners, owners, func(context.Context, func(context.Context) error) error {
				t.Fatal("invalid input must not open a transaction")
				return nil
			}, func() calendar.Date { return today }, nil)
			require.ErrorIs(t, workflow.Resume(context.Background(), tc.input), tc.want)
		})
	}
}
