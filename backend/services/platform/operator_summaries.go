package platform

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
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

// adminTxValue runs fn inside WithAdminTxOrDirect and captures its value —
// the shared capture-and-return wrapper of the operator summary readers.
func adminTxValue[T any](ctx context.Context, s *operatorProvisioningService, fn func(adminCtx context.Context) (T, error)) (T, error) {
	var result T
	err := tenant.WithAdminTxOrDirect(ctx, s.adminDB(), func(adminCtx context.Context) error {
		v, fnErr := fn(adminCtx)
		if fnErr != nil {
			return fnErr
		}
		result = v
		return nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

// GetProvisioningStats returns platform-wide counts for the operator overview.
func (s *operatorProvisioningService) GetProvisioningStats(ctx context.Context) (*ProvisioningStats, error) {
	return adminTxValue(ctx, s, s.SummariesRepo.Stats)
}

// ListOrganizationSummaries returns all organizations (including soft-deleted)
// with per-row aggregate counts. Counts only include non-deleted child entities.
func (s *operatorProvisioningService) ListOrganizationSummaries(ctx context.Context) ([]*OrganizationSummary, error) {
	return adminTxValue(ctx, s, s.SummariesRepo.OrganizationSummaries)
}

// ListSchoolSummaries returns all schools (global scope) with per-row counts.
func (s *operatorProvisioningService) ListSchoolSummaries(ctx context.Context) ([]*SchoolSummary, error) {
	return adminTxValue(ctx, s, s.SummariesRepo.SchoolSummaries)
}

// ListOrganizationSchoolSummaries returns schools under a specific organization
// with per-row counts. Includes soft-deleted schools so the operator drill-in
// can show them in the Papierkorb. Returns OrganizationNotFoundError if the org
// does not exist.
func (s *operatorProvisioningService) ListOrganizationSchoolSummaries(ctx context.Context, organizationID int64) ([]*SchoolSummary, error) {
	return adminTxValue(ctx, s, func(adminCtx context.Context) ([]*SchoolSummary, error) {
		org, findErr := s.OrganizationRepo.FindByID(adminCtx, organizationID)
		if findErr != nil {
			return nil, findErr
		}
		if org == nil {
			return nil, &OrganizationNotFoundError{OrganizationID: organizationID}
		}
		return s.SummariesRepo.SchoolSummariesByOrganization(adminCtx, organizationID)
	})
}

// ListOrganizationPersons returns all persons across every school belonging to
// the given organization, with school/org context on each row. Returns
// OrganizationNotFoundError if the org does not exist. Replaces the former
// client-side fan-out that issued one request per school.
func (s *operatorProvisioningService) ListOrganizationPersons(ctx context.Context, organizationID int64) ([]OperatorPersonInfo, error) {
	var result []OperatorPersonInfo
	err := tenant.WithAdminTxOrDirect(ctx, s.adminDB(), func(adminCtx context.Context) error {
		org, findErr := s.OrganizationRepo.FindByID(adminCtx, organizationID)
		if findErr != nil {
			return findErr
		}
		if org == nil {
			return &OrganizationNotFoundError{OrganizationID: organizationID}
		}
		persons, scanErr := s.SummariesRepo.PersonsByOrganization(adminCtx, organizationID)
		if scanErr != nil {
			return scanErr
		}
		result = persons
		return nil
	})
	return result, err
}
