package timetable

import "time"

type engine interface {
	Capability
	StaffRetentionCapability
	ObserveRejection(string, time.Duration, error)
}
