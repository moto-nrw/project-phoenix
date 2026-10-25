package owners

import (
	"context"
	"errors"
	"log/slog"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// The Träger-Büro account management (#4650) is Identity & Access's; the
// root binds the Träger membership of Organisation & Tenancy, the Audit
// Platform's Träger account ledger and the invitation mail into the outbox.

// TraegerAccountWiring is the configuration the Träger invitation is composed
// with. Dispatcher may be nil: the link is then not mailed, like the other
// identity links. Outbox enqueues into Delivery's outbox in the caller's
// transaction; its worker sends the mail after the commit.
type TraegerAccountWiring struct {
	Dispatcher  email.Dispatcher
	DefaultFrom email.Email
	Outbox      OutboxEnqueuer
	// FrontendURL serves the logo the mail embeds; TraegerURL is the
	// Träger-Portal the link opens.
	FrontendURL string
	TraegerURL  string
	Expiry      time.Duration
	// Backoff spaces the send retries; nil uses the production spacing.
	Backoff []time.Duration
	Logger  *slog.Logger
}

// TraegerAccountDependencies composes the account management's delivery and
// password policy. NewIdentityAccessWithSessions binds the membership and
// ledger ports over the owners it composes the module with.
func TraegerAccountDependencies(wiring *TraegerAccountWiring) *identityBindingPorts.TraegerAccountDependencies {
	if wiring == nil {
		return nil
	}
	return &identityBindingPorts.TraegerAccountDependencies{
		Delivery: traegerInvitationDelivery{
			dispatcher: wiring.Dispatcher, outbox: wiring.Outbox, from: wiring.DefaultFrom,
			frontendURL: wiring.FrontendURL, traegerURL: wiring.TraegerURL, backoff: wiring.Backoff,
		},
		Passwords: passwordPolicy{},
		Expiry:    wiring.Expiry,
		Logger:    wiring.Logger,
	}
}

// traegerAccountDependencies is the serving roots' wiring: the outbox of
// Delivery, the Träger-Portal link base and the invitation lifetime.
func traegerAccountDependencies(dispatcher email.Dispatcher, from email.Email, delivery *deliveryCompose.Runtime, links ServiceLinks, logger *slog.Logger) *identityBindingPorts.TraegerAccountDependencies {
	return TraegerAccountDependencies(&TraegerAccountWiring{
		Dispatcher: dispatcher, DefaultFrom: from, Outbox: OutboxEnqueuer{Runtime: delivery},
		FrontendURL: links.FrontendURL, TraegerURL: links.TraegerURL,
		Expiry: links.InvitationTokenExpiry, Logger: logger,
	})
}

type traegerInvitationDelivery struct {
	dispatcher  email.Dispatcher
	outbox      OutboxEnqueuer
	from        email.Email
	frontendURL string
	traegerURL  string
	backoff     []time.Duration
}

// DispatchTraegerInvitation queues the invitation mail with the invitation.
func (d traegerInvitationDelivery) DispatchTraegerInvitation(ctx context.Context, invitation identityaccess.TraegerInvitation, traegerName string, expiry time.Duration) error {
	if d.dispatcher == nil {
		return nil
	}
	message := *identityMessage(d.from, email.NewEmail("", invitation.Email),
		identityaccess.TraegerInvitationMail(invitation, traegerName, d.frontendURL, d.traegerURL, expiry))
	return d.outbox.EnqueueTransactionalMail(ctx, DurableMail{
		Message: message, Type: "traeger_invitation", ReferenceID: invitation.ID,
		Backoff: d.backoff, ExpiresAt: invitation.ExpiresAt,
	})
}

// TraegerBueroDirectory serves the account management's membership port over
// Organisation & Tenancy. The membership commands join the account
// management's administrative transaction.
type TraegerBueroDirectory struct {
	Organizations organizationtenancy.Capability
}

var errTraegerBueroDirectoryUnavailable = errors.New("the Träger membership directory is not composed")

// FindLiveTraeger reports an active, undeleted Träger.
func (d TraegerBueroDirectory) FindLiveTraeger(ctx context.Context, organizationID int64) (identityaccess.TraegerOrganization, bool, error) {
	if d.Organizations == nil {
		return identityaccess.TraegerOrganization{}, false, errTraegerBueroDirectoryUnavailable
	}
	organization, err := d.Organizations.FindOrganization(ctx, organizationID)
	if errors.Is(err, organizationtenancy.ErrOrganizationNotFound) {
		return identityaccess.TraegerOrganization{}, false, nil
	}
	if err != nil {
		return identityaccess.TraegerOrganization{}, false, err
	}
	if !organization.Active || organization.IsDeleted() {
		return identityaccess.TraegerOrganization{}, false, nil
	}
	return identityaccess.TraegerOrganization{ID: organization.ID, Name: organization.Name}, true, nil
}

// ListActiveTraegerMemberships returns the active memberships of the Träger.
func (d TraegerBueroDirectory) ListActiveTraegerMemberships(ctx context.Context, organizationID int64) ([]identityBindingPorts.TraegerMembership, error) {
	if d.Organizations == nil {
		return nil, errTraegerBueroDirectoryUnavailable
	}
	memberships, err := d.Organizations.ListTraegerMemberships(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	result := make([]identityBindingPorts.TraegerMembership, 0, len(memberships))
	for _, membership := range memberships {
		if membership.Status == organizationtenancy.TraegerMembershipActive {
			result = append(result, identityBindingPorts.TraegerMembership{AccountID: membership.AccountID, ActivatedAt: membership.ActivatedAt})
		}
	}
	return result, nil
}

// ActivateTraegerMembership creates or reactivates the membership.
func (d TraegerBueroDirectory) ActivateTraegerMembership(ctx context.Context, organizationID, accountID int64) error {
	if d.Organizations == nil {
		return errTraegerBueroDirectoryUnavailable
	}
	_, err := d.Organizations.ActivateTraegerMembership(ctx, organizationID, accountID)
	return err
}

// DeactivateTraegerMembership ends an active membership.
func (d TraegerBueroDirectory) DeactivateTraegerMembership(ctx context.Context, organizationID, accountID int64) error {
	if d.Organizations == nil {
		return errTraegerBueroDirectoryUnavailable
	}
	_, err := d.Organizations.DeactivateTraegerMembership(ctx, organizationID, accountID)
	return err
}

// TraegerAccountLedger appends the account management's entries to the
// Audit Platform's Träger account ledger.
type TraegerAccountLedger struct {
	Command auditModels.Command
}

// RecordTraegerAccountChange appends one entry on the caller's transaction.
func (l TraegerAccountLedger) RecordTraegerAccountChange(ctx context.Context, change identityaccess.TraegerAccountChange) error {
	if l.Command == nil {
		return errors.New("audit Träger account change: command is not configured")
	}
	return l.Command.Append(ctx, &auditModels.TraegerAccountChange{
		OrganizationID: change.OrganizationID, ActorAccountID: change.ActorAccountID, Action: change.Action,
		TargetAccountID: change.TargetAccountID, TargetEmail: change.TargetEmail,
		RolesBefore: change.RolesBefore, RolesAfter: change.RolesAfter,
	})
}
