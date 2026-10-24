package parentportal

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type StudentConsentCallbacks struct {
	Current func(context.Context, *StudentSnapshot, bool) ([]ChildConsent, error)
	Record  func(context.Context, *StudentSnapshot, *StudentSnapshot, string, *int64, time.Time) error
}
type NativeStudentConsents interface {
	CurrentStates(context.Context, *care.PortalStudent, bool) ([]care.ChildConsent, error)
	RecordTransitions(context.Context, *care.PortalStudent, *care.PortalStudent, string, *int64, time.Time) error
}

func NewNativeStudentConsents(c StudentConsentCallbacks) NativeStudentConsents {
	return nativeStudentConsents{c}
}

type nativeStudentConsents struct{ callbacks StudentConsentCallbacks }

func (s nativeStudentConsents) CurrentStates(ctx context.Context, value *care.PortalStudent, manage bool) ([]care.ChildConsent, error) {
	return s.callbacks.Current(ctx, (*StudentSnapshot)(value), manage)
}
func (s nativeStudentConsents) RecordTransitions(ctx context.Context, before, after *care.PortalStudent, source string, actor *int64, at time.Time) error {
	return s.callbacks.Record(ctx, (*StudentSnapshot)(before), (*StudentSnapshot)(after), source, actor, at)
}

type NativeStudentAudit interface {
	RecordChangesForActor(context.Context, *care.PortalStudent, *care.PortalStudent, int64) error
}

func NewNativeStudentAudit(record func(context.Context, *StudentSnapshot, *StudentSnapshot, int64) error) NativeStudentAudit {
	return nativeStudentAudit{record}
}

type nativeStudentAudit struct {
	record func(context.Context, *StudentSnapshot, *StudentSnapshot, int64) error
}

func (s nativeStudentAudit) RecordChangesForActor(ctx context.Context, before, after *care.PortalStudent, actor int64) error {
	return s.record(ctx, (*StudentSnapshot)(before), (*StudentSnapshot)(after), actor)
}
