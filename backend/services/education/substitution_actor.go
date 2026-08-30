package education

import (
	"context"
)

type Actor struct {
	StaffID   int64
	TeacherID int64
}

type ActorResolver interface {
	ResolveActor(ctx context.Context) (*Actor, error)
}
