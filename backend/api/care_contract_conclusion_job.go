package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// CareContractConclusionJob builds the Worker's conclusion at the
// acceptance (Weg b, #4680, ADR 0088 section 6) over the ports the portal
// routes bind (NewCareContractConclusion). A run walks every active Träger;
// a Träger whose run fails leaves its children candidates for the next run
// and does not stop the other Träger. After the conclusion the same run
// brings every active contract in line with an approved change of its
// booked care (#4681, Conclusion.AmendFromBookings); the count covers both.
func CareContractConclusionJob(logger *slog.Logger) owners.CareContractConclusionBuilder {
	return func(organizations organizationtenancy.Capability, settings owners.SettingsOwner,
		delivery deliveryRuntime.Capability, outcomes deliveryRuntime.EmailOutcomes, accessLog owners.DataAccessAppender) schedulerPorts.CareContractConclusion {
		if organizations == nil || settings.Traeger == nil || settings.Service == nil || delivery == nil || outcomes == nil || accessLog == nil {
			panic("care contract conclusion: organizations, settings, delivery, the mail outcomes and the access log are required")
		}
		conclusion := NewCareContractConclusion(organizations, settings.Traeger, settings.Service, delivery, outcomes, accessLog)
		return func(ctx context.Context) (int, error) {
			concluded, concludeErr := concludeEveryTraeger(ctx, organizations, conclusion, logger)
			amended, amendErr := amendEveryTraeger(ctx, organizations, conclusion, logger)
			return concluded + amended, errors.Join(concludeErr, amendErr)
		}
	}
}

// concludeEveryTraeger runs the conclusion for every active Träger and
// counts the concluded contracts; the failures of single Träger are joined.
func concludeEveryTraeger(ctx context.Context, organizations organizationtenancy.Capability, conclusion carecontract.Conclusion,
	logger *slog.Logger) (int, error) {
	found, err := organizations.ListOrganizations(ctx)
	if err != nil {
		return 0, fmt.Errorf("care contract conclusion: list Träger: %w", err)
	}
	concluded, failures := 0, []error{}
	for _, organization := range found {
		if !organization.Active || organization.IsDeleted() {
			continue
		}
		run, err := conclusion.ConcludeAccepted(ctx, organization.ID)
		concluded += len(run.Concluded)
		if err != nil {
			failures = append(failures, fmt.Errorf("care contract conclusion of Träger %d: %w", organization.ID, err))
			continue
		}
		if len(run.Skipped) > 0 {
			logger.Debug("care contract conclusion held back",
				"organization_id", organization.ID,
				"skipped", len(run.Skipped),
			)
		}
	}
	return concluded, errors.Join(failures...)
}

// amendEveryTraeger runs the amendments for every active Träger and counts
// the new versions; the failures of single Träger are joined.
func amendEveryTraeger(ctx context.Context, organizations organizationtenancy.Capability, amendments carecontract.Conclusion,
	logger *slog.Logger) (int, error) {
	found, err := organizations.ListOrganizations(ctx)
	if err != nil {
		return 0, fmt.Errorf("care contract amendments: list Träger: %w", err)
	}
	amended, failures := 0, []error{}
	for _, organization := range found {
		if !organization.Active || organization.IsDeleted() {
			continue
		}
		run, err := amendments.AmendFromBookings(ctx, organization.ID)
		amended += len(run.Amended)
		if err != nil {
			failures = append(failures, fmt.Errorf("care contract amendments of Träger %d: %w", organization.ID, err))
			continue
		}
		if len(run.Skipped) > 0 {
			logger.Debug("care contract amendments held back",
				"organization_id", organization.ID,
				"skipped", len(run.Skipped),
			)
		}
	}
	return amended, errors.Join(failures...)
}
