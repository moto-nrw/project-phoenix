package platform

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// Type aliases keep existing service callers (e.g. api/operator) referencing
// platformSvc.ProvisioningStats / OrganizationSummary / SchoolSummary while the
// underlying definitions live with the other bun-tagged structs in
// models/platform.
type (
	ProvisioningStats   = platform.ProvisioningStats
	OrganizationSummary = platform.OrganizationSummary
	SchoolSummary       = platform.SchoolSummary
)

// GetProvisioningStats returns platform-wide counts for the operator overview.
func (s *operatorProvisioningService) GetProvisioningStats(ctx context.Context) (*ProvisioningStats, error) {
	var result *ProvisioningStats
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		stats, scanErr := s.summariesRepo.Stats(adminCtx)
		if scanErr != nil {
			return scanErr
		}
		result = stats
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListOrganizationSummaries returns all organizations (including soft-deleted)
// with per-row aggregate counts. Counts only include non-deleted child entities.
func (s *operatorProvisioningService) ListOrganizationSummaries(ctx context.Context) ([]*OrganizationSummary, error) {
	var result []*OrganizationSummary
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		summaries, scanErr := s.summariesRepo.OrganizationSummaries(adminCtx)
		if scanErr != nil {
			return scanErr
		}
		result = summaries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListSchoolSummaries returns all schools (global scope) with per-row counts.
func (s *operatorProvisioningService) ListSchoolSummaries(ctx context.Context) ([]*SchoolSummary, error) {
	var result []*SchoolSummary
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		summaries, scanErr := s.summariesRepo.SchoolSummaries(adminCtx)
		if scanErr != nil {
			return scanErr
		}
		result = summaries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListOrganizationSchoolSummaries returns schools under a specific organization
// with per-row counts. Includes soft-deleted schools so the operator drill-in
// can show them in the Papierkorb. Returns OrganizationNotFoundError if the org
// does not exist.
func (s *operatorProvisioningService) ListOrganizationSchoolSummaries(ctx context.Context, organizationID int64) ([]*SchoolSummary, error) {
	var result []*SchoolSummary
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		org, findErr := s.organizationRepo.FindByID(adminCtx, organizationID)
		if findErr != nil {
			return findErr
		}
		if org == nil {
			return &OrganizationNotFoundError{OrganizationID: organizationID}
		}
		summaries, scanErr := s.summariesRepo.SchoolSummariesByOrganization(adminCtx, organizationID)
		if scanErr != nil {
			return scanErr
		}
		result = summaries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListOrganizationPersons returns all persons across every school belonging to
// the given organization, with school/org context on each row. Returns
// OrganizationNotFoundError if the org does not exist. Replaces the former
// client-side fan-out that issued one request per school.
func (s *operatorProvisioningService) ListOrganizationPersons(ctx context.Context, organizationID int64) ([]OperatorPersonInfo, error) {
	var result []OperatorPersonInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		org, findErr := s.organizationRepo.FindByID(adminCtx, organizationID)
		if findErr != nil {
			return findErr
		}
		if org == nil {
			return &OrganizationNotFoundError{OrganizationID: organizationID}
		}
		persons, scanErr := s.summariesRepo.PersonsByOrganization(adminCtx, organizationID)
		if scanErr != nil {
			return scanErr
		}
		result = persons
		return nil
	})
	return result, err
}
