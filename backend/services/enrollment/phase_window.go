package enrollment

import (
	"time"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// IsEnrollmentWindowOpen reports whether the phase's configured enrollment
// window includes `now`. NULL bounds mean "unbounded on that side". Close
// uses half-open semantics: the moment close arrives, the window is closed
// (mirrors care_offering's application-window gate).
//
// The open/close gating decision lives in the service layer (Rule 12:
// models hold data, services hold rules) and takes `now` as a parameter so
// it stays deterministic and testable. The public enrollment form consumes
// this; private admin pages do not, since admins may preview/edit closed
// phases.
func IsEnrollmentWindowOpen(p *enrollmentOwner.Phase, now time.Time) bool {
	if p == nil {
		return false
	}
	if p.EnrollmentOpenAt != nil && now.Before(*p.EnrollmentOpenAt) {
		return false
	}
	if p.EnrollmentCloseAt != nil && !now.Before(*p.EnrollmentCloseAt) {
		return false
	}
	return true
}
