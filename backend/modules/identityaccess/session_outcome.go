package identityaccess

import "errors"

// SessionOutcome is what a refused renewal means for the session. Refresh and
// session validation answer it alike: the same state gets the same status
// and code on both routes.
type SessionOutcome int

const (
	// SessionUnclassified is a read that could not run, not a refusal: the
	// session stays as it is and the caller retries.
	SessionUnclassified SessionOutcome = iota
	// SessionEnded: the session is over and needs a new sign-in (401).
	SessionEnded
	// SessionModuleNotBooked: the school no longer books the portal's module
	// (403 module.not_booked). The token stays usable, so booking the module
	// again lets the session continue.
	SessionModuleNotBooked
	// SessionUndecided: the second-factor state could not be read (503
	// identity.mfa_status_unavailable).
	SessionUndecided
)

// sessionEnders are the refusals after which a session does not renew.
var sessionEnders = []error{
	ErrInvalidToken,
	ErrTokenExpired,
	ErrTokenNotFound,
	ErrAccountNotFound,
	ErrAccountInactive,
	ErrTenantNotFound,
	ErrTenantAccessDenied,
	ErrMustUseSchoolPortal,
	ErrParentMustUseParentPortal,
	ErrAccountNoSchoolPortalRole,
	ErrAccountNoGuardianRole,
	ErrNoTraegerMembership,
	ErrTraegerSchoolSwitchForbidden,
	ErrTraegerSchoolNotFound,
	ErrAccountNotCaterer,
	ErrCatererWithoutSchool,
}

// SessionOutcomeOf classifies a refused refresh or session validation.
func SessionOutcomeOf(err error) SessionOutcome {
	if err == nil {
		return SessionUnclassified
	}
	if _, notBooked := PortalNotBooked(err); notBooked {
		return SessionModuleNotBooked
	}
	if errors.Is(err, ErrMFAStatusUnavailable) {
		return SessionUndecided
	}
	for _, ender := range sessionEnders {
		if errors.Is(err, ender) {
			return SessionEnded
		}
	}
	return SessionUnclassified
}
