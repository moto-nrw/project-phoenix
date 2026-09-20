package application

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Bulk invitation (#3378): a school whose children and contacts already
// exist invites the guardians of many children in one run instead of opening
// every child. The run is keyed by guardian profile, never by e-mail lookup:
// the relationships exist already, so nothing is created or linked here, and
// one guardian of several selected children gets one mail. Accepting it
// links the profile to the account, which opens every child the profile is
// linked to.

const opGuardianBulkInvite = "bulk invite guardians"

// bulkInviteCandidate is one guardian profile with a full role on at least
// one selected child.
type bulkInviteCandidate struct {
	profileID  int64
	studentIDs []int64
}

// BulkInviteToStudents classifies the guardians of the given children and,
// unless DryRun is set, invites the ones that can be invited. It runs on the
// caller's tenant transaction, or opens one when called directly; any error
// leaves nothing behind.
func (l *AccountLifecycle) BulkInviteToStudents(ctx context.Context, req domain.BulkInviteRequest) (result *domain.BulkInviteResult, err error) {
	if err := validateBulkInvite(req); err != nil {
		return nil, invalidGuardianInvitation(opGuardianBulkInvite, err)
	}
	tenantID := l.runtime.TenantID(ctx)
	err = l.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err = l.bulkInviteToStudents(txCtx, req, tenantID)
		return err
	})
	return result, err
}

func (l *AccountLifecycle) bulkInviteToStudents(ctx context.Context, req domain.BulkInviteRequest, tenantID int64) (*domain.BulkInviteResult, error) {
	links, err := l.guardians.ListStudentGuardianLinksByStudents(ctx, req.StudentIDs)
	if err != nil {
		return nil, failed(opGuardianBulkInvite, err)
	}
	candidates, restricted := l.bulkInviteCandidates(links)
	result := &domain.BulkInviteResult{DryRun: req.DryRun, SkippedRestricted: restricted, Problems: []domain.BulkInviteProblem{}}
	if len(candidates) == 0 {
		return result, nil
	}

	profiles, open, err := l.loadBulkInviteProfiles(ctx, req, tenantID, candidates)
	if err != nil {
		return nil, err
	}
	run, err := l.newBulkInviteRun(ctx, req, tenantID, result, profiles)
	if err != nil {
		return nil, err
	}
	if err := l.processBulkInviteCandidates(ctx, run, candidates, profiles, open); err != nil {
		return nil, err
	}
	if err := l.nameBulkInviteProblems(ctx, run.problemStudents, result); err != nil {
		return nil, err
	}
	l.logBulkInvite(req, result)
	return result, nil
}

func (l *AccountLifecycle) loadBulkInviteProfiles(ctx context.Context, req domain.BulkInviteRequest, tenantID int64, candidates []bulkInviteCandidate) (map[int64]domain.GuardianProfile, map[int64][]domain.GuardianInvitation, error) {
	profileIDs := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		profileIDs = append(profileIDs, candidate.profileID)
	}
	if !req.DryRun {
		if err := l.lockBulkInviteProfiles(ctx, tenantID, profileIDs); err != nil {
			return nil, nil, err
		}
	}
	profiles, err := l.guardians.FindGuardianProfiles(ctx, profileIDs)
	if err != nil {
		return nil, nil, failed(opGuardianBulkInvite, err)
	}
	open, err := l.openInvitationsByProfile(ctx, profileIDs)
	if err != nil {
		return nil, nil, err
	}
	return profiles, open, nil
}

func (l *AccountLifecycle) newBulkInviteRun(ctx context.Context, req domain.BulkInviteRequest, tenantID int64, result *domain.BulkInviteResult, profiles map[int64]domain.GuardianProfile) (*bulkInviteRun, error) {
	run := &bulkInviteRun{req: req, tenantID: tenantID, result: result, seenEmails: map[string]bool{}, existingAccounts: map[string]domain.Account{}}
	if !req.DryRun {
		existingAccounts, err := l.bulkInviteExistingAccounts(ctx, profiles)
		if err != nil {
			return nil, err
		}
		run.existingAccounts = existingAccounts
	}
	return run, nil
}

