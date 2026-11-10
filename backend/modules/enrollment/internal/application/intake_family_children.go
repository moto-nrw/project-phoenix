package application

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The children of the family on the status page of a registration (#6335);
// enrollment.IntakeFamilyChildren states the invariants.

// IntakeFamilyChildrenStore stores the parents' number on the request of
// the caller's school transaction.
type IntakeFamilyChildrenStore interface {
	SetRequestFamilyChildren(ctx context.Context, requestID int64, children int) error
}

var _ enrollmentNative.IntakeFamilyChildren = (*Intake)(nil)

// FamilyChildrenByStatusToken reads the number of the registration behind
// the status token and whether the parents may state it now.
func (s *Intake) FamilyChildrenByStatusToken(ctx context.Context, token string) (enrollmentNative.RegistrationFamilyChildren, error) {
	return s.familyChildren(ctx, token, nil)
}

// StateFamilyChildrenByStatusToken stores the number the parents state on
// their own registration; they can correct it until a child is accepted.
func (s *Intake) StateFamilyChildrenByStatusToken(ctx context.Context, token string, children int) (enrollmentNative.RegistrationFamilyChildren, error) {
	if children < 1 || children > enrollmentNative.MaxFamilyChildren {
		return enrollmentNative.RegistrationFamilyChildren{}, enrollmentNative.ErrFamilyChildrenInvalid
	}
	if s.deps.FamilyChildren == nil {
		return enrollmentNative.RegistrationFamilyChildren{}, errors.New("enrollment: the family children store is not configured")
	}
	return s.familyChildren(ctx, token, &children)
}

// familyChildren resolves the token in the administrative transaction, then
// reads, under the request's lock when it writes, in the school's own
// transaction: the token names exactly one request and nothing else.
func (s *Intake) familyChildren(ctx context.Context, token string, children *int) (enrollmentNative.RegistrationFamilyChildren, error) {
	var req *enrollmentNative.Request
	if err := s.deps.Runtime.AdminTx(ctx, func(adminCtx context.Context) error {
		var err error
		req, err = s.requestByStatusToken(adminCtx, token, false)
		return err
	}); err != nil {
		return enrollmentNative.RegistrationFamilyChildren{}, err
	}
	asked := s.familyChildrenAsked(ctx, req.TenantID)
	var result enrollmentNative.RegistrationFamilyChildren
	tenantCtx := s.deps.Runtime.WithTenant(ctx, req.TenantID)
	err := s.deps.Runtime.TenantTx(tenantCtx, req.TenantID, func(txCtx context.Context) error {
		locked, requestChildren, err := s.lockedStatusByToken(txCtx, token)
		if err != nil {
			return err
		}
		result = enrollmentNative.RegistrationFamilyChildren{Children: locked.FamilyChildren, StatedAt: locked.FamilyChildrenStatedAt,
			Asked: asked}
		result.CanState = result.Asked && familyChildrenOpen(locked, requestChildren)
		result.Accepted = slices.ContainsFunc(requestChildren, func(child *enrollmentNative.RequestChild) bool {
			return child.Status == enrollmentNative.ChildStatusApproved
		})
		if children == nil {
			return nil
		}
		if !result.CanState {
			return enrollmentNative.ErrFamilyChildrenClosed
		}
		if err := s.deps.FamilyChildren.SetRequestFamilyChildren(txCtx, locked.ID, *children); err != nil {
			return err
		}
		stated, _, err := s.lockedStatusByToken(txCtx, token)
		if err != nil {
			return err
		}
		result.Children, result.StatedAt = stated.FamilyChildren, stated.FamilyChildrenStatedAt
		return nil
	})
	if err != nil {
		return enrollmentNative.RegistrationFamilyChildren{}, err
	}
	return result, nil
}

// familyChildrenOpen: the registration takes a number while it is not
// withdrawn, no child is accepted (the contract carries the number from
// then on) and a child still waits for the decision.
func familyChildrenOpen(req *enrollmentNative.Request, children []*enrollmentNative.RequestChild) bool {
	if req.WithdrawnAt != nil {
		return false
	}
	open := false
	for _, child := range children {
		if child.Status == enrollmentNative.ChildStatusApproved {
			return false
		}
		if !child.IsTerminal() {
			open = true
		}
	}
	return open
}

// familyChildrenAsked asks the Träger's purpose outside the school's
// transaction; a failed read asks nothing.
func (s *Intake) familyChildrenAsked(ctx context.Context, schoolID int64) bool {
	if s.deps.FamilyChildrenPurpose == nil {
		return false
	}
	asked, err := s.deps.FamilyChildrenPurpose.FamilyChildrenAsked(ctx, schoolID)
	if err != nil {
		logger := s.logger()
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("enrollment: read whether the Träger counts the family",
			"school_id", schoolID,
			"error", err.Error())
		return false
	}
	return asked
}
