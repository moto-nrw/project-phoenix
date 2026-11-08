package enrollmenthttp

import (
	"context"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The intake as these routes and the parents portal speak it: the owner's
// capabilities with its care offerings decoded for rendering.

// IntakeOwner is Enrollment's intake: submissions, status link and form
// loads.
type IntakeOwner interface {
	capability.IntakeSubmissions
	capability.IntakeStatus
	capability.IntakeForms
	capability.IntakeSignatureProofs
	capability.IntakeFormLabels
}

// SignatureProof is the request behind a status token with its signature,
// flattened for the proof (#4786). SignatureRequired reports whether the
// request's form asks the parent to sign every edit.
type SignatureProof struct {
	Request            *capability.Request
	Children           []*capability.RequestChild
	Guardians          []*capability.RequestGuardian
	Schema             *capability.FormSchema
	OfferingSelections []*capability.RequestChildOfferingRecord
	OfferingNames      map[int64]string
	Signature          *capability.RequestSignature
	SignatureRequired  bool
}

// StatusSignature is the signature metadata shown on the public status page.
// The PDF path loads SignatureProof instead.
type StatusSignature struct {
	Signature         *capability.RequestSignature
	SignatureRequired bool
}

// Intake values the owner's contract carries unchanged.
type (
	SubmitGuardian          = capability.SubmitGuardian
	SubmitOfferingDays      = capability.SubmitOfferingDays
	SubmissionWarning       = capability.SubmissionWarning
	FormCapabilities        = capability.FormCapabilities
	CreateLateInviteInput   = capability.CreateLateInviteInput
	CreateLateInviteResult  = capability.CreateLateInviteResult
	LegalTexts              = capability.LegalTexts
	LegalBlock              = capability.LegalBlock
	EnrolleeAudienceAccess  = capability.EnrolleeAudienceAccess
	BootstrapStage          = capability.BootstrapStage
	BootstrapStageError     = capability.BootstrapStageError
	CareOfferingBookingStat = capability.CareOfferingBookingStat
	// The submission, its result and the parent's patch carry the family's
	// answers as the owner's FormValues (ADR 0170).
	SubmitRequest                  = capability.SubmitRequest
	SubmitChild                    = capability.SubmitChild
	SubmitResult                   = capability.SubmitResult
	EditPatch                      = capability.EditPatch
	ManualApprovedEnrollmentInput  = capability.ManualApprovedEnrollmentInput
	ManualApprovedEnrollmentResult = capability.ManualApprovedEnrollmentResult
)

// Stages of a form load that fail as a server error.
const (
	BootstrapStageCapabilities = capability.BootstrapStageCapabilities
	BootstrapStageLegal        = capability.BootstrapStageLegal
)

// PublicFormBootstrapData is a form load with its offerings in enrollment
// rows.
type PublicFormBootstrapData struct {
	Phase                 *capability.Phase
	Schema                *capability.FormSchema
	Offerings             []*CareOffering
	Capabilities          FormCapabilities
	EffectiveCapabilities FormCapabilities
	LegalTexts            LegalTexts
	LateInvite            *capability.LateInvite
}

// EditDraft is the stored request that reopens the public form, with its
// open offerings decoded.
type EditDraft struct {
	Request              *capability.Request
	Children             []*capability.RequestChild
	Guardians            []*capability.RequestGuardian
	OfferingsByChild     map[int64][]*RequestChildOffering
	Phase                *capability.Phase
	School               *capability.School
	Schema               *capability.FormSchema
	OpenOfferings        []*CareOffering
	LegalTexts           LegalTexts
	EditMode             string
	CollectSchoolClass   bool
	CollectGradeLevel    bool
	CareOfferingsEnabled bool
	GradeLevelMax        int
	CareFacilityTerm     string
}

// RequestService is the intake these routes and the parents portal call.
type RequestService interface {
	Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error)
	CreateLateInvite(ctx context.Context, input CreateLateInviteInput) (*CreateLateInviteResult, error)
	GetByStatusToken(ctx context.Context, token string) (*capability.Request, []*capability.RequestChild, error)
	StatusPage(ctx context.Context, req *capability.Request, children []*capability.RequestChild) (capability.StatusPage, error)
	GetEditDraft(ctx context.Context, token string) (*EditDraft, error)
	ReplaceEditable(ctx context.Context, token string, req SubmitRequest) (*SubmitResult, error)
	GuardiansByStatusToken(ctx context.Context, token string) ([]*capability.RequestGuardian, error)
	Edit(ctx context.Context, token string, patch EditPatch) error
	Withdraw(ctx context.Context, token string, childID int64) error
	ConfirmRenewal(ctx context.Context, token string) (int, error)
	IsEnrollmentEnabled(ctx context.Context) bool
	LegalTexts(ctx context.Context) (LegalTexts, error)
	LegalTextsForPhaseWithLateInvite(ctx context.Context, phaseID int64, lateInviteToken string) (LegalTexts, error)
	LoadPublicFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error)
	LoadEnrolleeFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access EnrolleeAudienceAccess) (*PublicFormBootstrapData, error)
	phasePreviewForms
	LoadManualEnrollmentBootstrap(ctx context.Context, phaseID int64) (*PublicFormBootstrapData, error)
	CreateManualApprovedEnrollment(ctx context.Context, input ManualApprovedEnrollmentInput) (*ManualApprovedEnrollmentResult, error)
	// SignatureProof loads a request with its signature (#4786): behind its
	// status token when token is set, otherwise the request of the school
	// in the caller's tenant transaction.
	SignatureProof(ctx context.Context, requestID int64, token string) (*SignatureProof, error)
	// StatusSignature loads only the signature metadata shown on the public
	// status page. It is authorized by the status token.
	StatusSignature(ctx context.Context, token string) (*StatusSignature, error)
	// FormLabels loads the form version a request is pinned to (#6142):
	// behind its status token when token is set, otherwise the request of
	// the school in the caller's tenant transaction.
	FormLabels(ctx context.Context, requestID int64, token string) (*capability.RequestFormLabels, error)
}

