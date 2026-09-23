package domain

import (
	"errors"
	"testing"
)

func TestCheckChildQuota(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		limit, before, after int
		refused              *ChildQuotaReachedError
	}{
		"below the Kinderkontingent":         {limit: 3, before: 1, after: 2},
		"reaches it exactly":                 {limit: 3, before: 2, after: 3},
		"goes one past it":                   {limit: 3, before: 3, after: 4, refused: &ChildQuotaReachedError{Booked: 3, Occupied: 3, Requested: 1}},
		"a batch that does not fit":          {limit: 3, before: 2, after: 4, refused: &ChildQuotaReachedError{Booked: 3, Occupied: 2, Requested: 2}},
		"adds nobody while above it":         {limit: 3, before: 5, after: 5},
		"frees a place while above it":       {limit: 3, before: 5, after: 4},
		"adds one while above a lowered one": {limit: 3, before: 5, after: 6, refused: &ChildQuotaReachedError{Booked: 3, Occupied: 5, Requested: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := CheckChildQuota(tc.limit, tc.before, tc.after)
			if tc.refused == nil {
				if err != nil {
					t.Fatalf("unexpected refusal: %v", err)
				}
				return
			}
			var reached *ChildQuotaReachedError
			if !errors.As(err, &reached) || *reached != *tc.refused {
				t.Fatalf("want %+v, got %v", tc.refused, err)
			}
		})
	}
}