func (l *AccountLifecycle) processBulkInviteCandidates(ctx context.Context, run *bulkInviteRun, candidates []bulkInviteCandidate, profiles map[int64]domain.GuardianProfile, open map[int64][]domain.GuardianInvitation) error {
	for _, candidate := range candidates {
		profile, ok := profiles[candidate.profileID]
		if !ok {
			continue
		}
		if err := l.bulkInviteOne(ctx, run, candidate, profile, open[profile.ID]); err != nil {
			return err
		}
	}
	return nil
}

func (l *AccountLifecycle) logBulkInvite(req domain.BulkInviteRequest, result *domain.BulkInviteResult) {
	l.logger.Info("guardian bulk invitation",
		slog.Int64("created_by", req.CreatedBy),
		slog.Bool("dry_run", req.DryRun),
		slog.Int("students", len(req.StudentIDs)),
		slog.Int("invited", result.Invited),
		slog.Int("linked_existing_account", result.LinkedExistingAccount),
		slog.Int("resent", result.Resent),
		slog.Int("problems", len(result.Problems)),
	)
}

func (l *AccountLifecycle) lockBulkInviteProfiles(ctx context.Context, tenantID int64, profileIDs []int64) error {
	for _, profileID := range profileIDs {
		key := fmt.Sprintf("guardian-bulk-invite:%d:%d", tenantID, profileID)
		if err := l.runtime.AcquireLock(ctx, key); err != nil {
			return failed(opGuardianBulkInvite, fmt.Errorf("lock guardian profile %d: %w", profileID, err))
		}
	}
	return nil
}

func (l *AccountLifecycle) bulkInviteExistingAccounts(ctx context.Context, profiles map[int64]domain.GuardianProfile) (map[string]domain.Account, error) {
	emails := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if !profile.HasAccount {
			emails = append(emails, profile.Email)
		}
	}
	accounts, err := l.sessions.FindAccountsByEmails(ctx, emails)
	if err != nil {
		return nil, failed(opGuardianBulkInvite, err)
	}
	return accounts, nil
}

func validateBulkInvite(req domain.BulkInviteRequest) error {
	if req.CreatedBy <= 0 {
		return fmt.Errorf("created_by is required")
	}
	if len(req.StudentIDs) == 0 {
		return fmt.Errorf("at least one student is required")
	}
	if len(req.StudentIDs) > domain.BulkInviteMaxStudents {
		return fmt.Errorf("at most %d students per run", domain.BulkInviteMaxStudents)
	}
	for _, id := range req.StudentIDs {
		if id <= 0 {
			return fmt.Errorf("student IDs must be positive")
		}
	}
	return nil
}

// bulkInviteCandidates groups the links by guardian and keeps the ones with a
// full guardian role. A restrictive link is a deliberate staff decision; the
// bulk run never upgrades it (#2172), it only counts the contact.
func (l *AccountLifecycle) bulkInviteCandidates(links []domain.StudentGuardianLink) ([]bulkInviteCandidate, int) {
	full := map[int64][]int64{}
	seen := map[int64]bool{}
	for _, link := range links {
		seen[link.GuardianProfileID] = true
		if l.guardians.GuardianRoleClass(link.GuardianRole) == domain.GuardianRoleFull {
			full[link.GuardianProfileID] = append(full[link.GuardianProfileID], link.StudentID)
		}
	}
	candidates := make([]bulkInviteCandidate, 0, len(full))
	for profileID, studentIDs := range full {
		slices.Sort(studentIDs)
		candidates = append(candidates, bulkInviteCandidate{profileID: profileID, studentIDs: slices.Compact(studentIDs)})
	}
	slices.SortFunc(candidates, func(a, b bulkInviteCandidate) int {
		if a.profileID < b.profileID {
			return -1
		}
		return 1
	})
	return candidates, len(seen) - len(full)
}

func (l *AccountLifecycle) openInvitationsByProfile(ctx context.Context, profileIDs []int64) (map[int64][]domain.GuardianInvitation, error) {
	invitations, err := l.invitations.ListOpenGuardianInvitations(ctx, profileIDs, time.Now())
	if err != nil {
		return nil, failed(opGuardianBulkInvite, err)
	}
	byProfile := map[int64][]domain.GuardianInvitation{}
	for _, invitation := range invitations {
		byProfile[invitation.GuardianProfileID] = append(byProfile[invitation.GuardianProfileID], invitation)
	}
	return byProfile, nil
}

