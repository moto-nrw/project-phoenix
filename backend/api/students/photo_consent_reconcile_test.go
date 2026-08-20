package students

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"
)

func TestReconcilePhotoConsentRequest_StaleUnchangedValuesAreNoOp(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		requested bool
		snapshot  *time.Time
		fresh     *time.Time
	}{
		{
			name:      "stale true does not re-grant withdrawn consent",
			requested: true,
			snapshot:  &now,
			fresh:     nil,
		},
		{
			name:      "stale false does not withdraw newly granted consent",
			requested: false,
			snapshot:  nil,
			fresh:     &now,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reconcilePhotoConsentRequest(
				&tc.requested,
				&users.Student{PhotoConsentGivenAt: tc.snapshot},
				&users.Student{PhotoConsentGivenAt: tc.fresh},
			)
			if got != nil {
				t.Fatalf("expected stale unchanged consent value to be ignored, got %v", *got)
			}
		})
	}
}

func TestReconcilePhotoConsentRequest_UserTransitionsStillApply(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		requested bool
		snapshot  *time.Time
		fresh     *time.Time
	}{
		{
			name:      "grant from no consent",
			requested: true,
			snapshot:  nil,
			fresh:     nil,
		},
		{
			name:      "withdraw existing consent",
			requested: false,
			snapshot:  &now,
			fresh:     &now,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reconcilePhotoConsentRequest(
				&tc.requested,
				&users.Student{PhotoConsentGivenAt: tc.snapshot},
				&users.Student{PhotoConsentGivenAt: tc.fresh},
			)
			if got == nil || *got != tc.requested {
				t.Fatalf("expected transition %v to survive, got %v", tc.requested, got)
			}
		})
	}
}
