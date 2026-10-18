package records

import (
	"context"
	"time"
)

// SettingsLookupObserver records bounded lookup evidence without storing values.
type SettingsLookupObserver func(key, cache, outcome string, duration time.Duration)

// ServiceOptions binds optional observations and enrollment restriction reads at construction.
type ServiceOptions struct {
	ObserveLookup         SettingsLookupObserver
	ClassRestrictionGuard func(context.Context) (bool, error)
	GradeRestrictionGuard func(context.Context) (bool, error)
	GradeCapGuard         func(context.Context) (int, error)
}

// SideEffectDispatcher applies the handlers fixed at construction.
type SideEffectDispatcher interface {
	Dispatch(context.Context, int64, string, any) (func(), error)
}

// SideEffectFailureObserver records a failed handler without storing its value.
type SideEffectFailureObserver func(key string)
