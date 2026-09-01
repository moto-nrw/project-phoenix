package platform

import (
	"context"

	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
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
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
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
		_, findErr := s.Organizations.FindOrganization(adminCtx, organizationID)
		if findErr != nil {
			return nil, mapOrganizationCapabilityError(findErr, organizationID)
		}
		return s.SummariesRepo.SchoolSummariesByOrganization(adminCtx, organizationID)
	})
}

// PWAPortalUsage is one portal's slice of a school's PWA standalone-usage
// counts.
type PWAPortalUsage struct {
	StandaloneUsers int `json:"standalone_users"`
	EligibleUsers   int `json:"eligible_users"`
}

// SchoolPWAUsage is the per-school PWA standalone-usage aggregate (#2189):
// how many of the school's staff/parent accounts used the app in standalone
// display mode within the window. Deliberately NOT an install count — the
// browser offers no honest install signal.
type SchoolPWAUsage struct {
	WindowDays int            `json:"window_days"`
	Staff      PWAPortalUsage `json:"staff"`
	Parent     PWAPortalUsage `json:"parent"`
}

// GetSchoolPWAUsage returns the school's standalone-usage counts over the
// pwa.UsageWindow. Missing portal buckets stay zero-valued.
func (s *operatorProvisioningService) GetSchoolPWAUsage(ctx context.Context, schoolID int64) (*SchoolPWAUsage, error) {
	return adminTxValue(ctx, s, func(adminCtx context.Context) (*SchoolPWAUsage, error) {
		school, findErr := s.SchoolRepo.FindByID(adminCtx, schoolID)
		if findErr != nil {
			if isLookupNotFound(findErr) {
				return nil, &SchoolNotFoundError{SchoolID: schoolID}
			}
			return nil, findErr
		}
		if school == nil {
			return nil, &SchoolNotFoundError{SchoolID: schoolID}
		}

		rows, err := s.SummariesRepo.PWAUsage(adminCtx, schoolID, pwa.UsageWindow)
		if err != nil {
			return nil, err
		}
		usage := &SchoolPWAUsage{WindowDays: pwa.UsageWindowDays}
		for _, row := range rows {
			portalUsage := PWAPortalUsage{StandaloneUsers: row.StandaloneUsers, EligibleUsers: row.EligibleUsers}
			switch row.Portal {
			case deliveryModels.PushPortalStaff:
				usage.Staff = portalUsage
			case deliveryModels.PushPortalParent:
				usage.Parent = portalUsage
			}
		}
		return usage, nil
	})
}

// ListOrganizationPersons returns all persons across every school belonging to
// the given organization, with school/org context on each row. Returns
// OrganizationNotFoundError if the org does not exist. Replaces the former
// client-side fan-out that issued one request per school.
func (s *operatorProvisioningService) ListOrganizationPersons(ctx context.Context, organizationID int64) ([]OperatorPersonInfo, error) {
	var result []OperatorPersonInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		_, findErr := s.Organizations.FindOrganization(adminCtx, organizationID)
		if findErr != nil {
			return mapOrganizationCapabilityError(findErr, organizationID)
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
