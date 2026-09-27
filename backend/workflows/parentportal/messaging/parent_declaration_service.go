package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	declarations "github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
)

// Erklärungen (#3430): the guardian side. A guardian with
// parent_portal.declarations.submit on the relationship to exactly this child
// declares for the version shown to them; every declaration is appended as
// proof and never changed.

var (
	// ErrDeclarationNotPermitted: the child is the guardian's, but the
	// relationship does not allow declaring (e.g. pickup only). 403.
	ErrDeclarationNotPermitted = errors.New("parent: guardian may not declare for this child")
	// ErrDeclarationVersionChanged: the wording changed since the guardian
	// loaded it. 409; the portal reloads and shows the new version.
	ErrDeclarationVersionChanged = errors.New("parent: the declaration changed since it was loaded")
	// ErrDeclarationClosed: the deadline has passed. 409.
	ErrDeclarationClosed = errors.New("parent: the deadline of this declaration has passed")
	// ErrDeclarationActionNotAllowed: the action does not fit (e.g. revoking
	// without consent, acknowledging a consent). 409.
	ErrDeclarationActionNotAllowed = errors.New("parent: this action is not possible for the declaration")
	// ErrDeclarationPasswordRequired / Incorrect: the school asks for the
	// password before a declaration. 403, never 401: the session is fine.
	ErrDeclarationPasswordRequired  = errors.New("parent: the declaration requires the account password")
	ErrDeclarationPasswordIncorrect = errors.New("parent: the account password is not correct")
)

// DeclarationInput is one declaration by the signed-in guardian.
type DeclarationInput struct {
	StudentID int64
	VersionID int64
	Action    string
	Password  string
}

// PasswordConfirmer checks the signed-in account's password. The handler
// binds it to Identity & Access; nil means no confirmation is possible.
type PasswordConfirmer func(ctx context.Context, password string) error

// declarationTarget is the resolved announcement a declaration is made on.
type declarationTarget struct {
	tenantID         int64
	rules            declarations.DeclarationRules
	requiresPassword bool
}

// SubmitDeclaration records one declaration. The returned bool is false when
// the same action was already the guardian's latest one for this child and
// version: a double click or a retried request yields the same proof, not a
// second row.
func (s *Service) SubmitDeclaration(ctx context.Context, accountID, announcementID int64, in DeclarationInput, confirm PasswordConfirmer) (*usersModels.DeclarationSubmission, bool, error) {
	if accountID <= 0 || announcementID <= 0 || in.StudentID <= 0 || in.VersionID <= 0 {
		return nil, false, fmt.Errorf("parent: account_id, announcement_id, student_id and version_id must be positive")
	}
	target, err := s.resolveDeclarationTarget(ctx, accountID, announcementID, in.StudentID)
	if err != nil {
		return nil, false, err
	}
	if err := s.authorizeDeclarationChild(ctx, accountID, in.StudentID, target.tenantID); err != nil {
		return nil, false, err
	}
	if !knownDeclarationAction(in.Action) {
		return nil, false, ErrDeclarationActionNotAllowed
	}
	passwordConfirmed, err := confirmDeclarationPassword(ctx, target.requiresPassword, in.Password, confirm)
	if err != nil {
		return nil, false, err
	}
	var result *usersModels.DeclarationSubmission
	created := false
	err = tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		result, created, err = s.writeDeclaration(adminCtx, accountID, announcementID, target, in, passwordConfirmed)
		return err
	})
	if err != nil {
		return nil, false, err
	}
	if created {
		s.Logger.Info("parent declaration recorded",
			slog.Int64("account_id", accountID),
			slog.Int64("announcement_id", announcementID),
			slog.Int64("student_id", in.StudentID),
			slog.String("action", in.Action),
			slog.Int64("submission_id", result.ID),
		)
	}
	return result, created, nil
}

