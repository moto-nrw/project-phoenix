package application

import (
	"context"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// CreateTestTraeger creates a test Träger with its test schools in one
// administrative unit (#4658): either the whole test system exists or none of
// it. Each school is created like an operator-provisioned one and every row is
// in the operator audit. The sample children follow through Data Import.
func (p *Provisioning) CreateTestTraeger(ctx context.Context, input organizationtenancy.CreateTestTraeger, operatorID int64, clientIP net.IP) (*organizationtenancy.TestTraeger, error) {
	if err := organizationtenancy.NormalizeCreateTestTraeger(&input); err != nil {
		return nil, &organizationtenancy.InvalidProvisioningDataError{Err: err}
	}
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.TestTraeger, error) {
		organization, err := p.organizations.CreateOrganization(adminCtx, organizationtenancy.CreateOrganization{
			Name: input.Name, Slug: input.Slug, Active: true, TestSystem: true,
		})
		if err != nil {
			return nil, mapOrganizationError(err, 0)
		}
		if err := p.recordAction(adminCtx, operatorID, domain.AuditActionCreate, domain.AuditResourceOrganization, &organization.ID, clientIP, map[string]any{
			"name":        organization.Name,
			"slug":        organization.Slug,
			"test_system": true,
		}); err != nil {
			return nil, err
		}
		result := &organizationtenancy.TestTraeger{Organization: organization, Schools: make([]organizationtenancy.School, 0, input.SchoolCount)}
		for _, school := range organizationtenancy.TestSchools(organization, input) {
			created, err := p.createSchool(adminCtx, school, operatorID, clientIP)
			if err != nil {
				return nil, err
			}
			result.Schools = append(result.Schools, *created)
		}
		return result, nil
	})
}

// RecordSampleChildrenRequest names the schools Data Import fills with sample
// children: the undeleted schools of an active test Träger. The request is in
// the operator audit before the first child is written.
func (p *Provisioning) RecordSampleChildrenRequest(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) ([]int64, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]int64, error) {
		organization, err := p.organizations.FindOrganization(adminCtx, organizationID)
		if err != nil {
			return nil, mapOrganizationError(err, organizationID)
		}
		if !organization.TestSystem {
			return nil, organizationtenancy.ErrNotTestTraeger
		}
		ids, err := p.organizations.ListTraegerSchoolIDs(adminCtx, organizationID)
		if err != nil {
			return nil, err
		}
		return ids, p.recordAction(adminCtx, operatorID, domain.AuditActionCreate, domain.AuditResourceSampleChildren, &organizationID, clientIP, map[string]any{
			"school_ids": ids,
		})
	})
}
