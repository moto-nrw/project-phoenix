package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// The history of the Träger-Büro and the notice to the person concerned
// (#5474). The ledger of the Audit Platform holds every change; the history
// shows it to the people who manage the accounts, and the person whose roles
// changed or who was removed learns it by mail in the unit of the change.

// ListTraegerAccountHistory lists one page of the changes of the Träger of
// the acting person, newest first, with the address of each acting person.
func (t *TraegerAccounts) ListTraegerAccountHistory(ctx context.Context, actor identityaccess.TraegerActor, beforeID int64) (identityaccess.TraegerAccountHistory, error) {
	result := identityaccess.TraegerAccountHistory{Entries: []identityaccess.TraegerAccountHistoryEntry{}}
	err := t.run(ctx, actor, func(txCtx context.Context, _ traegerUnit) error {
		changes, err := t.audit.ListTraegerAccountChanges(txCtx, actor.OrganizationID, beforeID, identityaccess.TraegerAccountHistoryPageSize+1)
		if err != nil {
			return err
		}
		if len(changes) > identityaccess.TraegerAccountHistoryPageSize {
			changes, result.HasMore = changes[:identityaccess.TraegerAccountHistoryPageSize], true
		}
		emails, err := t.actorEmails(txCtx, actor.OrganizationID, changes)
		if err != nil {
			return err
		}
		for _, change := range changes {
			result.Entries = append(result.Entries, historyEntry(change, emails[change.ActorAccountID]))
		}
		return nil
	})
	return result, err
}

// actorEmails reads the address of every acting person of the changes in
// one statement.
func (t *TraegerAccounts) actorEmails(ctx context.Context, organizationID int64, changes []identityPorts.TraegerAccountLedgerEntry) (map[int64]string, error) {
	actorIDs := make([]int64, 0, len(changes))
	for _, change := range changes {
		if !slices.Contains(actorIDs, change.ActorAccountID) {
			actorIDs = append(actorIDs, change.ActorAccountID)
		}
	}
	accounts, _, err := t.store.ListTraegerAccounts(ctx, organizationID, actorIDs)
	if err != nil {
		return nil, err
	}
	emails := make(map[int64]string, len(accounts))
	for _, account := range accounts {
		emails[account.AccountID] = account.Email
	}
	return emails, nil
}

func historyEntry(change identityPorts.TraegerAccountLedgerEntry, actorEmail string) identityaccess.TraegerAccountHistoryEntry {
	entry := identityaccess.TraegerAccountHistoryEntry{
		ID: change.ID, OccurredAt: change.OccurredAt, Action: change.Action,
		ActorAccountID: change.ActorAccountID, ActorEmail: actorEmail,
		TargetAccountID: change.TargetAccountID, RolesBefore: change.RolesBefore, RolesAfter: change.RolesAfter,
	}
	if change.TargetEmail != nil {
		entry.TargetEmail = *change.TargetEmail
	}
	return entry
}

// notifyAndRecord mails the person concerned and writes the change to the
// ledger, both in the unit of the change.
func (t *TraegerAccounts) notifyAndRecord(ctx context.Context, actor identityaccess.TraegerActor, unit traegerUnit, action string, target domain.TraegerAccount, before, after []string) error {
	if err := t.notifyTraegerAccountChange(ctx, actor, unit, action, target, before, after); err != nil {
		return err
	}
	return t.record(ctx, actor, action, &target.AccountID, &target.Email, before, after)
}

// notifyTraegerAccountChange queues the mail to the person whose roles
// changed or who was removed, in the unit of the change. A role change that
// keeps the same roles changes nothing and mails nothing.
func (t *TraegerAccounts) notifyTraegerAccountChange(ctx context.Context, actor identityaccess.TraegerActor, unit traegerUnit, action string, target domain.TraegerAccount, before, after []string) error {
	if action == identityaccess.TraegerAccountRolesChanged && slices.Equal(slices.Sorted(slices.Values(before)), slices.Sorted(slices.Values(after))) {
		return nil
	}
	actorEmail := target.Email
	if actor.AccountID != target.AccountID {
		accounts, _, err := t.store.ListTraegerAccounts(ctx, actor.OrganizationID, []int64{actor.AccountID})
		if err != nil {
			return err
		}
		if len(accounts) != 1 {
			return identityaccess.ErrNoTraegerMembership
		}
		actorEmail = accounts[0].Email
	}
	return t.delivery.DispatchTraegerAccountChange(ctx, identityaccess.TraegerAccountChangeNotice{
		OrganizationID: actor.OrganizationID, TraegerName: unit.name, Action: action,
		TargetAccountID: target.AccountID, TargetEmail: target.Email, ActorAccountID: actor.AccountID, ActorEmail: actorEmail,
		RolesBefore: before, RolesAfter: after, OccurredAt: t.now(),
	})
}
