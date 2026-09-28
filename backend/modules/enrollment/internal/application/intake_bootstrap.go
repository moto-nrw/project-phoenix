package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The form loads assemble their data inside one tenant transaction: the gated
// phase, its pinned schema, the active care offerings, the form capabilities,
// the legal contract and the late invite a link carries.

type publicBootstrapOptions struct {
	loadSchema      bool
	loadLegal       bool
	classifyStages  bool
	lateInviteToken string
	lateInviteNow   time.Time
	legalLoader     func(ctx context.Context) (enrollment.LegalTexts, error)
}

// isPublicGateError reports whether err is one of the public enrollment
// gates, which answer 404 rather than 500. ErrPhaseAudienceRestricted is one
// too: an admin can restrict a phase between the bootstrap's phase load and
// its legal reload (#1663).
func isPublicGateError(err error) bool {
	return errors.Is(err, enrollment.ErrInvalidSubmission) ||
		errors.Is(err, enrollment.ErrEnrollmentDisabled) ||
		errors.Is(err, enrollment.ErrEnrollmentWindowClosed) ||
		errors.Is(err, enrollment.ErrLateInviteInvalid) ||
		errors.Is(err, enrollment.ErrPhaseAudienceRestricted)
}

func (s *Intake) assemblePublicBootstrap(
	ctx context.Context,
	phaseLoader func(ctx context.Context) (*enrollment.Phase, error),
	opts publicBootstrapOptions,
) (*enrollment.PublicFormBootstrapData, error) {
	phase, err := phaseLoader(ctx)
	if err != nil {
		return nil, err
	}
	capabilities, err := s.formCapabilities(ctx)
	if err != nil {
		if opts.classifyStages {
			return nil, &enrollment.BootstrapStageError{Stage: enrollment.BootstrapStageCapabilities, Err: err}
		}
		return nil, err
	}
	data := &enrollment.PublicFormBootstrapData{Phase: phase, Capabilities: capabilities, Offerings: []*enrollment.CareOffering{}}
	if data.LateInvite, err = s.bootstrapLateInvite(ctx, phase.ID, opts); err != nil {
		return nil, err
	}
	if opts.loadSchema {
		if data.Schema, err = s.resolveSubmissionSchema(ctx, phase); err != nil {
			return nil, err
		}
	}
	offerings := []*enrollmentModels.CareOffering{}
	if capabilities.CareOfferingsEnabled {
		if offerings, err = validateLoadedAvailabilityRules(s.deps.Offerings.ListActiveByPhase(ctx, phase.ID)); err != nil {
			return nil, err
		}
	}
	data.Offerings = publicCareOfferings(offerings)
	data.EffectiveCapabilities = effectiveFormCapabilities(capabilities, offerings)
	if opts.loadLegal {
		if data.LegalTexts, err = s.bootstrapLegalTexts(ctx, opts); err != nil {
			return nil, err
		}
	}
	return data, nil
}

// bootstrapLateInvite resolves the prefill invite of a form link. An invalid
// token does not block an otherwise public, open phase; it only grants no
// prefill or closed-phase override.
func (s *Intake) bootstrapLateInvite(ctx context.Context, phaseID int64, opts publicBootstrapOptions) (*enrollment.LateInvite, error) {
	token := strings.TrimSpace(opts.lateInviteToken)
	if s.deps.LateInvites == nil || token == "" {
		return nil, nil
	}
	invite, err := s.deps.LateInvites.UsableLateInvite(ctx, s.lateInviteTokenHash(token), phaseID, opts.lateInviteNow, false)
	switch {
	case err == nil:
		return invite, nil
	case errors.Is(err, enrollment.ErrLateInviteNotFound):
		return nil, nil
	default:
		return nil, fmt.Errorf("resolve late invite prefill: %w", err)
	}
}

func (s *Intake) bootstrapLegalTexts(ctx context.Context, opts publicBootstrapOptions) (enrollment.LegalTexts, error) {
	texts, err := opts.legalLoader(ctx)
	if err != nil {
		if opts.classifyStages && !isPublicGateError(err) {
			return enrollment.LegalTexts{}, &enrollment.BootstrapStageError{Stage: enrollment.BootstrapStageLegal, Err: err}
		}
		return enrollment.LegalTexts{}, err
	}
	return texts, nil
}

