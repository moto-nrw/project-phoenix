package domain

import (
	"time"
)

// OffboardingInstance is one locked instance staff assignment an offboarding
// preview may remove, with the version facts its revision encodes.
type OffboardingInstance struct {
	AssignmentID        int64
	AssignmentUpdatedAt time.Time
	InstanceID          int64
	Date                string
	Status              string
	InstanceUpdatedAt   time.Time
}
