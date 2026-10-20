package domain

import (
	"errors"
)

var (

	// ErrActivityInstanceStarted refuses a planning-status change of a block
	// that has a session; the execution must end first.
	ErrActivityInstanceStarted = errors.New("activity instance has started")
)

// Planning states. Execution state is Student Presence's activity session.
const (
	InstanceStatusPlanned   = "planned"
	InstanceStatusCancelled = "cancelled"
)