// resolveDeclarationTarget collapses every "may not see it" into 404 before
// any other answer, so a parent token cannot probe announcement or student
// ids. A guardian in the audience whose child is not reached gets the more
// useful child error.
func (s *Service) resolveDeclarationTarget(ctx context.Context, accountID, announcementID, studentID int64) (*declarationTarget, error) {
	var target *declarationTarget
	if err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		a, err := s.AnnouncementRepo.FindByID(adminCtx, announcementID)
		if err != nil {
			return fmt.Errorf("parent: load declaration: %w", err)
		}
		if a == nil || !announcementIsLive(a) || !a.IsDeclaration() {
			return ErrAnnouncementNotFound
		}
		children, err := s.AnnouncementRepo.DeclarationChildrenForAccount(adminCtx, accountID, []int64{announcementID})
		if err != nil {
			return fmt.Errorf("parent: declaration children: %w", err)
		}
		if len(children) == 0 {
			return ErrAnnouncementNotFound
		}
		if !containsDeclarationChild(children, studentID) {
			return ErrChildNotAnswerable
		}
		target = &declarationTarget{
			tenantID: a.GetTenantID(), rules: feedDeclarationRules(a.Declaration, a.ResponseDeadline),
			requiresPassword: a.Declaration.RequiresPassword,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if !s.announcementVisibleForTenant(ctx, target.tenantID, announcementID, nil) {
		return nil, ErrAnnouncementNotFound
	}
	return target, nil
}

// authorizeDeclarationChild applies the project's guardian gate with the
// action-specific permission. Seeing the child is not enough to declare.
func (s *Service) authorizeDeclarationChild(ctx context.Context, accountID, studentID, tenantID int64) error {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionDeclarationSubmit)
	switch {
	case errors.Is(err, care.ErrGuardianPermissionDenied):
		return ErrDeclarationNotPermitted
	case errors.Is(err, care.ErrChildNotLinked):
		return ErrChildNotAnswerable
	case err != nil:
		return err
	}
	if child.TenantID != tenantID {
		return ErrChildNotAnswerable
	}
	return nil
}

func confirmDeclarationPassword(ctx context.Context, required bool, password string, confirm PasswordConfirmer) (bool, error) {
	if !required {
		return false, nil
	}
	if strings.TrimSpace(password) == "" {
		return false, ErrDeclarationPasswordRequired
	}
	if confirm == nil {
		return false, errors.New("parent: password confirmation is not wired")
	}
	if err := confirm(ctx, password); err != nil {
		if errors.Is(err, ErrDeclarationPasswordIncorrect) {
			return false, err
		}
		return false, fmt.Errorf("parent: confirm password: %w", err)
	}
	return true, nil
}

// writeDeclaration decides and appends under the (announcement, child) lock,
// against the committed history and a share lock on the authorizing
// relationship, so neither a concurrent declaration nor a concurrent
// revocation of the permission can slip between check and write.
func (s *Service) writeDeclaration(ctx context.Context, accountID, announcementID int64, target *declarationTarget, in DeclarationInput, passwordConfirmed bool) (*usersModels.DeclarationSubmission, bool, error) {
	repo := s.AnnouncementRepo
	if err := repo.LockDeclarationChild(ctx, announcementID, in.StudentID); err != nil {
		return nil, false, err
	}
	rules, version, err := s.lockedDeclarationVersion(ctx, target.tenantID, announcementID, in)
	if err != nil {
		return nil, false, err
	}
	signer, err := repo.HoldDeclarationSigner(ctx, target.tenantID, announcementID, accountID, in.StudentID)
	if err != nil {
		return nil, false, err
	}
	if signer == nil {
		return nil, false, ErrDeclarationNotPermitted
	}
	mine, err := s.latestOwnDeclaration(ctx, accountID, announcementID, in.StudentID, version.ID)
	if err != nil {
		return nil, false, err
	}
	if mine != nil && mine.Action == in.Action {
		mine.VersionNo = version.VersionNo
		return mine, false, nil
	}
	now := time.Now()
	if err := checkDeclarationAction(rules, mine, in.Action, now); err != nil {
		return nil, false, err
	}
	submission := newDeclarationSubmission(target.tenantID, announcementID, accountID, version, signer, in, passwordConfirmed, now)
	if err := repo.InsertDeclarationSubmission(ctx, submission); err != nil {
		return nil, false, fmt.Errorf("parent: record declaration: %w", err)
	}
	submission.VersionNo = version.VersionNo
	return submission, true, nil
}

// lockedDeclarationVersion re-reads the announcement under the child lock and
// requires that the guardian declares on the current version. A new
// declaration for a child whose care ended is refused; a revocation is not.
func (s *Service) lockedDeclarationVersion(ctx context.Context, tenantID, announcementID int64, in DeclarationInput) (declarations.DeclarationRules, *usersModels.DeclarationVersion, error) {
	a, err := s.AnnouncementRepo.FindByID(ctx, announcementID)
	if err != nil {
		return declarations.DeclarationRules{}, nil, fmt.Errorf("parent: reload declaration: %w", err)
	}
	if a == nil || !announcementIsLive(a) || !a.IsDeclaration() {
		return declarations.DeclarationRules{}, nil, ErrAnnouncementNotFound
	}
	version, err := s.AnnouncementRepo.LatestDeclarationVersion(ctx, tenantID, announcementID)
	if err != nil {
		return declarations.DeclarationRules{}, nil, fmt.Errorf("parent: load declaration version: %w", err)
	}
	if version == nil || version.ID != in.VersionID {
		return declarations.DeclarationRules{}, nil, ErrDeclarationVersionChanged
	}
	if in.Action != declarations.DeclarationActionRevoked {
		if err := s.requireCareRunning(ctx, tenantID, in.StudentID); err != nil {
			return declarations.DeclarationRules{}, nil, err
		}
	}
	return feedDeclarationRules(a.Declaration, a.ResponseDeadline), version, nil
}

