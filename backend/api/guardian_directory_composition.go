package api

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// The guardian HTTP adapter's runtime (#2663): the identity, invitation and
// document flows that are not part of the People Directory capability.

// --- runtime for the HTTP adapter ---

// GuardianFailureKind classifies an invitation failure for the HTTP layer.
type GuardianFailureKind string

const (
	GuardianFailureInvalidRequest GuardianFailureKind = "invalid_request"
	GuardianFailureConflict       GuardianFailureKind = "conflict"
	GuardianFailureForbidden      GuardianFailureKind = "forbidden"
	GuardianFailureInternal       GuardianFailureKind = "internal"
)

// GuardianInvitationSummary is the staff-initiated invitation the deprecated
// per-guardian invite endpoint reports; Token is only exposed to the seeder.
type GuardianInvitationSummary struct {
	ID                int64
	GuardianProfileID int64
	ExpiresAt         time.Time
	EmailSent         bool
	Token             string
}

// PendingGuardianInvitation is one open invitation of the tenant.
type PendingGuardianInvitation struct {
	ID                int64
	GuardianProfileID int64
	Token             string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	EmailSentAt       *time.Time
	EmailError        *string
	EmailRetryCount   int
}

// GuardianInviteInput invites a further guardian to a child by e-mail.
type GuardianInviteInput struct {
	StudentID          int64
	Email              string
	FirstName          string
	LastName           string
	RelationshipType   string
	ActorAccountID     int64
	ConfirmRoleUpgrade bool
}

// GuardianInviteResult echoes what the invite resolved to.
type GuardianInviteResult struct {
	Outcome           string
	GuardianProfileID int64
	InvitationID      *int64
	ExistingRole      string
}

// The bulk invitation (#3378) passes Identity & Access' own contract through.
type (
	GuardianBulkInviteRequest = identityaccess.BulkInviteRequest
	GuardianBulkInviteResult  = identityaccess.BulkInviteResult
)

// GuardianPendingApproval is one parent-initiated request awaiting staff.
type GuardianPendingApproval struct {
	InvitationID      int64
	GuardianProfileID int64
	GuardianName      string
	GuardianEmail     string
	StudentID         int64
	StudentName       string
	RequestedByEmail  string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	RoleUpgrade       bool
}

// GuardianExportFile is the rendered Bankverbindungen document.
type GuardianExportFile struct {
	ContentType string
	Filename    string
	Data        []byte
}

// GuardianDirectoryRuntime is the legacy-service side of the guardian HTTP
// adapter: the identity, invitation and document flows that are not part
// of the People Directory capability. Every closure keeps the semantics of
// the handler code it replaced in api/guardians.
type GuardianDirectoryRuntime struct {
	// IsVerifiedStaff reports whether the caller has a staff record; the
	// legacy handlers treated any lookup failure as "not staff".
	IsVerifiedStaff func(context.Context) bool
	MarkRollback    func(context.Context)

	SendInvitation         func(ctx context.Context, guardianID, actorAccountID int64) (GuardianInvitationSummary, error)
	ListPendingInvitations func(context.Context) ([]PendingGuardianInvitation, error)

	InviteGuardianToStudent func(context.Context, GuardianInviteInput) (GuardianInviteResult, error)
	// BulkInviteGuardians invites the guardians of many children (#3378); a
	// failed run rolls the request's transaction back.
	BulkInviteGuardians        func(context.Context, GuardianBulkInviteRequest) (*GuardianBulkInviteResult, error)
	ListPendingApprovals       func(context.Context) ([]GuardianPendingApproval, error)
	PendingInvitationStudentID func(context.Context, int64) (int64, error)
	ApproveInvitation          func(ctx context.Context, invitationID, actorAccountID int64) error
	RejectInvitation           func(ctx context.Context, invitationID, actorAccountID int64) error

	// RenderPaymentExport renders the Bankverbindungen list; format is one
	// of pdf, docx or xlsx, validated by the adapter beforehand.
	RenderPaymentExport func(rows []peopledirectory.GuardianPaymentRow, format string) (GuardianExportFile, error)
}

// GuardianDirectoryInvitations is the Identity & Access half the guardian
// routes drive: the invitation lifecycle and the relative access flows.
type GuardianDirectoryInvitations interface {
	identityaccess.GuardianInvitations
	identityaccess.GuardianRelativeAccess
}

