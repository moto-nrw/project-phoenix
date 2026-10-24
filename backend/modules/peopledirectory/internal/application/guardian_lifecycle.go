package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// errGuardianRolePresetsUnbound refuses a relationship role write in a graph
// that did not bind the Security Runtime's role presets.
var errGuardianRolePresetsUnbound = errors.New("people directory: the guardian role presets are not bound")

// GuardianLifecycle serves the guardian profile and relationship writes of the
// Identity & Access invitation, approval and revocation flows
// (peopledirectory.GuardianLifecycleCommand). It decides the role a link
// takes and when an account may move between profiles; the rows are written
// through the guardian store and the two other owners of a relationship.
type GuardianLifecycle struct {
	guardians *GuardianService
	// presets is the role policy. Optional: a graph without it refuses only
	// the relationship inserts and promotions.
	presets ports.GuardianRolePresets
}

func NewGuardianLifecycle(guardians *GuardianService, presets ports.GuardianRolePresets) *GuardianLifecycle {
	if guardians == nil {
		panic("people directory application: the guardian service is required")
	}
	return &GuardianLifecycle{guardians: guardians, presets: presets}
}

func (s *GuardianLifecycle) DeleteGuardianProfile(ctx context.Context, guardianID int64) error {
	g := s.guardians
	return lifecycleError(observeRun(ctx, g.observe, "delete_guardian_profile", g.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if g.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		deleted, writeStats, err := g.store.DeleteGuardian(txCtx, guardianID)
		stats.Add(writeStats)
		if err == nil && !deleted {
			return domain.ErrGuardianNotFound
		}
		return err
	}))
}

// DeleteChildlessGuardiansBefore deletes the tenant's profiles that have been
// without a child since before cutoff (#4043), except those openInvitations
// still names: an invitation in flight would otherwise lose its contact. It
// runs in a savepoint of the nightly cleanup's tenant transaction and never
// opens its own, so the deletion commits with its deletion-log entry.
func (s *GuardianLifecycle) DeleteChildlessGuardiansBefore(ctx context.Context, cutoff time.Time, openInvitations peopledirectory.OpenGuardianInvitations) (deleted int, err error) {
	if cutoff.IsZero() || openInvitations == nil {
		return 0, errors.New("people directory: a cutoff and the invitation check are required to delete childless guardians")
	}
	g := s.guardians
	if g.tx.TenantID(ctx) <= 0 {
		return 0, domain.ErrTenantRequired
	}
	err = observeRun(ctx, g.observe, "delete_childless_guardians", g.tx.RunSavepoint, func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, err = s.deleteChildlessGuardians(txCtx, cutoff, openInvitations, stats)
		return err
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

// deleteChildlessGuardians marks, locks and deletes inside the savepoint.
func (s *GuardianLifecycle) deleteChildlessGuardians(ctx context.Context, cutoff time.Time, openInvitations peopledirectory.OpenGuardianInvitations, stats *domain.OperationStats) (int, error) {
	store := s.guardians.store
	markStats, err := store.MarkChildlessGuardians(ctx)
	stats.Add(markStats)
	if err != nil {
		return 0, err
	}
	ids, readStats, err := store.LockChildlessGuardiansBefore(ctx, cutoff)
	stats.Add(readStats)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	invited, err := openInvitations(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("people directory: check open guardian invitations: %w", err)
	}
	due := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !invited[id] {
			due = append(due, id)
		}
	}
	deleted, writeStats, err := store.DeleteChildlessGuardians(ctx, due, cutoff)
	stats.Add(writeStats)
	return deleted, err
}

// LinkGuardianProfileToAccount binds the account under the account's advisory
// lock and the profiles' row locks. A removed contact may leave the account
// on a childless profile (#3477); only that orphaned binding moves, and both
// profiles keep their relationships.
func (s *GuardianLifecycle) LinkGuardianProfileToAccount(ctx context.Context, guardianID, accountID int64) error {
	if guardianID <= 0 || accountID <= 0 {
		return peopledirectory.ErrGuardianAccountConflict
	}
	g := s.guardians
	return lifecycleError(observeRun(ctx, g.observe, "link_guardian_profile_to_account", g.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if g.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		return g.tx.RunSavepoint(txCtx, func(ctx context.Context) error {
			return s.linkAccount(ctx, guardianID, accountID, stats)
		})
	}))
}

func (s *GuardianLifecycle) linkAccount(ctx context.Context, guardianID, accountID int64, stats *domain.OperationStats) error {
	store := s.guardians.store
	lockStats, err := store.LockGuardianAccount(ctx, accountID)
	stats.Add(lockStats)
	if err != nil {
		return err
	}
	if s.guardians.owners == nil {
		return domain.ErrGuardianLinkOwnersUnbound
	}
	live, err := s.guardians.owners.LockLiveGuardianAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if !live {
		return domain.ErrGuardianAccountUnavailable
	}
	holders, readStats, err := store.LockAccountHolders(ctx, guardianID, accountID)
	stats.Add(readStats)
	if err != nil {
		return err
	}
	target, previous := splitAccountHolders(holders, guardianID)
	if target == nil {
		return domain.ErrGuardianNotFound
	}
	if target.AccountID != nil && *target.AccountID != accountID {
		return peopledirectory.ErrGuardianAccountConflict
	}
	if previous != nil {
		if err := s.releaseAccount(ctx, previous.ID, stats); err != nil {
			return err
		}
	}
	linked, writeStats, err := store.SetGuardianAccount(ctx, guardianID, &accountID)
	stats.Add(writeStats)
	if err == nil && !linked {
		return domain.ErrGuardianNotFound
	}
	return err
}

