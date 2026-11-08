package application

import (
	"context"
	"errors"
)

var errPortalBookingChanged = errors.New("module booking changed during the check, please retry")

// The parents portal follows the module Eltern-App, the school portal the
// module Unterricht am Vormittag (#4895). Every mint and renewal of their
// sessions checks the booking first, before its session transaction: a
// school that has not booked the module issues no new session and renews
// none. The presented refresh token stays untouched, so booking the module
// again lets the session continue.

// requireSchoolPortal refuses a school-portal session at a school that has
// not booked Unterricht am Vormittag.
func (s *AccountAuthentication) requireSchoolPortal(ctx context.Context, op string, accountID, tenantID int64) error {
	if err := s.portalModules.RequireSchoolPortal(ctx, tenantID); err != nil {
		s.logPortalRefusal(op, accountID, tenantID, err)
		return failed(op, err)
	}
	return nil
}

// bookedGuardianSchools returns the guardian schools of the account that
// booked Eltern-App. An account that is a guardian only at schools without
// the module gets the refusal of its first school.
func (s *AccountAuthentication) bookedGuardianSchools(ctx context.Context, op string, accountID int64, guardianTenantIDs []int64) ([]int64, error) {
	booked, err := s.portalModules.BookedParentPortalSchools(ctx, guardianTenantIDs)
	if err != nil {
		return nil, failed(op+": module booking", err)
	}
	if len(booked) > 0 || len(guardianTenantIDs) == 0 {
		return booked, nil
	}
	refusal := s.portalModules.RequireParentPortal(ctx, guardianTenantIDs[0])
	if refusal == nil {
		// The booking changed between the two reads; the next attempt
		// sees the new state.
		return nil, failed(op+": module booking", errPortalBookingChanged)
	}
	s.logPortalRefusal(op, accountID, guardianTenantIDs[0], refusal)
	return nil, failed(op, refusal)
}

// requireParentPortalForAccount returns the account's guardian schools
// that book Eltern-App for a parents-portal session renewal, and refuses it
// once none of them does. The guard then needs a guardian role at one of
// them.
func (s *AccountAuthentication) requireParentPortalForAccount(ctx context.Context, op string, accountID int64) ([]int64, error) {
	guardianTenantIDs, err := s.findGuardianTenantForAccount(ctx, accountID)
	if err != nil {
		return nil, failed(op+": enumerate tenants", err)
	}
	return s.bookedGuardianSchools(ctx, op, accountID, guardianTenantIDs)
}

func (s *AccountAuthentication) logPortalRefusal(op string, accountID, tenantID int64, err error) {
	s.logger.Info("portal session refused: module not booked",
		"operation", op,
		"account_id", accountID,
		"tenant_id", tenantID,
		"reason", err.Error(),
	)
}