type bulkInviteRun struct {
	req        domain.BulkInviteRequest
	tenantID   int64
	result     *domain.BulkInviteResult
	seenEmails map[string]bool
	// existingAccounts was read in one statement before the loop. It is only
	// needed on the write path; previews deliberately remain side-effect free.
	existingAccounts map[string]domain.Account
	// problemStudents keeps the children per problem, in result order, until
	// the names are resolved in one read.
	problemStudents [][]int64
}

func (r *bulkInviteRun) problem(candidate bulkInviteCandidate, profile domain.GuardianProfile, reason string) {
	r.result.Problems = append(r.result.Problems, domain.BulkInviteProblem{
		GuardianProfileID: profile.ID, GuardianName: profile.FullName(), Reason: reason,
	})
	r.problemStudents = append(r.problemStudents, candidate.studentIDs)
}

// bulkInviteOne decides one guardian. The order matters: an active account
// needs nothing, an address problem blocks everything else, and an open
// invitation is only touched when the school asked for it.
func (l *AccountLifecycle) bulkInviteOne(ctx context.Context, run *bulkInviteRun, candidate bulkInviteCandidate, profile domain.GuardianProfile, open []domain.GuardianInvitation) error {
	email := strings.ToLower(strings.TrimSpace(profile.Email))
	active, err := l.handleActiveBulkGuardian(ctx, run, candidate, profile, email)
	if err != nil {
		return err
	}
	if active {
		return nil
	}
	if l.skipBulkInviteEmailProblem(run, candidate, profile, email) {
		return nil
	}
	linked, err := l.linkBulkExistingAccount(ctx, run, candidate, profile, email)
	if err != nil {
		return err
	}
	if linked {
		return nil
	}
	return l.inviteEligibleBulkGuardian(ctx, run, candidate, profile, email, open)
}

func (l *AccountLifecycle) handleActiveBulkGuardian(ctx context.Context, run *bulkInviteRun, candidate bulkInviteCandidate, profile domain.GuardianProfile, email string) (bool, error) {
	if !profile.HasAccount {
		return false, nil
	}
	if email != "" && run.seenEmails[email] {
		run.problem(candidate, profile, domain.BulkInviteProblemDuplicateEmail)
		return true, nil
	}
	if !run.req.DryRun {
		if err := l.restoreBulkGuardianTenantAccess(ctx, profile); err != nil {
			return false, err
		}
	}
	if email != "" {
		run.seenEmails[email] = true
	}
	run.result.SkippedActive++
	return true, nil
}

func (l *AccountLifecycle) restoreBulkGuardianTenantAccess(ctx context.Context, profile domain.GuardianProfile) error {
	if profile.AccountID == nil {
		return failed(opGuardianBulkInvite, fmt.Errorf("guardian profile %d has no account ID", profile.ID))
	}
	if _, err := l.sessions.GrantGuardianTenantAccess(ctx, *profile.AccountID); err != nil {
		return failed(opGuardianBulkInvite, fmt.Errorf("grant guardian school access: %w", err))
	}
	// The selected link already has a full guardian role, which grants the
	// relationship-level parent portal permission. Only the tenant mapping can
	// have been removed and needs restoring here.
	return nil
}

func (l *AccountLifecycle) linkBulkExistingAccount(ctx context.Context, run *bulkInviteRun, candidate bulkInviteCandidate, profile domain.GuardianProfile, email string) (bool, error) {
	account, found := run.existingAccounts[email]
	if !found {
		return false, nil
	}
	if err := l.attachExistingAccount(ctx, &profile, account); err != nil {
		return false, err
	}
	req := domain.InviteToStudentRequest{StudentID: candidate.studentIDs[0], Email: email, CreatedBy: run.req.CreatedBy}
	if _, err := l.resolveInviteNow(ctx, req, profile, run.tenantID, false, false); err != nil {
		return false, err
	}
	run.result.LinkedExistingAccount++
	return true, nil
}

func (l *AccountLifecycle) skipBulkInviteEmailProblem(run *bulkInviteRun, candidate bulkInviteCandidate, profile domain.GuardianProfile, email string) bool {
	if reason := bulkInviteEmailProblem(email, run.seenEmails); reason != "" {
		run.problem(candidate, profile, reason)
		return true
	}
	run.seenEmails[email] = true
	return false
}

