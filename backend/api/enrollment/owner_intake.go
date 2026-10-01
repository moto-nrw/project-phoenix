package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The intake as these routes and the parents portal speak it: the owner's
// capabilities with the submission's answers decoded.

// IntakeOwner is Enrollment's intake: submissions, status link and form
// loads.
type IntakeOwner interface {
	capability.IntakeSubmissions
	capability.IntakeStatus
	capability.IntakeForms
}

// Intake values the owner's contract carries unchanged.
type (
	SubmitGuardian                 = capability.SubmitGuardian
	SubmitOfferingDays             = capability.SubmitOfferingDays
	SubmissionWarning              = capability.SubmissionWarning
	FormCapabilities               = capability.FormCapabilities
	CreateLateInviteInput          = capability.CreateLateInviteInput
	CreateLateInviteResult         = capability.CreateLateInviteResult
	LegalTexts                     = capability.LegalTexts
	LegalBlock                     = capability.LegalBlock
	EnrolleeAudienceAccess         = capability.EnrolleeAudienceAccess
	BootstrapStage                 = capability.BootstrapStage
	BootstrapStageError            = capability.BootstrapStageError
	CareOfferingBookingStat        = capability.CareOfferingBookingStat
	ManualApprovedEnrollmentOutput = capability.ManualApprovedEnrollmentResult
)

// Stages of a form load that fail as a server error.
const (
	BootstrapStageCapabilities = capability.BootstrapStageCapabilities
	BootstrapStageLegal        = capability.BootstrapStageLegal
)

// SubmitRequest is one submission with its answers decoded.
type SubmitRequest struct {
	TenantID                 int64
	PhaseID                  int64
	RemoteIP                 string
	LateInviteToken          string
	SkipRateLimit            bool
	AllowClosedPhase         bool
	SuppressSubmissionEmails bool
	ExternalConsentConfirmed bool
	SubmissionSource         string
	SourceMetadata           map[string]any
	GuardianFirstName        string
	GuardianLastName         string
	GuardianEmail            string
	GuardianPhone            *string
	ConsentFlags             map[string]any
	CustomData               map[string]any
	GuardianAccountID        *int64
	GuardianSubmitEligible   bool
	Children                 []SubmitChild
	AdditionalGuardians      []SubmitGuardian
}

// SubmitChild is one child of a submission with its answers decoded.
type SubmitChild struct {
	ID                       int64
	FirstName                string
	LastName                 string
	DateOfBirth              timezone.Date
	TargetGradeLevel         *int16
	TargetSchoolClass        *string
	CustomData               map[string]any
	OfferingIDs              []int64
	OfferingDays             []SubmitOfferingDays
	ExcludedAutoAddTargetIDs map[int64]bool
}

// SubmitResult is what a submission or an edit stored, decoded.
type SubmitResult struct {
	Request   *enrollmentModels.Request
	Children  []*RequestChild
	StatusURL string
	Warnings  []SubmissionWarning
}

// EditPatch carries the guardian fields a parent may patch before the first
// decision; a non-nil map replaces the stored one.
type EditPatch struct {
	GuardianFirstName *string
	GuardianLastName  *string
	GuardianPhone     *string
	ConsentFlags      map[string]any
	CustomData        map[string]any
}

// ManualApprovedEnrollmentInput is a staff-created submission approved in one
// step.
type ManualApprovedEnrollmentInput struct {
	Request          SubmitRequest
	Reason           string
	SendNotification bool
	ActorID          int64
}

// ManualApprovedEnrollmentResult is the created request, the approved child,
// its status link and the guardian invitation the approval owes, decoded.
type ManualApprovedEnrollmentResult struct {
	Request       *enrollmentModels.Request
	Child         *RequestChild
	StatusURL     string
	PendingInvite *PendingGuardianInvite
}

// PublicFormBootstrapData is a form load with its offerings in enrollment
// rows.
type PublicFormBootstrapData struct {
	Phase                 *capability.Phase
	Schema                *capability.FormSchema
	Offerings             []*enrollmentModels.CareOffering
	Capabilities          FormCapabilities
	EffectiveCapabilities FormCapabilities
	LegalTexts            LegalTexts
	LateInvite            *capability.LateInvite
}

