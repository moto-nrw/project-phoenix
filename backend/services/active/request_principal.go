package active

import (
	"context"
)

// RequestPrincipal is the authenticated identity information attendance needs.
// HasStaff preserves principal presence independently of the numeric ID.
type RequestPrincipal struct {
	DeviceID int64
	StaffID  int64
	HasStaff bool
	IsIoT    bool
}

func (s *service) attendancePrincipal(ctx context.Context) RequestPrincipal {
	if s.PrincipalReader == nil {
		panic("attendance principal reader is not configured")
	}
	return s.PrincipalReader(ctx)
}