// LoadPublicFormBootstrap assembles the anonymous public form load.
// Capability- and legal-resolution failures come back as a
// BootstrapStageError. Caller must be inside a tenant transaction.
func (s *Intake) LoadPublicFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*enrollment.PublicFormBootstrapData, error) {
	return s.assemblePublicBootstrap(ctx,
		func(ctx context.Context) (*enrollment.Phase, error) {
			return s.loadPublicPhaseWithLateInvite(ctx, phaseID, now, lateInviteToken)
		},
		publicBootstrapOptions{
			loadSchema: true, loadLegal: true, classifyStages: true,
			lateInviteToken: lateInviteToken, lateInviteNow: now,
			legalLoader: func(ctx context.Context) (enrollment.LegalTexts, error) {
				return s.LegalTextsForPhaseWithLateInvite(ctx, phaseID, lateInviteToken)
			},
		})
}

// LoadEnrolleeFormBootstrap mirrors LoadPublicFormBootstrap for the
// authenticated parents-portal form load, admitting the restricted audiences
// the caller's access covers. Caller must be inside a tenant transaction.
func (s *Intake) LoadEnrolleeFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access enrollment.EnrolleeAudienceAccess) (*enrollment.PublicFormBootstrapData, error) {
	return s.assemblePublicBootstrap(ctx,
		func(ctx context.Context) (*enrollment.Phase, error) {
			return s.loadEditablePhaseWithLateInvite(ctx, phaseID, now, lateInviteToken, access)
		},
		publicBootstrapOptions{
			loadSchema: true, loadLegal: true, classifyStages: true,
			lateInviteToken: lateInviteToken, lateInviteNow: now,
			legalLoader: func(ctx context.Context) (enrollment.LegalTexts, error) {
				return s.legalTextsForEnrolleePhase(ctx, phaseID, lateInviteToken, access)
			},
		})
}

// LoadPublicCareOfferings is the offering-only projection of the public form
// load: phase gate, capabilities and active offerings.
func (s *Intake) LoadPublicCareOfferings(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*enrollment.PublicFormBootstrapData, error) {
	return s.assemblePublicBootstrap(ctx,
		func(ctx context.Context) (*enrollment.Phase, error) {
			return s.loadPublicPhaseWithLateInvite(ctx, phaseID, now, lateInviteToken)
		},
		publicBootstrapOptions{classifyStages: true})
}

// LoadManualEnrollmentBootstrap mirrors LoadPublicFormBootstrap for the staff
// manual enrollment. It does not wrap stage errors.
func (s *Intake) LoadManualEnrollmentBootstrap(ctx context.Context, phaseID int64) (*enrollment.PublicFormBootstrapData, error) {
	return s.assemblePublicBootstrap(ctx,
		func(ctx context.Context) (*enrollment.Phase, error) {
			return s.loadPhaseForEditableRequest(ctx, phaseID)
		},
		publicBootstrapOptions{
			loadSchema: true, loadLegal: true,
			legalLoader: func(ctx context.Context) (enrollment.LegalTexts, error) {
				return s.legalTextsForManualEnrollmentPhase(ctx, phaseID)
			},
		})
}

// validateLoadedAvailabilityRules refuses an offering whose stored
// availability rule no longer validates, so the parent form never offers a
// choice the submission would reject.
func validateLoadedAvailabilityRules(offerings []*enrollmentModels.CareOffering, loadErr error) ([]*enrollmentModels.CareOffering, error) {
	if loadErr != nil {
		return nil, loadErr
	}
	for _, offering := range offerings {
		if offering.AvailabilityRule != nil {
			if err := offering.AvailabilityRule.NormalizeAndValidate(); err != nil {
				return nil, fmt.Errorf("care offering %d has an invalid persisted availability rule: %w", offering.ID, err)
			}
		}
	}
	return offerings, nil
}