// splitAccountHolders separates the profile to link from the profile that
// holds the account now.
func splitAccountHolders(holders []domain.GuardianAccountHolder, guardianID int64) (target, previous *domain.GuardianAccountHolder) {
	for i := range holders {
		if holders[i].ID == guardianID {
			target = &holders[i]
		} else {
			previous = &holders[i]
		}
	}
	return target, previous
}

// releaseAccount clears the account of the profile that holds it, refusing
// while that profile still has children: they would lose their parent.
func (s *GuardianLifecycle) releaseAccount(ctx context.Context, holderID int64, stats *domain.OperationStats) error {
	store := s.guardians.store
	linked, readStats, err := store.GuardianHasLinks(ctx, holderID)
	stats.Add(readStats)
	if err != nil {
		return err
	}
	if linked {
		return peopledirectory.ErrGuardianAccountConflict
	}
	_, writeStats, err := store.SetGuardianAccount(ctx, holderID, nil)
	stats.Add(writeStats)
	return err
}

// LinkInvitedGuardian gives the relationship the default preset of its type,
// as given by the invitation, and writes it with its pickup permission and
// portal access unless the pair is linked already.
func (s *GuardianLifecycle) LinkInvitedGuardian(ctx context.Context, link peopledirectory.InvitedGuardianLink) (bool, error) {
	if s.presets == nil {
		return false, errGuardianRolePresetsUnbound
	}
	role, permissions, err := s.preset(s.presets.DefaultRole(link.RelationshipType, false, false, false))
	if err != nil {
		return false, err
	}
	linkID, err := s.guardians.LinkContact(ctx, domain.GuardianLinkRecord{
		StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: role,
		EmergencyPriority: link.EmergencyPriority, Permissions: permissions,
	})
	return linkID > 0, lifecycleError(err)
}

// PromoteGuardianLinkToLegalGuardian writes the legal guardian role on the
// relationship and its permissions on the portal access under the
// relationship row lock. The other columns and the pickup permission stay as
// stored.
func (s *GuardianLifecycle) PromoteGuardianLinkToLegalGuardian(ctx context.Context, linkID int64) error {
	if s.presets == nil {
		return errGuardianRolePresetsUnbound
	}
	role, permissions, err := s.preset(domain.GuardianRoleLegalGuardian)
	if err != nil {
		return err
	}
	g := s.guardians
	return lifecycleError(observeRun(ctx, g.observe, "promote_guardian_link", g.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if g.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		if g.owners == nil {
			return domain.ErrGuardianLinkOwnersUnbound
		}
		return g.tx.RunSavepoint(txCtx, func(ctx context.Context) error {
			return s.promote(ctx, linkID, role, permissions, stats)
		})
	}))
}

func (s *GuardianLifecycle) promote(ctx context.Context, linkID int64, role string, permissions json.RawMessage, stats *domain.OperationStats) error {
	g := s.guardians
	found, lockStats, err := g.store.LockLinkForPickup(ctx, linkID, nil)
	stats.Add(lockStats)
	if err != nil {
		return err
	}
	if !found {
		return peopledirectory.ErrGuardianLinkNotFound
	}
	_, writeStats, err := g.store.SetLinkRole(ctx, linkID, role)
	stats.Add(writeStats)
	if err != nil {
		return err
	}
	_, err = g.owners.SetGuardianStudentPermissions(ctx, g.tx.TenantID(ctx), linkID, permissions)
	return err
}

func (s *GuardianLifecycle) DeleteStudentGuardianLink(ctx context.Context, linkID int64) error {
	g := s.guardians
	return lifecycleError(observeRun(ctx, g.observe, "delete_student_guardian_link", g.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		if g.tx.TenantID(txCtx) <= 0 {
			return domain.ErrTenantRequired
		}
		writeStats, err := g.store.DeleteLink(txCtx, linkID)
		stats.Add(writeStats)
		return err
	}))
}

// preset returns the stored role of the preset and the permissions object it
// grants, each granted permission set to true.
func (s *GuardianLifecycle) preset(role string) (string, json.RawMessage, error) {
	normalized, granted := s.presets.Preset(role)
	permissions := make(map[string]bool, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	encoded, err := json.Marshal(permissions)
	if err != nil {
		return "", nil, fmt.Errorf("people directory: encode guardian permissions: %w", err)
	}
	return normalized, encoded, nil
}

// lifecycleError translates the guardian store's outcomes into the public
// contract.
func lifecycleError(err error) error {
	var invalid *domain.GuardianInvalidError
	switch {
	case errors.As(err, &invalid):
		return &peopledirectory.InvalidGuardianError{Reason: invalid.Reason}
	case errors.Is(err, domain.ErrGuardianNotFound):
		return peopledirectory.ErrGuardianNotFound
	case errors.Is(err, domain.ErrGuardianAccountUnavailable):
		return peopledirectory.ErrGuardianAccountConflict
	default:
		return err
	}
}
