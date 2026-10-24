package schoolmembership_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		want string
		err  error
	}{
		{"none", nil},
		{"not_found", schoolmembership.ErrStaffNotFound},
		{"not_found", schoolmembership.ErrTeacherNotFound},
		{"not_found", schoolmembership.ErrExternalCaregiverRecordNotFound},
		{"invalid", schoolmembership.ErrInvalidMembership},
		{"invalid", &schoolmembership.InvalidMembershipError{Reason: "staff ID is required"}},
		{"membership_conflict", schoolmembership.ErrStaffPersonConflict},
		{"membership_conflict", schoolmembership.ErrTeacherStaffConflict},
		{"membership_conflict", schoolmembership.ErrExternalCaregiverRecordStaffConflict},
		{"personnel_number_conflict", schoolmembership.ErrPersonnelNumberConflict},
		{"internal_error", errors.New("boom")},
	}
	for _, testCase := range cases {
		if got := schoolmembership.ErrorCode(testCase.err); got != testCase.want {
			t.Errorf("ErrorCode(%v) = %q, want %q", testCase.err, got, testCase.want)
		}
	}
}
