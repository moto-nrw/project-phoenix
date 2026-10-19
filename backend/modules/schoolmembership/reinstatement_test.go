package schoolmembership_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func TestOffboardingReinstatementRequiresPositiveIDs(t *testing.T) {
	t.Parallel()
	called := false
	offboarding := schoolmembership.NewOffboarding(
		func(context.Context, int64) (schoolmembership.RetirementPreview, error) {
			return schoolmembership.RetirementPreview{}, nil
		},
		func(context.Context, int64, string) (schoolmembership.Retirement, error) {
			return schoolmembership.Retirement{}, nil
		},
		nil,
		schoolmembership.Reinstatement{
			Staff: func(context.Context, int64) (schoolmembership.Staff, error) {
				called = true
				return schoolmembership.Staff{}, nil
			},
			Teacher: func(context.Context, int64) (schoolmembership.Teacher, error) {
				called = true
				return schoolmembership.Teacher{}, nil
			},
		},
	)

	_, err := offboarding.ReinstateStaff(context.Background(), 0)
	if !errors.Is(err, schoolmembership.ErrInvalidMembership) {
		t.Fatalf("staff ID 0: want ErrInvalidMembership, got %v", err)
	}
	_, err = offboarding.ReinstateTeacher(context.Background(), -1)
	if !errors.Is(err, schoolmembership.ErrInvalidMembership) {
		t.Fatalf("teacher ID -1: want ErrInvalidMembership, got %v", err)
	}
	if called {
		t.Fatal("an invalid ID must not reach the commands")
	}
}

// A graph that never bound the reinstatement fails closed instead of
// panicking on a nil command.
func TestOffboardingReinstatementUnboundFailsClosed(t *testing.T) {
	t.Parallel()
	offboarding := schoolmembership.NewOffboarding(
		func(context.Context, int64) (schoolmembership.RetirementPreview, error) {
			return schoolmembership.RetirementPreview{}, nil
		},
		func(context.Context, int64, string) (schoolmembership.Retirement, error) {
			return schoolmembership.Retirement{}, nil
		},
		nil, schoolmembership.Reinstatement{},
	)
	if _, err := offboarding.ReinstateStaff(context.Background(), 7); err == nil {
		t.Fatal("an unbound staff reinstatement must fail")
	}
	if _, err := offboarding.ReinstateTeacher(context.Background(), 7); err == nil {
		t.Fatal("an unbound teacher reinstatement must fail")
	}
}