// NewRequestService decodes the owner's intake for these routes and the
// parents portal. A nil owner yields nil.
func NewRequestService(owner IntakeOwner) RequestService {
	if owner == nil {
		return nil
	}
	return intakeService{owner: owner, phasePreviewForms: owner}
}

// The preview carries public owner values directly. Promote that capability
// instead of adding a forwarding HTTP adapter.
type phasePreviewForms interface {
	LoadPhasePreviewCareOfferings(ctx context.Context, phaseID int64) (*capability.PublicFormBootstrapData, error)
}

type intakeService struct {
	owner IntakeOwner
	phasePreviewForms
}

func bootstrapValue(data *capability.PublicFormBootstrapData, err error) (*PublicFormBootstrapData, error) {
	if err != nil {
		return nil, err
	}
	offerings, err := careOfferingValues(data.Offerings)
	if err != nil {
		return nil, err
	}
	return &PublicFormBootstrapData{
		Phase: data.Phase, Schema: data.Schema, Offerings: offerings, Capabilities: data.Capabilities,
		EffectiveCapabilities: data.EffectiveCapabilities, LegalTexts: data.LegalTexts, LateInvite: data.LateInvite,
	}, nil
}

func (s intakeService) Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error) {
	return s.owner.Submit(ctx, req)
}

func (s intakeService) ReplaceEditable(ctx context.Context, token string, req SubmitRequest) (*SubmitResult, error) {
	return s.owner.ReplaceEditable(ctx, token, req)
}

func (s intakeService) CreateLateInvite(ctx context.Context, input CreateLateInviteInput) (*CreateLateInviteResult, error) {
	return s.owner.CreateLateInvite(ctx, input)
}

