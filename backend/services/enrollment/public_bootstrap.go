package enrollment

import (
	"context"
	"errors"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// PublicFormBootstrapData bundles the data the public and manual
// enrollment form-load endpoints assemble inside one tenant transaction:
// the gated phase, its optionally pinned schema, the active care
// offerings, the resolved raw form capabilities, and the legal-text
// contract. EffectiveFormCapabilities and DTO shaping stay in the API
// layer. Schema and LegalTexts stay zero for the offering-only projection.
type PublicFormBootstrapData struct {
	Phase        *enrollmentModels.Phase
	Schema       *enrollmentModels.FormSchema
	Offerings    []*enrollmentModels.CareOffering
	Capabilities FormCapabilities
	LegalTexts   LegalTexts
}

// BootstrapStage identifies which resolution step of a public bootstrap
// failed so the caller can render a 500 for a real settings/DB failure
// instead of the public 404 gate mapping.
type BootstrapStage string

const (
	BootstrapStageCapabilities BootstrapStage = "capabilities"
	BootstrapStageLegal        BootstrapStage = "legal"
)

// BootstrapStageError marks a public-bootstrap failure that must surface
// as a server error (500) rather than the public not-found gate mapping.
type BootstrapStageError struct {
	Stage BootstrapStage
	Err   error
}

func (e *BootstrapStageError) Error() string { return e.Err.Error() }
func (e *BootstrapStageError) Unwrap() error { return e.Err }

type publicBootstrapOptions struct {
	loadSchema     bool
	loadLegal      bool
	classifyStages bool
	legalLoader    func(ctx context.Context) (LegalTexts, error)
}

// isPublicGateError reports whether err is one of the public
// enrollment-gate sentinels, which map to a 404 rather than a 500.
func isPublicGateError(err error) bool {
	return errors.Is(err, ErrInvalidSubmission) ||
		errors.Is(err, ErrEnrollmentDisabled) ||
		errors.Is(err, ErrEnrollmentWindowClosed) ||
		errors.Is(err, ErrLateInviteInvalid)
}

func (s *requestService) assemblePublicBootstrap(
	ctx context.Context,
	phaseLoader func(ctx context.Context) (*enrollmentModels.Phase, error),
	opts publicBootstrapOptions,
) (*PublicFormBootstrapData, error) {
	phase, err := phaseLoader(ctx)
	if err != nil {
		return nil, err
	}
	capabilities, err := s.FormCapabilities(ctx)
	if err != nil {
		if opts.classifyStages {
			return nil, &BootstrapStageError{Stage: BootstrapStageCapabilities, Err: err}
		}
		return nil, err
	}
	data := &PublicFormBootstrapData{
		Phase:        phase,
		Capabilities: capabilities,
		Offerings:    []*enrollmentModels.CareOffering{},
	}
	if opts.loadSchema {
		schema, schemaErr := s.resolveSubmissionSchema(ctx, phase)
		if schemaErr != nil {
			return nil, schemaErr
		}
		data.Schema = schema
	}
	if capabilities.CareOfferingsEnabled {
		offerings, listErr := validateLoadedAvailabilityRules(s.CareOfferingRepo.ListActiveByPhase(ctx, phase.ID))
		if listErr != nil {
			return nil, listErr
		}
		data.Offerings = offerings
	}
	if opts.loadLegal {
		texts, legalErr := opts.legalLoader(ctx)
		if legalErr != nil {
			if opts.classifyStages && !isPublicGateError(legalErr) {
				return nil, &BootstrapStageError{Stage: BootstrapStageLegal, Err: legalErr}
			}
			return nil, legalErr
		}
		data.LegalTexts = texts
	}
	return data, nil
}

func (s *requestService) LoadPublicFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error) {
	return s.assemblePublicBootstrap(ctx,
		func(ctx context.Context) (*enrollmentModels.Phase, error) {
			return s.LoadPublicPhaseWithLateInvite(ctx, phaseID, now, lateInviteToken)
		},
		publicBootstrapOptions{
			loadSchema:     true,
			loadLegal:      true,
			classifyStages: true,
			legalLoader: func(ctx context.Context) (LegalTexts, error) {
				return s.LegalTextsForPhaseWithLateInvite(ctx, phaseID, lateInviteToken)
			},
		})
}

// LoadEnrolleeFormBootstrap mirrors LoadPublicFormBootstrap for the
// authenticated parents-portal form load. It uses the enrollee phase gate
// (which allows audience-restricted linked_parents phases) and the matching
// legal loader; everything else — schema, offerings, capabilities, stage
// error classification — is identical to the public path. Caller must be
// inside a tenant-tx.
func (s *requestService) LoadEnrolleeFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error) {
	return s.assemblePublicBootstrap(ctx,
		func(ctx context.Context) (*enrollmentModels.Phase, error) {
			return s.LoadEnrolleePhaseWithLateInvite(ctx, phaseID, now, lateInviteToken)
		},
		publicBootstrapOptions{
			loadSchema:     true,
			loadLegal:      true,
			classifyStages: true,
			legalLoader: func(ctx context.Context) (LegalTexts, error) {
				return s.LegalTextsForEnrolleePhaseWithLateInvite(ctx, phaseID, lateInviteToken)
			},
		})
}

func (s *requestService) LoadPublicCareOfferings(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error) {
	return s.assemblePublicBootstrap(ctx,
		func(ctx context.Context) (*enrollmentModels.Phase, error) {
			return s.LoadPublicPhaseWithLateInvite(ctx, phaseID, now, lateInviteToken)
		},
		publicBootstrapOptions{classifyStages: true})
}

func (s *requestService) LoadManualEnrollmentBootstrap(ctx context.Context, phaseID int64) (*PublicFormBootstrapData, error) {
	return s.assemblePublicBootstrap(ctx,
		func(ctx context.Context) (*enrollmentModels.Phase, error) {
			return s.LoadManualEnrollmentPhase(ctx, phaseID)
		},
		publicBootstrapOptions{
			loadSchema: true,
			loadLegal:  true,
			legalLoader: func(ctx context.Context) (LegalTexts, error) {
				return s.LegalTextsForManualEnrollmentPhase(ctx, phaseID)
			},
		})
}
