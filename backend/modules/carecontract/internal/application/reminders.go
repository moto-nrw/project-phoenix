package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Reminders (#4679): a sent contract that waits for its signature gets a
// reminder mail with a new one-time link once its last mail or reminder is
// as many days old as the Träger set, at most carecontract.MaxReminders
// times. A Träger that sets 0 days sends none.

// ReminderPorts are the root-bound ports of the reminders.
type ReminderPorts struct {
	Organizations carecontract.TraegerOrganizations
	Settings      carecontract.ReminderSettings
	Sender        carecontract.TraegerSender
	Post          carecontract.Post
	Runtime       carecontract.Runtime
	Links         carecontract.SigningLinks
}

// Reminders serves carecontract.Reminders.
type Reminders struct {
	store ContractStore
	ports ReminderPorts
	clock Clock
}

var _ carecontract.Reminders = (*Reminders)(nil)

// NewReminders builds the reminders over the root-bound ports.
func NewReminders(store ContractStore, ports ReminderPorts, clock Clock) *Reminders {
	if store == nil || ports.Organizations == nil || ports.Settings == nil || ports.Sender == nil || ports.Post == nil ||
		ports.Runtime == nil || ports.Links.Tokens == nil || ports.Links.Fingerprint == nil {
		panic("care contract: the reminders need the store, the Träger, the settings, the sender, the post, the runtime and the tokens")
	}
	return &Reminders{store: store, ports: ports, clock: clock}
}

// RemindDue runs every Träger in its own transaction; one Träger's failure
// does not hold back the others and is answered after all ran.
func (r *Reminders) RemindDue(ctx context.Context) error {
	organizations, err := r.ports.Organizations.ActiveTraegerIDs(ctx)
	if err != nil {
		return fmt.Errorf("care contract reminders: list Träger: %w", err)
	}
	var failures []error
	for _, organizationID := range organizations {
		err := r.ports.Runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
			return r.remindTraeger(txCtx, organizationID)
		})
		if err != nil {
			failures = append(failures, fmt.Errorf("Träger %d: %w", organizationID, err))
		}
	}
	return errors.Join(failures...)
}

func (r *Reminders) remindTraeger(ctx context.Context, organizationID int64) error {
	now := r.clock.now()
	days, err := r.ports.Settings.SignatureReminderDays(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("reminder days: %w", err)
	}
	if days <= 0 {
		return nil
	}
	due, err := r.store.DueReminders(ctx, organizationID, now.Add(-time.Duration(days)*24*time.Hour), carecontract.MaxReminders)
	if err != nil || len(due) == 0 {
		return err
	}
	letterhead, err := r.ports.Sender.TraegerLetterhead(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("letterhead: %w", err)
	}
	for _, candidate := range due {
		if err := r.remind(ctx, organizationID, firstLine(textLines(letterhead)), candidate); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reminders) remind(ctx context.Context, organizationID int64, traeger string, candidate ports.ReminderCandidate) error {
	record, err := r.store.Contract(ctx, organizationID, candidate.ContractID, true)
	if err != nil {
		return err
	}
	if record.Summary.Status != carecontract.StatusSent {
		return nil
	}
	document, err := r.store.CurrentDocument(ctx, organizationID, candidate.ContractID)
	if err != nil {
		return err
	}
	link, err := createSigningLink(ctx, r.store, r.ports.Links, r.clock, organizationID, candidate.ContractID, document.ID)
	if err != nil {
		return err
	}
	party, _ := recipient(record.Parties)
	outboxID, err := r.ports.Post.EnqueueContractMail(ctx, carecontract.ContractMail{
		OrganizationID: organizationID, ContractID: candidate.ContractID, Recipient: candidate.Recipient,
		GuardianName: fullName(party.FirstName, party.LastName), TraegerName: firstText(traeger, record.Version.Terms.ContractPartner),
		ChildName: fullName(record.Version.ChildFirstName, record.Version.ChildLastName),
		Modules:   joinGerman(ModuleNames(record.Version.Services)), Filename: document.File.Filename,
		IdempotencyKey: fmt.Sprintf("care-contract:%d:reminder:%d", candidate.ContractID, candidate.Number),
		Purpose:        carecontract.MailPurposeReminder, SigningURL: link.url, LinkExpiresOn: link.expiresOn,
	})
	if err != nil {
		return fmt.Errorf("enqueue reminder: %w", err)
	}
	if err := r.store.InsertReminder(ctx, organizationID, candidate.ContractID, candidate.Number, outboxID, candidate.Recipient); err != nil {
		return err
	}
	contractID, schoolID := record.Summary.ID, record.Summary.SchoolID
	return r.store.InsertEvents(ctx, organizationID, []Event{{
		ContractID: &contractID, SchoolID: &schoolID, Action: ports.ActionRemind,
		Details: map[string]any{"number": candidate.Number},
	}})
}

// berlinDay prints the calendar day of an instant in Berlin.
func berlinDay(at time.Time) string {
	return germanDate(calendar.DateFromTime(at))
}