func (l *AccountLifecycle) inviteEligibleBulkGuardian(ctx context.Context, run *bulkInviteRun, candidate bulkInviteCandidate, profile domain.GuardianProfile, email string, open []domain.GuardianInvitation) error {
	if len(open) > 0 && !run.req.ResendOpen {
		run.result.SkippedOpen++
		return nil
	}
	if len(open) > 0 {
		resent, err := l.resendOpenInvitation(ctx, run.req.DryRun, profile, open)
		if err != nil || resent {
			if resent {
				run.result.Resent++
			}
			return err
		}
	}
	if run.req.DryRun {
		run.result.Invited++
		return nil
	}
	return l.bulkInviteFresh(ctx, run, candidate, profile, email, open)
}

func bulkInviteEmailProblem(email string, seen map[string]bool) string {
	if email == "" {
		return domain.BulkInviteProblemMissingEmail
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return domain.BulkInviteProblemInvalidEmail
	}
	if seen[email] {
		return domain.BulkInviteProblemDuplicateEmail
	}
	return ""
}

// resendOpenInvitation mails the redeemable invitation again and restarts its
// window, so nobody receives a link that expires within the hour. A request
// still waiting for staff approval is not redeemable; the caller then issues
// a staff invitation, which supersedes it.
func (l *AccountLifecycle) resendOpenInvitation(ctx context.Context, dryRun bool, profile domain.GuardianProfile, open []domain.GuardianInvitation) (bool, error) {
	for _, invitation := range open {
		if invitation.IsPendingApproval() {
			continue
		}
		if dryRun {
			return true, nil
		}
		invitation.ExpiresAt = time.Now().Add(l.delivery.InvitationExpiry(ctx))
		invitation.EmailSentAt = nil
		invitation.EmailError = nil
		if err := l.invitations.UpdateGuardianInvitation(ctx, invitation); err != nil {
			return false, failed(opGuardianBulkInvite, err)
		}
		l.delivery.EnqueueInvitationEmail(ctx, invitation, profile, l.delivery.SchoolName(ctx, invitation.TenantID))
		return true, nil
	}
	return false, nil
}

// bulkInviteFresh runs the single-invite resolve for the profile itself: an
// account that already owns the address is attached and told where to log
// in, everyone else gets a token invitation anchored to the first child.
func (l *AccountLifecycle) bulkInviteFresh(ctx context.Context, run *bulkInviteRun, candidate bulkInviteCandidate, profile domain.GuardianProfile, email string, open []domain.GuardianInvitation) error {
	req := domain.InviteToStudentRequest{StudentID: candidate.studentIDs[0], Email: email, CreatedBy: run.req.CreatedBy}
	if !profile.HasAccount && len(open) == 0 {
		invitation, err := l.insertStudentInvitation(ctx, req, profile, run.tenantID, domain.GuardianInvitationApprovalNotRequired, false, false)
		if err != nil {
			return err
		}
		l.delivery.EnqueueInvitationEmail(ctx, invitation, profile, l.delivery.SchoolName(ctx, invitation.TenantID))
		run.result.Invited++
		return nil
	}
	invited, err := l.resolveInviteNow(ctx, req, profile, run.tenantID, false, false)
	if err != nil {
		return err
	}
	if invited.Outcome == domain.InviteOutcomeInvited {
		run.result.Invited++
		return nil
	}
	run.result.LinkedExistingAccount++
	return nil
}

// nameBulkInviteProblems resolves the children of the guardians the run
// could not reach, so the school knows where to fix the address.
func (l *AccountLifecycle) nameBulkInviteProblems(ctx context.Context, problemStudents [][]int64, result *domain.BulkInviteResult) error {
	if len(problemStudents) == 0 {
		return nil
	}
	studentIDs := slices.Concat(problemStudents...)
	slices.Sort(studentIDs)
	students, err := l.guardians.FindStudents(ctx, slices.Compact(studentIDs))
	if err != nil {
		return failed(opGuardianBulkInvite, err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	names, err := l.guardians.FindPersonNamesByIDs(ctx, personIDs)
	if err != nil {
		return failed(opGuardianBulkInvite, err)
	}
	for i, ids := range problemStudents {
		for _, id := range ids {
			name, ok := names[students[id].PersonID]
			if !ok {
				continue
			}
			result.Problems[i].StudentNames = append(result.Problems[i].StudentNames, strings.TrimSpace(name.FirstName+" "+name.LastName))
		}
	}
	return nil
}
