package application

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	identityPorts "github.com/moto-ogs/project-phoenix/modules/identityaccess/ports"
)

// The names of the Träger-Büro (#4693). The export protocol and the history
// of an invoice name who exported or acted; colleagues without "Konten
// verwalten" may not read each other's addresses, so every person carries a
// name per Träger membership. Organisation & Tenancy keeps it on the
// membership; the change joins the administrative unit of the account
// management as a participating command (ADR 0067) and is written to the
// ledger as name_changed.

var _ identityaccess.TraegerMemberProfiles = (*TraegerAccounts)(nil)

// normalizeTraegerMemberName trims the name and refuses an empty one, one
// longer than TraegerMemberNameMaxLength characters, invalid UTF-8 and any
// control character. It is the one place the rule lives.
func normalizeTraegerMemberName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > identityaccess.TraegerMemberNameMaxLength ||
		strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", identityaccess.ErrTraegerInvalidName
	}
	return name, nil
}

// ChangeTraegerMemberName names an active colleague; it needs "Konten
// verwalten" like every change of another person.
func (t *TraegerAccounts) ChangeTraegerMemberName(ctx context.Context, actor identityaccess.TraegerActor, accountID int64, name string) (identityaccess.TraegerMember, error) {
	name, err := normalizeTraegerMemberName(name)
	if err != nil {
		return identityaccess.TraegerMember{}, err
	}
	var result identityaccess.TraegerMember
	err = t.run(ctx, actor, func(txCtx context.Context, unit traegerUnit) error {
		result, err = t.rename(txCtx, actor, unit, accountID, name)
		return err
	})
	return result, err
}

// OwnTraegerProfile answers the acting person's own address and name at the
// Träger of the session; every active member reads it.
func (t *TraegerAccounts) OwnTraegerProfile(ctx context.Context, actor identityaccess.TraegerActor) (identityaccess.TraegerMember, error) {
	var result identityaccess.TraegerMember
	err := t.runMember(ctx, actor, func(txCtx context.Context, unit traegerUnit) error {
		account, err := t.traegerAccount(txCtx, actor.OrganizationID, actor.AccountID)
		if err != nil {
			return err
		}
		membership, _ := unit.member(actor.AccountID)
		result = publicTraegerMember(account, membership)
		return nil
	})
	return result, err
}

// ChangeOwnTraegerName names the acting person; every active member names
// themselves without "Konten verwalten".
func (t *TraegerAccounts) ChangeOwnTraegerName(ctx context.Context, actor identityaccess.TraegerActor, name string) (identityaccess.TraegerMember, error) {
	name, err := normalizeTraegerMemberName(name)
	if err != nil {
		return identityaccess.TraegerMember{}, err
	}
	var result identityaccess.TraegerMember
	err = t.runMember(ctx, actor, func(txCtx context.Context, unit traegerUnit) error {
		result, err = t.rename(txCtx, actor, unit, actor.AccountID, name)
		return err
	})
	return result, err
}

// runMember opens the administrative unit of an acting person and re-checks
// the live Träger, the active membership and the switched-on account, not
// the right "Konten verwalten".
func (t *TraegerAccounts) runMember(ctx context.Context, actor identityaccess.TraegerActor, fn func(context.Context, traegerUnit) error) error {
	if t == nil {
		return identityaccess.ErrTraegerAccountsUnavailable
	}
	if actor.OrganizationID <= 0 || actor.AccountID <= 0 {
		return identityaccess.ErrNoTraegerMembership
	}
	return t.runAdmin(ctx, func(txCtx context.Context) error {
		unit, err := t.memberUnit(txCtx, actor)
		if err != nil {
			return err
		}
		return fn(txCtx, unit)
	})
}

// memberUnit re-checks the acting person: a live Träger, an active
// membership and an account that is switched on. The token may outlive a
// switch-off of the account; the account itself decides.
func (t *TraegerAccounts) memberUnit(ctx context.Context, actor identityaccess.TraegerActor) (traegerUnit, error) {
	traeger, live, err := t.memberships.FindLiveTraeger(ctx, actor.OrganizationID)
	if err != nil {
		return traegerUnit{}, err
	}
	if !live {
		return traegerUnit{}, identityaccess.ErrNoTraegerMembership
	}
	members, err := t.memberships.ListActiveTraegerMemberships(ctx, actor.OrganizationID)
	if err != nil {
		return traegerUnit{}, err
	}
	unit := traegerUnit{name: traeger.Name, members: members}
	if _, ok := unit.member(actor.AccountID); !ok {
		return traegerUnit{}, identityaccess.ErrNoTraegerMembership
	}
	account, found, _, err := t.store.FindLoginAccount(ctx, actor.AccountID, false)
	if err != nil {
		return traegerUnit{}, err
	}
	if !found || !account.Active {
		return traegerUnit{}, identityaccess.ErrAccountInactive
	}
	return unit, nil
}

// rename sets the name of an active member of the unit and writes it to the
// ledger.
func (t *TraegerAccounts) rename(ctx context.Context, actor identityaccess.TraegerActor, unit traegerUnit, accountID int64, name string) (identityaccess.TraegerMember, error) {
	membership, ok := unit.member(accountID)
	if !ok {
		return identityaccess.TraegerMember{}, identityaccess.ErrTraegerMemberNotFound
	}
	if err := t.memberships.NameTraegerMember(ctx, actor.OrganizationID, accountID, name); err != nil {
		return identityaccess.TraegerMember{}, err
	}
	account, err := t.traegerAccount(ctx, actor.OrganizationID, accountID)
	if err != nil {
		return identityaccess.TraegerMember{}, err
	}
	membership.DisplayName = name
	return publicTraegerMember(account, membership), t.appendChange(ctx, identityaccess.TraegerAccountChange{
		OrganizationID: actor.OrganizationID, ActorAccountID: actor.AccountID, Action: identityaccess.TraegerAccountNameChanged,
		TargetAccountID: &accountID, TargetEmail: &account.Email, TargetName: &name,
	})
}

// traegerAccount reads the address and Träger roles of one account.
func (t *TraegerAccounts) traegerAccount(ctx context.Context, organizationID, accountID int64) (domain.TraegerAccount, error) {
	accounts, _, err := t.store.ListTraegerAccounts(ctx, organizationID, []int64{accountID})
	if err != nil {
		return domain.TraegerAccount{}, err
	}
	if len(accounts) != 1 {
		return domain.TraegerAccount{}, identityaccess.ErrTraegerMemberNotFound
	}
	return accounts[0], nil
}

func (t *TraegerAccounts) record(ctx context.Context, actor identityaccess.TraegerActor, action string, targetAccountID *int64, targetEmail *string, before, after []string) error {
	return t.appendChange(ctx, identityaccess.TraegerAccountChange{
		OrganizationID: actor.OrganizationID, ActorAccountID: actor.AccountID, Action: action,
		TargetAccountID: targetAccountID, TargetEmail: targetEmail, RolesBefore: before, RolesAfter: after,
	})
}

func (t *TraegerAccounts) appendChange(ctx context.Context, change identityaccess.TraegerAccountChange) error {
	return t.audit.RecordTraegerAccountChange(ctx, change)
}

func publicTraegerMember(account domain.TraegerAccount, membership identityPorts.TraegerMembership) identityaccess.TraegerMember {
	roles := account.Roles
	if roles == nil {
		roles = []string{}
	}
	return identityaccess.TraegerMember{
		AccountID: account.AccountID, Email: account.Email, Name: membership.DisplayName, Roles: roles, ActivatedAt: membership.ActivatedAt,
	}
}
