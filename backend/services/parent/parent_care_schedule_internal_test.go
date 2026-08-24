package parent

import (
	"errors"
	"fmt"
	"testing"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// mapCareRequestError is the boundary that translates schedule-domain sentinels
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
		{"not found", scheduleModels.ErrCareRequestNotFound, ErrCareRequestNotFound},
		{"not pending", scheduleModels.ErrCareRequestNotPending, ErrCareRequestNotPending},
		{"already pending", scheduleService.ErrCareRequestAlreadyPending, ErrCareRequestAlreadyPending},
		{"invalid payload", scheduleService.ErrInvalidCareRequestPayload, ErrInvalidCareRequestPayload},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapCareRequestError(tc.in, "create")
			if !errors.Is(got, tc.want) {
				t.Errorf("mapCareRequestError(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMapCareRequestError_WrapsUnknown(t *testing.T) {
	t.Parallel()

	underlying := errors.New("db exploded")
	got := mapCareRequestError(underlying, "withdraw")
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