// checkDeclarationAction maps a refused action to the error the portal can
// explain: the deadline for a late answer, otherwise "not possible".
func checkDeclarationAction(rules declarations.DeclarationRules, mine *usersModels.DeclarationSubmission, action string, now time.Time) error {
	myLatest := ""
	if mine != nil {
		myLatest = mine.Action
	}
	if declarations.DeclarationActionAllowed(rules, true, myLatest, action, now) {
		return nil
	}
	if rules.Closed(now) && action != declarations.DeclarationActionRevoked {
		return ErrDeclarationClosed
	}
	return ErrDeclarationActionNotAllowed
}

// requireCareRunning refuses a new declaration for a child whose care ended.
// A revocation stays possible: withdrawing consent must never be harder than
// giving it.
func (s *Service) requireCareRunning(ctx context.Context, tenantID, studentID int64) error {
	student, err := s.StudentRepo.FindByIDForUpdate(tenant.WithTenantID(ctx, tenantID), studentID)
	if err != nil {
		return err
	}
	if student.CareEndedOn(s.todayDate()) {
		return care.ErrChildCareEnded
	}
	return nil
}

func (s *Service) latestOwnDeclaration(ctx context.Context, accountID, announcementID, studentID, versionID int64) (*usersModels.DeclarationSubmission, error) {
	history, err := s.AnnouncementRepo.ListDeclarationSubmissionsForStudents(ctx, []int64{announcementID}, []int64{studentID})
	if err != nil {
		return nil, fmt.Errorf("parent: load declaration history: %w", err)
	}
	for _, sub := range history { // newest first
		if sub.VersionID == versionID && sub.AccountID != nil && *sub.AccountID == accountID {
			return sub, nil
		}
	}
	return nil, nil
}

func newDeclarationSubmission(tenantID, announcementID, accountID int64, version *usersModels.DeclarationVersion, signer *usersModels.DeclarationSignerContext, in DeclarationInput, passwordConfirmed bool, now time.Time) *usersModels.DeclarationSubmission {
	account, profile := accountID, signer.GuardianProfileID
	submission := &usersModels.DeclarationSubmission{
		TenantID: tenantID, AnnouncementID: announcementID, VersionID: version.ID, StudentID: in.StudentID,
		AccountID: &account, GuardianProfileID: &profile,
		SignerName:   strings.TrimSpace(signer.FirstName + " " + signer.LastName),
		GuardianRole: signer.GuardianRole, Action: in.Action, Method: usersModels.DeclarationMethodSimpleElectronic,
		PasswordConfirmed: passwordConfirmed, ContentHash: version.ContentHash,
		// PostgreSQL keeps microseconds; hashing the stored precision keeps
		// the record hash verifiable after a round trip.
		SubmittedAt: time.UnixMicro(now.UnixMicro()).UTC(),
	}
	submission.RecordHash = DeclarationRecordHash(submission)
	return submission
}

// DeclarationRecordHash is the SHA-256 over a submission's canonical record:
// what makes a later change to the stored row detectable.
func DeclarationRecordHash(submission *usersModels.DeclarationSubmission) string {
	return authorize.ContentFingerprint(submission.CanonicalRecord())
}

func knownDeclarationAction(action string) bool {
	switch action {
	case declarations.DeclarationActionAgreed, declarations.DeclarationActionDeclined,
		declarations.DeclarationActionAcknowledged, declarations.DeclarationActionRevoked:
		return true
	default:
		return false
	}
}

func containsDeclarationChild(children []*usersModels.DeclarationChild, studentID int64) bool {
	for _, c := range children {
		if c.StudentID == studentID {
			return true
		}
	}
	return false
}

func feedDeclarationRules(settings usersModels.AnnouncementDeclarationSettings, deadline *time.Time) declarations.DeclarationRules {
	return declarations.DeclarationRules{
		Kind: settings.Kind, Signers: settings.Signers, Revocable: settings.Revocable, Deadline: deadline,
	}
}