// NewGuardianDirectoryRuntime composes the closures over the legacy service
// graph and the Identity & Access the root composed (#4006): invitations
// serves the invitation writes, verifiedStaff tells a verified staff caller.
// runtime is the People Directory's tenant runtime the invitation writes run
// in.
func NewGuardianDirectoryRuntime(guardians peoplePorts.GuardianService, renderer *lists.RendererService, invitations GuardianDirectoryInvitations, verifiedStaff func(context.Context) (bool, error), runtime peopleCompose.LegacyRepositoryRuntime) GuardianDirectoryRuntime {
	sendInvitation, inviteGuardian, bulkInvite := guardianInvitationFlows(invitations, runtime)
	listApprovals, approve, reject := guardianApprovalFlows(invitations, runtime)
	return GuardianDirectoryRuntime{
		IsVerifiedStaff: func(ctx context.Context) bool {
			staff, err := verifiedStaff(ctx)
			return err == nil && staff
		},
		MarkRollback:               runtime.MarkRollback,
		SendInvitation:             sendInvitation,
		ListPendingInvitations:     pendingGuardianInvitations(guardians),
		InviteGuardianToStudent:    inviteGuardian,
		BulkInviteGuardians:        bulkInvite,
		ListPendingApprovals:       listApprovals,
		PendingInvitationStudentID: invitations.PendingInvitationStudentID,
		ApproveInvitation:          approve,
		RejectInvitation:           reject,
		RenderPaymentExport:        guardianPaymentExport(renderer),
	}
}

// guardianInvitationFlows are the staff-initiated invitations: one guardian,
// a further guardian of a child, and the bulk run (#3378). A failed invite
// rolls the request's transaction back.
func guardianInvitationFlows(invitations GuardianDirectoryInvitations, runtime peopleCompose.LegacyRepositoryRuntime) (
	func(ctx context.Context, guardianID, actorAccountID int64) (GuardianInvitationSummary, error),
	func(context.Context, GuardianInviteInput) (GuardianInviteResult, error),
	func(context.Context, GuardianBulkInviteRequest) (*GuardianBulkInviteResult, error),
) {
	send := func(ctx context.Context, guardianID, actorAccountID int64) (GuardianInvitationSummary, error) {
		var summary GuardianInvitationSummary
		err := runtime.InTenantTx(ctx, func(txCtx context.Context) error {
			invitation, err := invitations.CreateGuardianInvitation(txCtx, identityaccess.GuardianInvitationRequest{
				GuardianProfileID: guardianID, CreatedBy: actorAccountID,
			})
			if err != nil {
				return err
			}
			summary = GuardianInvitationSummary{
				ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, ExpiresAt: invitation.ExpiresAt,
				EmailSent: invitation.EmailSentAt != nil, Token: invitation.Token,
			}
			return nil
		})
		return summary, err
	}
	invite := func(ctx context.Context, input GuardianInviteInput) (GuardianInviteResult, error) {
		result, err := invitations.InviteToStudent(ctx, identityaccess.InviteToStudentRequest{
			StudentID: input.StudentID, Email: input.Email, FirstName: input.FirstName, LastName: input.LastName,
			RelationshipType: input.RelationshipType, CreatedBy: input.ActorAccountID,
			RequireApproval: identityaccess.StaffInviteRequiresApproval, ConfirmRoleUpgrade: input.ConfirmRoleUpgrade,
		})
		if err != nil {
			runtime.MarkRollback(ctx)
			return GuardianInviteResult{}, err
		}
		return GuardianInviteResult{
			Outcome: string(result.Outcome), GuardianProfileID: result.GuardianProfileID,
			InvitationID: result.InvitationID, ExistingRole: result.ExistingRole,
		}, nil
	}
	bulk := func(ctx context.Context, req GuardianBulkInviteRequest) (*GuardianBulkInviteResult, error) {
		result, err := invitations.BulkInviteToStudents(ctx, req)
		if err != nil {
			runtime.MarkRollback(ctx)
		}
		return result, err
	}
	return send, invite, bulk
}

// pendingGuardianInvitations lists the tenant's open invitations.
func pendingGuardianInvitations(guardians peoplePorts.GuardianService) func(context.Context) ([]PendingGuardianInvitation, error) {
	return func(ctx context.Context) ([]PendingGuardianInvitation, error) {
		invitations, err := guardians.GuardianInvitationReads().ListRedeemable(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]PendingGuardianInvitation, 0, len(invitations))
		for _, invitation := range invitations {
			result = append(result, PendingGuardianInvitation{
				ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, Token: invitation.Token, CreatedAt: invitation.CreatedAt,
				ExpiresAt: invitation.ExpiresAt, EmailSentAt: invitation.EmailSentAt, EmailError: invitation.EmailError,
			})
		}
		return result, nil
	}
}

