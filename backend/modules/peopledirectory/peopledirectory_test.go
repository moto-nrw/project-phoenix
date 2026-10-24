package peopledirectory_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	cases := map[string]error{
		"none":             nil,
		"not_found":        peopledirectory.ErrPersonNotFound,
		"invalid":          &peopledirectory.InvalidPersonError{Reason: "x"},
		"tag_conflict":     peopledirectory.ErrTagConflict,
		"account_conflict": peopledirectory.ErrAccountConflict,
		"internal_error":   errors.New("boom"),
	}
	for want, err := range cases {
		if got := peopledirectory.ErrorCode(err); got != want {
			t.Errorf("ErrorCode(%v) = %q, want %q", err, got, want)
		}
	}
}