// EditDraft is the stored request that reopens the public form, decoded.
type EditDraft struct {
	Request              *enrollmentModels.Request
	Children             []*RequestChild
	Guardians            []*capability.RequestGuardian
	OfferingsByChild     map[int64][]*RequestChildOffering
	Phase                *capability.Phase
	School               *capability.School
	Schema               *capability.FormSchema
	OpenOfferings        []*enrollmentModels.CareOffering
	LegalTexts           LegalTexts
	EditMode             string
	CollectSchoolClass   bool
	CollectGradeLevel    bool
	CareOfferingsEnabled bool
	GradeLevelMax        int
}

// RequestService is the intake these routes and the parents portal call.
type RequestService interface {
	Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error)
	CreateLateInvite(ctx context.Context, input CreateLateInviteInput) (*CreateLateInviteResult, error)
	GetByStatusToken(ctx context.Context, token string) (*enrollmentModels.Request, []*RequestChild, error)
	EditModeForStatus(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild) (string, error)
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
	LoadPublicCareOfferings(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error)
	LoadManualEnrollmentBootstrap(ctx context.Context, phaseID int64) (*PublicFormBootstrapData, error)
	CreateManualApprovedEnrollment(ctx context.Context, input ManualApprovedEnrollmentInput) (*ManualApprovedEnrollmentResult, error)
	PublicActiveSchema(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*capability.FormSchema, error)
}

// NewRequestService decodes the owner's intake for these routes and the
// parents portal. A nil owner yields nil.
func NewRequestService(owner IntakeOwner) RequestService {
	if owner == nil {
		return nil
	}
	return intakeService{owner: owner}
}

type intakeService struct{ owner IntakeOwner }

func encodeSubmitRequest(in SubmitRequest) (capability.SubmitRequest, error) {
	out := capability.SubmitRequest{
		TenantID: in.TenantID, PhaseID: in.PhaseID, RemoteIP: in.RemoteIP, LateInviteToken: in.LateInviteToken,
		SkipRateLimit: in.SkipRateLimit, AllowClosedPhase: in.AllowClosedPhase,
		SuppressSubmissionEmails: in.SuppressSubmissionEmails, ExternalConsentConfirmed: in.ExternalConsentConfirmed,
		SubmissionSource: in.SubmissionSource, GuardianFirstName: in.GuardianFirstName,
		GuardianLastName: in.GuardianLastName, GuardianEmail: in.GuardianEmail, GuardianPhone: in.GuardianPhone,
		GuardianAccountID: in.GuardianAccountID, GuardianSubmitEligible: in.GuardianSubmitEligible,
		AdditionalGuardians: in.AdditionalGuardians,
	}
	var err error
	if out.SourceMetadata, err = encodeJSONObject(in.SourceMetadata); err != nil {
		return out, err
	}
	if out.ConsentFlags, err = encodeJSONObject(in.ConsentFlags); err != nil {
		return out, err
	}
	if out.CustomData, err = encodeJSONObject(in.CustomData); err != nil {
		return out, err
	}
	if in.Children != nil {
		out.Children = make([]capability.SubmitChild, 0, len(in.Children))
	}
	for _, child := range in.Children {
		customData, err := encodeJSONObject(child.CustomData)
		if err != nil {
			return out, err
		}
		out.Children = append(out.Children, capability.SubmitChild{
			ID: child.ID, FirstName: child.FirstName, LastName: child.LastName, DateOfBirth: child.DateOfBirth,
			TargetGradeLevel: child.TargetGradeLevel, TargetSchoolClass: child.TargetSchoolClass, CustomData: customData,
			OfferingIDs: child.OfferingIDs, OfferingDays: child.OfferingDays, ExcludedAutoAddTargetIDs: child.ExcludedAutoAddTargetIDs,
		})
	}
	return out, nil
}

func submitResultValue(result *capability.SubmitResult, err error) (*SubmitResult, error) {
	if err != nil {
		return nil, err
	}
	request, err := requestValue(result.Request)
	if err != nil {
		return nil, err
	}
	children, err := childValues(result.Children)
	if err != nil {
		return nil, err
	}
	return &SubmitResult{Request: request, Children: children, StatusURL: result.StatusURL, Warnings: result.Warnings}, nil
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
	encoded, err := encodeSubmitRequest(req)
	if err != nil {
		return nil, err
	}
	return submitResultValue(s.owner.Submit(ctx, encoded))
}

func (s intakeService) ReplaceEditable(ctx context.Context, token string, req SubmitRequest) (*SubmitResult, error) {
	encoded, err := encodeSubmitRequest(req)
	if err != nil {
		return nil, err
	}
	return submitResultValue(s.owner.ReplaceEditable(ctx, token, encoded))
}

