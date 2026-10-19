package care

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// MapCareRequestError is the boundary that translates schedule-domain sentinels
// into this package's parent-facing sentinels (which the handler maps to HTTP
// status codes). A miss here would collapse a specific 404/409 into a generic
// 500, so each mapping is pinned.
func TestMapCareRequestError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   error
		want error
	}{
		{"not found", carerequests.ErrNotFound, ErrCareRequestNotFound},
		{"not pending", carerequests.ErrNotPending, ErrCareRequestNotPending},
		{"already pending", carerequests.ErrAlreadyPending, ErrCareRequestAlreadyPending},
		{"invalid payload", carerequests.ErrInvalidPayload, ErrInvalidCareRequestPayload},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MapCareRequestError(tc.in, "create")
			if !errors.Is(got, tc.want) {
				t.Errorf("MapCareRequestError(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMapCareRequestError_WrapsUnknown(t *testing.T) {
	t.Parallel()

	underlying := errors.New("db exploded")
	got := MapCareRequestError(underlying, "withdraw")
	// An unmapped error must NOT masquerade as a known parent sentinel...
	for _, sentinel := range []error{
		ErrCareRequestNotFound, ErrCareRequestNotPending,
		ErrCareRequestAlreadyPending, ErrInvalidCareRequestPayload,
	} {
		if errors.Is(got, sentinel) {
			t.Errorf("unknown error mapped to sentinel %v", sentinel)
		}
	}
	// ...but the operation label and original cause must be preserved for logs.
	if !errors.Is(got, underlying) {
		t.Error("wrapped error lost its cause (errors.Is underlying = false)")
	}
	if want := fmt.Sprintf("parent: withdraw: %v", underlying); got.Error() != want {
		t.Errorf("wrapped message = %q, want %q", got.Error(), want)
	}
}
