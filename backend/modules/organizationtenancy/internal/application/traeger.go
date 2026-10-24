package application

import (
	"context"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// Träger memberships (ADR 0084, #4649). A membership only counts while the
// Träger itself is active and undeleted: a deactivated Träger lets nobody in
// and shows no school.

// ListAccountTraegerOrganizations returns the active, undeleted Träger where
// the account's membership is active, ordered by name and ID.
func (s *Service) ListAccountTraegerOrganizations(ctx context.Context, accountID int64) ([]organizationtenancy.TraegerOrganization, error) {
	result := []organizationtenancy.TraegerOrganization{}
	if accountID <= 0 {
		return result, nil
	}
	err := s.runRead(ctx, "list_account_traeger_organizations", func(txCtx context.Context, stats *domain.OperationStats) error {
		ids, queryStats, err := s.store.ListActiveAccountTraegerIDs(txCtx, accountID)
		stats.Add(queryStats)
		if err != nil || len(ids) == 0 {
			return err
		}
		organizations, listStats, err := s.store.ListByIDs(txCtx, ids)
		stats.Add(listStats)
		if err != nil {
			return err
		}
		for _, organization := range organizations {
			if organization.Active && !organization.IsDeleted() {
				result = append(result, organizationtenancy.TraegerOrganization{ID: organization.ID, Name: organization.Name})
			}
		}
		return nil
	})
	slices.SortFunc(result, func(a, b organizationtenancy.TraegerOrganization) int {
		if order := strings.Compare(a.Name, b.Name); order != 0 {
			return order
		}
		return int(a.ID - b.ID)
	})
	return result, err
}

// ListTraegerSchoolIDs returns the undeleted schools of an active, undeleted
// Träger. Any other Träger reaches no school.
func (s *Service) ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error) {
	ids := []int64{}
	err := s.runRead(ctx, "list_traeger_school_ids", func(txCtx context.Context, stats *domain.OperationStats) error {
		organization, found, findStats, err := s.store.FindByID(txCtx, organizationID, "")
		stats.Add(findStats)
		if err := liveTraeger(organization, found, err); err != nil {
			return err
		}
		schools, queryStats, err := s.store.ListTraegerSchoolIDs(txCtx, organizationID)
		stats.Add(queryStats)
		ids = schools
		return err
	})
	return ids, err
}

// ListTraegerMemberships returns every membership of the Träger.
func (s *Service) ListTraegerMemberships(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerMembership, error) {
	memberships := []organizationtenancy.TraegerMembership{}
	err := s.runRead(ctx, "list_traeger_memberships", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, findStats, err := s.store.FindByID(txCtx, organizationID, "")
		stats.Add(findStats)
		if err == nil && !found {
			return organizationtenancy.ErrOrganizationNotFound
		}
		if err != nil {
			return err
		}
		rows, queryStats, err := s.store.ListTraegerMemberships(txCtx, organizationID)
		stats.Add(queryStats)
		memberships = rows
		return err
	})
	return memberships, err
}

// ActivateTraegerMembership creates or reactivates the membership at an
// active, undeleted Träger. The Träger row is share-locked so it cannot be
// deleted or deactivated under the grant.
func (s *Service) ActivateTraegerMembership(ctx context.Context, organizationID, accountID int64) (membership organizationtenancy.TraegerMembership, err error) {
	if accountID <= 0 {
		return organizationtenancy.TraegerMembership{}, organizationtenancy.ErrInvalidOrganization
	}
	err = s.run(ctx, "activate_traeger_membership", func(txCtx context.Context, stats *domain.OperationStats) error {
		organization, found, findStats, findErr := s.store.FindByID(txCtx, organizationID, "SHARE")
		stats.Add(findStats)
		if err := liveTraeger(organization, found, findErr); err != nil {
			return err
		}
		var queryStats domain.OperationStats
		membership, queryStats, err = s.store.UpsertActiveTraegerMembership(txCtx, organizationID, accountID)
		stats.Add(queryStats)
		return err
	})
	return membership, err
}

// liveTraeger fails for an unknown, deleted or deactivated Träger.
func liveTraeger(organization organizationtenancy.Organization, found bool, err error) error {
	switch {
	case err != nil:
		return err
	case !found:
		return organizationtenancy.ErrOrganizationNotFound
	case organization.IsDeleted() || !organization.Active:
		return organizationtenancy.ErrOrganizationInactive
	}
	return nil
}
