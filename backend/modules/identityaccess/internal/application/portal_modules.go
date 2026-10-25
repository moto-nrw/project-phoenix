package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
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

// requireParentPortalForAccount refuses a parents-portal session renewal
// once none of the account's guardian schools books Eltern-App. An account
// without any guardian school keeps the refresh behaviour it had before.
func (s *AccountAuthentication) requireParentPortalForAccount(ctx context.Context, op string, accountID int64) error {
	guardianTenantIDs, err := s.findGuardianTenantForAccount(ctx, accountID)
	if err != nil {
		return failed(op+": enumerate tenants", err)
	}
	_, err = s.bookedGuardianSchools(ctx, op, accountID, guardianTenantIDs)
	return err
}

func (s *AccountAuthentication) logPortalRefusal(op string, accountID, tenantID int64, err error) {
	s.logger.Info("portal session refused: module not booked",
		"operation", op,
		"account_id", accountID,
		"tenant_id", tenantID,
		"reason", err.Error(),
	)
}

// requirePortalBookedForRefresh applies the portal lock to a refresh of a
// parents- or school-portal session; tenant and Träger sessions pass.
func (s *AccountAuthentication) requirePortalBookedForRefresh(ctx context.Context, claims domain.RefreshClaims) error {
	switch claims.Scope {
	case domain.ScopeSchool:
		return s.requireSchoolPortal(ctx, "refresh school session", claims.AccountID, claims.TenantID)
	case domain.ScopeParent:
		return s.requireParentPortalForAccount(ctx, "refresh parent session", claims.AccountID)
	default:
		return nil
	}
}