func (s intakeService) GetByStatusToken(ctx context.Context, token string) (*capability.Request, []*capability.RequestChild, error) {
	status, err := s.owner.StatusByToken(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	return status.Request, status.Children, nil
}

func (s intakeService) StatusPage(ctx context.Context, req *capability.Request, children []*capability.RequestChild) (capability.StatusPage, error) {
	if req == nil {
		return capability.StatusPage{EditMode: capability.EditModeNone}, nil
	}
	return s.owner.StatusPage(ctx, &capability.RequestStatus{Request: req, Children: children})
}

func (s intakeService) GetEditDraft(ctx context.Context, token string) (*EditDraft, error) {
	draft, err := s.owner.EditDraft(ctx, token)
	if err != nil {
		return nil, err
	}
	return editDraftValue(draft)
}

func editDraftValue(draft *capability.EditDraft) (*EditDraft, error) {
	offerings, err := careOfferingValues(draft.OpenOfferings)
	if err != nil {
		return nil, err
	}
	return &EditDraft{
		Request: draft.Request, Children: draft.Children, Guardians: draft.Guardians, OfferingsByChild: draft.OfferingsByChild,
		Phase: draft.Phase, School: draft.School, Schema: draft.Schema, OpenOfferings: offerings,
		LegalTexts: draft.LegalTexts, EditMode: draft.EditMode, CollectSchoolClass: draft.CollectSchoolClass,
		CollectGradeLevel: draft.CollectGradeLevel, CareOfferingsEnabled: draft.CareOfferingsEnabled, GradeLevelMax: draft.GradeLevelMax,
		CareFacilityTerm: draft.CareFacilityTerm,
	}, nil
}

func (s intakeService) GuardiansByStatusToken(ctx context.Context, token string) ([]*capability.RequestGuardian, error) {
	return s.owner.GuardiansByStatusToken(ctx, token)
}

func (s intakeService) Edit(ctx context.Context, token string, patch EditPatch) error {
	return s.owner.Edit(ctx, token, patch)
}

func (s intakeService) Withdraw(ctx context.Context, token string, childID int64) error {
	return s.owner.Withdraw(ctx, token, childID)
}

func (s intakeService) ConfirmRenewal(ctx context.Context, token string) (int, error) {
	return s.owner.ConfirmRenewal(ctx, token)
}

func (s intakeService) IsEnrollmentEnabled(ctx context.Context) bool {
	return s.owner.IsEnrollmentEnabled(ctx)
}

func (s intakeService) LegalTexts(ctx context.Context) (LegalTexts, error) {
	return s.owner.LegalTexts(ctx)
}

func (s intakeService) LegalTextsForPhaseWithLateInvite(ctx context.Context, phaseID int64, lateInviteToken string) (LegalTexts, error) {
	return s.owner.LegalTextsForPhaseWithLateInvite(ctx, phaseID, lateInviteToken)
}

func (s intakeService) LoadPublicFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error) {
	return bootstrapValue(s.owner.LoadPublicFormBootstrap(ctx, phaseID, now, lateInviteToken))
}

func (s intakeService) LoadEnrolleeFormBootstrap(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string, access EnrolleeAudienceAccess) (*PublicFormBootstrapData, error) {
	return bootstrapValue(s.owner.LoadEnrolleeFormBootstrap(ctx, phaseID, now, lateInviteToken, access))
}

func (s intakeService) LoadManualEnrollmentBootstrap(ctx context.Context, phaseID int64) (*PublicFormBootstrapData, error) {
	return bootstrapValue(s.owner.LoadManualEnrollmentBootstrap(ctx, phaseID))
}

func (s intakeService) CreateManualApprovedEnrollment(ctx context.Context, input ManualApprovedEnrollmentInput) (*ManualApprovedEnrollmentResult, error) {
	return s.owner.CreateManualApprovedEnrollment(ctx, input)
}

func (s intakeService) SignatureProof(ctx context.Context, requestID int64, token string) (*SignatureProof, error) {
	if token != "" {
		return signatureProofValue(s.owner.SignatureProofByStatusToken(ctx, token))
	}
	return signatureProofValue(s.owner.SignatureProof(ctx, requestID))
}

func (s intakeService) StatusSignature(ctx context.Context, token string) (*StatusSignature, error) {
	status, err := s.owner.SignatureStatusByStatusToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return &StatusSignature{Signature: status.Signature, SignatureRequired: status.SignatureRequired}, nil
}

func (s intakeService) FormLabels(ctx context.Context, requestID int64, token string) (*capability.RequestFormLabels, error) {
	if token != "" {
		return s.owner.FormLabelsByStatusToken(ctx, token)
	}
	return s.owner.FormLabels(ctx, requestID)
}

// signatureProofValue flattens the owner's proof for these routes.
func signatureProofValue(proof *capability.RequestSignatureProof, err error) (*SignatureProof, error) {
	if err != nil {
		return nil, err
	}
	return &SignatureProof{
		Request: proof.Status.Request, Children: proof.Status.Children, Guardians: proof.Guardians,
		Schema: proof.Schema, OfferingSelections: proof.OfferingSelections, OfferingNames: proof.OfferingNames,
		Signature: proof.Signature, SignatureRequired: proof.SignatureRequired,
	}, nil
}
