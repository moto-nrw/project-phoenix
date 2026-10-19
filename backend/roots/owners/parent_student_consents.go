package owners

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

type parentStudentConsentSource interface {
	peopledirectory.StudentConsentQuery
	RecordStudentConsentTransitions(context.Context, peopledirectory.StudentConsentSnapshot, peopledirectory.StudentConsentSnapshot, string, *int64, time.Time) error
}

// NewParentStudentConsents uses the already-read timestamps and the same
// existing consent projection/trail, without another directory lookup.
func NewParentStudentConsents(source parentStudentConsentSource) parentportal.NativeStudentConsents {
	return parentportal.NewNativeStudentConsents(parentportal.StudentConsentCallbacks{
		Current: func(ctx context.Context, value *parentportal.StudentSnapshot, manage bool) ([]parentportal.ChildConsent, error) {
			if value == nil || value.ID <= 0 {
				return nil, fmt.Errorf("student consent reader: persisted student is required")
			}
			states, err := source.CurrentStudentConsents(ctx, parentStudentConsentSnapshot(value), manage)
			if err != nil {
				return nil, err
			}
			out := make([]parentportal.ChildConsent, 0, len(states))
			for _, s := range states {
				out = append(out, parentportal.ChildConsent{Key: s.Key, State: s.State, ChangedAt: s.ChangedAt, CanWithdraw: s.CanWithdraw, CanGrant: s.CanGrant})
			}
			return out, nil
		},
		Record: func(ctx context.Context, before, after *parentportal.StudentSnapshot, kind string, actor *int64, at time.Time) error {
			return source.RecordStudentConsentTransitions(ctx, parentStudentConsentSnapshot(before), parentStudentConsentSnapshot(after), kind, actor, at)
		},
	})
}

func parentStudentConsentSnapshot(value *parentportal.StudentSnapshot) peopledirectory.StudentConsentSnapshot {
	if value == nil {
		return peopledirectory.StudentConsentSnapshot{}
	}
	return peopledirectory.StudentConsentSnapshot{StudentID: value.ID, AGBAcceptedAt: value.AGBAcceptedAt, DataProcessingAcceptedAt: value.DataProcessingAcceptedAt, EmailContactAcceptedAt: value.EmailContactAcceptedAt, PhotoConsentGivenAt: value.PhotoConsentGivenAt}
}