// guardianApprovalFlows list, approve and reject the parent-initiated
// requests; a failed decision rolls the request's transaction back.
func guardianApprovalFlows(invitations GuardianDirectoryInvitations, runtime peopleCompose.LegacyRepositoryRuntime) (
	func(context.Context) ([]GuardianPendingApproval, error),
	func(ctx context.Context, invitationID, actorAccountID int64) error,
	func(ctx context.Context, invitationID, actorAccountID int64) error,
) {
	list := func(ctx context.Context) ([]GuardianPendingApproval, error) {
		views, err := invitations.ListPendingApprovalsDetailed(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]GuardianPendingApproval, 0, len(views))
		for _, view := range views {
			result = append(result, GuardianPendingApproval{
				InvitationID: view.InvitationID, GuardianProfileID: view.GuardianProfileID,
				GuardianName: view.GuardianName, GuardianEmail: view.GuardianEmail,
				StudentID: view.StudentID, StudentName: view.StudentName, RequestedByEmail: view.RequestedByEmail,
				CreatedAt: view.CreatedAt, ExpiresAt: view.ExpiresAt, RoleUpgrade: view.RoleUpgrade,
			})
		}
		return result, nil
	}
	approve := func(ctx context.Context, invitationID, actorAccountID int64) error {
		if err := invitations.ApproveInvitation(ctx, invitationID, actorAccountID); err != nil {
			runtime.MarkRollback(ctx)
			return err
		}
		return nil
	}
	reject := func(ctx context.Context, invitationID, actorAccountID int64) error {
		if err := invitations.RejectInvitation(ctx, invitationID, actorAccountID); err != nil {
			runtime.MarkRollback(ctx)
			return err
		}
		return nil
	}
	return list, approve, reject
}

// guardianPaymentExport renders the Bankverbindungen list; format is one of
// pdf, docx or xlsx, validated by the adapter beforehand.
func guardianPaymentExport(renderer *lists.RendererService) func(rows []peopledirectory.GuardianPaymentRow, format string) (GuardianExportFile, error) {
	return func(rows []peopledirectory.GuardianPaymentRow, format string) (GuardianExportFile, error) {
		if renderer == nil {
			return GuardianExportFile{}, errors.New("list export service is not configured")
		}
		list := peopledirectory.GuardianPaymentListOf(rows)
		doc := lists.Document{
			Title:       list.Title,
			Subtitle:    list.Subtitle,
			GeneratedAt: time.Now(),
			Footer:      list.Footer,
			Columns: []lists.Column{
				{ID: lists.ColumnStudentName, Label: list.Headings.Student},
				{ID: lists.ColumnStudentClass, Label: list.Headings.Class},
				{ID: lists.ColumnContactName, Label: list.Headings.AccountHolder},
				{ID: lists.ColumnIBAN, Label: list.Headings.IBAN},
			},
			Rows: paymentExportRows(list.Lines),
		}
		file, err := renderer.Render(doc, lists.Format(format), doc.Title)
		if err != nil {
			return GuardianExportFile{}, err
		}
		return GuardianExportFile{ContentType: file.ContentType, Filename: file.Filename, Data: file.Data}, nil
	}
}

// ClassifyGuardianInvitationFailure maps the invitation sentinels. Unknown
// failures come from persistence or delivery and must not be reported as bad
// client input.
func ClassifyGuardianInvitationFailure(err error) GuardianFailureKind {
	var validation *identityaccess.GuardianInvitationValidationError
	switch {
	case errors.Is(err, peopledirectory.ErrGuardianAccountConflict):
		return GuardianFailureConflict
	case errors.As(err, &validation),
		errors.Is(err, identityaccess.ErrGuardianInvitationNotFound),
		errors.Is(err, identityaccess.ErrGuardianInvitationExpired):
		return GuardianFailureInvalidRequest
	case errors.Is(err, identityaccess.ErrInviteSocialWorkerManaged):
		return GuardianFailureForbidden
	default:
		return GuardianFailureInternal
	}
}

// paymentExportRows places the owner's lines in the export's columns.
func paymentExportRows(lines []peopledirectory.GuardianPaymentLine) []lists.Row {
	out := make([]lists.Row, 0, len(lines))
	for _, line := range lines {
		out = append(out, lists.Row{
			Values: map[lists.ColumnID]string{
				lists.ColumnStudentName:  line.Student,
				lists.ColumnStudentClass: line.Class,
				lists.ColumnContactName:  line.AccountHolder,
				lists.ColumnIBAN:         line.IBAN,
			},
		})
	}
	return out
}