func (s intakeService) CreateLateInvite(ctx context.Context, input CreateLateInviteInput) (*CreateLateInviteResult, error) {
	return s.owner.CreateLateInvite(ctx, input)
}

func (s intakeService) GetByStatusToken(ctx context.Context, token string) (*enrollmentModels.Request, []*RequestChild, error) {
	status, err := s.owner.StatusByToken(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	request, err := requestValue(status.Request)
	if err != nil {
		return nil, nil, err
	}
	children, err := childValues(status.Children)
	if err != nil {
		return nil, nil, err
	}
	return request, children, nil
}

func (s intakeService) EditModeForStatus(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild) (string, error) {
	if req == nil {
		return capability.EditModeNone, nil
	}
	status, err := requestStatusInput(req, children)
	if err != nil {
		return capability.EditModeNone, err
	}
	return s.owner.EditModeForStatus(ctx, status)
}

func (s intakeService) GetEditDraft(ctx context.Context, token string) (*EditDraft, error) {
	draft, err := s.owner.EditDraft(ctx, token)
	if err != nil {
		return nil, err
	}
	return editDraftValue(draft)
}

func editDraftValue(draft *capability.EditDraft) (*EditDraft, error) {
	request, err := requestValue(draft.Request)
	if err != nil {
		return nil, err
	}
	children, err := childValues(draft.Children)
	if err != nil {
		return nil, err
	}
	offerings, err := careOfferingValues(draft.OpenOfferings)
	if err != nil {
		return nil, err
	}
	return &EditDraft{
		Request: request, Children: children, Guardians: draft.Guardians, OfferingsByChild: draft.OfferingsByChild,
		Phase: draft.Phase, School: draft.School, Schema: draft.Schema, OpenOfferings: offerings,
		LegalTexts: draft.LegalTexts, EditMode: draft.EditMode, CollectSchoolClass: draft.CollectSchoolClass,
		CollectGradeLevel: draft.CollectGradeLevel, CareOfferingsEnabled: draft.CareOfferingsEnabled, GradeLevelMax: draft.GradeLevelMax,
	}, nil
}

func (s intakeService) GuardiansByStatusToken(ctx context.Context, token string) ([]*capability.RequestGuardian, error) {
	return s.owner.GuardiansByStatusToken(ctx, token)
}

func (s intakeService) Edit(ctx context.Context, token string, patch EditPatch) error {
	consentFlags, err := encodeJSONObject(patch.ConsentFlags)
	if err != nil {
		return err
	}
	customData, err := encodeJSONObject(patch.CustomData)
	if err != nil {
		return err
	}
	return s.owner.Edit(ctx, token, capability.EditPatch{
		GuardianFirstName: patch.GuardianFirstName, GuardianLastName: patch.GuardianLastName,
		GuardianPhone: patch.GuardianPhone, ConsentFlags: consentFlags, CustomData: customData,
	})
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

func (s intakeService) LoadPublicCareOfferings(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*PublicFormBootstrapData, error) {
	return bootstrapValue(s.owner.LoadPublicCareOfferings(ctx, phaseID, now, lateInviteToken))
}

func (s intakeService) LoadManualEnrollmentBootstrap(ctx context.Context, phaseID int64) (*PublicFormBootstrapData, error) {
	return bootstrapValue(s.owner.LoadManualEnrollmentBootstrap(ctx, phaseID))
}

func (s intakeService) CreateManualApprovedEnrollment(ctx context.Context, input ManualApprovedEnrollmentInput) (*ManualApprovedEnrollmentResult, error) {
	request, err := encodeSubmitRequest(input.Request)
	if err != nil {
		return nil, err
	}
	result, err := s.owner.CreateManualApprovedEnrollment(ctx, capability.ManualApprovedEnrollmentInput{
		Request: request, Reason: input.Reason, SendNotification: input.SendNotification, ActorID: input.ActorID,
	})
	if err != nil {
		return nil, err
	}
	decodedRequest, err := requestValue(result.Request)
	if err != nil {
		return nil, err
	}
	child, err := childValue(result.Child)
	if err != nil {
		return nil, err
	}
	return &ManualApprovedEnrollmentResult{Request: decodedRequest, Child: child, StatusURL: result.StatusURL, PendingInvite: result.PendingInvite}, nil
}

func (s intakeService) PublicActiveSchema(ctx context.Context, phaseID int64, now time.Time, lateInviteToken string) (*capability.FormSchema, error) {
	return s.owner.PublicActiveSchema(ctx, phaseID, now, lateInviteToken)
}
