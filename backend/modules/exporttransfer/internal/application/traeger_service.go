package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-ogs/project-phoenix/modules/exporttransfer"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/ports"
)

// TraegerService is the Träger scope of Export Transfer (ADR 0218, ADR
// 0221). It shares the transport and the fence protocol of the school scope
// but reads only the recipients of the Träger and writes only the Träger
// journal.
type TraegerService struct {
	recipients ports.RecipientStore
	journal    ports.TraegerJournal
	uploader   ports.Uploader
	cipher     ports.PasswordCipher
	runtime    ports.TraegerRuntime
	logger     *slog.Logger
}

var _ exporttransfer.TraegerTransfers = (*TraegerService)(nil)

// errRecipientNotReady ends the intent transaction before anything is made.
var errRecipientNotReady = errors.New("export transfer: the format has no ready recipient of the Träger")

// maxJournalEntries bounds one read of the journal.
const maxJournalEntries = 50

func NewTraegerService(recipients ports.RecipientStore, journal ports.TraegerJournal, uploader ports.Uploader,
	cipher ports.PasswordCipher, runtime ports.TraegerRuntime, logger *slog.Logger,
) *TraegerService {
	return &TraegerService{recipients: recipients, journal: journal, uploader: uploader, cipher: cipher, runtime: runtime, logger: logger}
}

func (s *TraegerService) log() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

// within runs fn in the Träger transaction of organizationID: the only
// boundary of the scope.
func (s *TraegerService) within(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	if organizationID <= 0 {
		return fmt.Errorf("%w: a Träger is required", ErrUnavailable)
	}
	return s.runtime.WithinTraeger(ctx, organizationID, fn)
}

// openPassword opens the stored password. A password that no longer opens
// (its key left the ring) counts as missing: the Träger enters it again.
func (s *TraegerService) openPassword(recipient domain.StoredRecipient) (string, bool) {
	if len(recipient.SealedPassword) == 0 {
		return "", false
	}
	plain, err := s.cipher.Open(recipient.SealedPassword, domain.PasswordContext(recipient.OrganizationID))
	if err != nil {
		s.log().Warn("export transfer recipient password does not open",
			"organization_id", recipient.OrganizationID,
			"recipient_id", recipient.ID,
		)
		return "", false
	}
	return string(plain), true
}

func (s *TraegerService) view(recipient domain.StoredRecipient) exporttransfer.TraegerRecipient {
	_, opens := s.openPassword(recipient)
	missing := recipient.MissingFields(opens)
	view := exporttransfer.TraegerRecipient{
		ID: recipient.ID, Name: recipient.Name, Enabled: recipient.Enabled, Ready: recipient.Enabled && len(missing) == 0,
		Host: recipient.Host, Port: recipient.Port, Username: recipient.Username,
		RemoteDirectory: recipient.RemoteDirectory, HostKeyFingerprint: recipient.HostKeyFingerprint,
		PasswordSet: opens, MissingFields: missing, Formats: recipient.Formats, UpdatedAt: recipient.UpdatedAt,
	}
	if view.Formats == nil {
		view.Formats = []string{}
	}
	if recipient.LastHandover != nil {
		last := journalEntry(*recipient.LastHandover)
		view.LastHandover = &last
	}
	return view
}

func journalLimitOf(limit int) int {
	if limit <= 0 || limit > maxJournalEntries {
		return maxJournalEntries
	}
	return limit
}

// Recipients answers every recipient, the recipient of each format and the
// newest handovers in one Träger transaction.
func (s *TraegerService) Recipients(ctx context.Context, organizationID int64, journalLimit int) (exporttransfer.TraegerRecipientsPage, error) {
	var page exporttransfer.TraegerRecipientsPage
	err := s.within(ctx, organizationID, func(txCtx context.Context) error {
		var err error
		page, err = s.recipientsPage(txCtx, organizationID, journalLimitOf(journalLimit))
		return err
	})
	return page, err
}

func (s *TraegerService) recipientsPage(ctx context.Context, organizationID int64, journalLimit int) (exporttransfer.TraegerRecipientsPage, error) {
	recipients, err := s.recipients.Recipients(ctx, organizationID)
	if err != nil {
		return exporttransfer.TraegerRecipientsPage{}, err
	}
	rows, err := s.journal.List(ctx, organizationID, 0, journalLimit)
	if err != nil {
		return exporttransfer.TraegerRecipientsPage{}, err
	}
	page := exporttransfer.TraegerRecipientsPage{Recipients: make([]exporttransfer.TraegerRecipient, 0, len(recipients)),
		Assignments: make([]exporttransfer.TraegerAssignment, 0, len(domain.Formats)), Handovers: journalEntries(rows)}
	receiver := map[string]int64{}
	for _, recipient := range recipients {
		page.Recipients = append(page.Recipients, s.view(recipient))
		for _, format := range recipient.Formats {
			receiver[format] = recipient.ID
		}
	}
	for _, format := range domain.Formats {
		assignment := exporttransfer.TraegerAssignment{Format: format}
		if id, ok := receiver[format]; ok {
			assignment.RecipientID = &id
		}
		page.Assignments = append(page.Assignments, assignment)
	}
	return page, nil
}

// Recipient answers one recipient of the Träger with its own handovers.
func (s *TraegerService) Recipient(ctx context.Context, organizationID, recipientID int64, journalLimit int,
) (exporttransfer.TraegerRecipientPage, error) {
	var page exporttransfer.TraegerRecipientPage
	err := s.within(ctx, organizationID, func(txCtx context.Context) error {
		recipient, found, err := s.recipients.Recipient(txCtx, organizationID, recipientID)
		if err != nil {
			return err
		}
		if !found {
			return exporttransfer.ErrRecipientNotFound
		}
		rows, err := s.journal.List(txCtx, organizationID, recipientID, journalLimitOf(journalLimit))
		if err != nil {
			return err
		}
		// Its newest handover is the first of its own journal.
		if len(rows) > 0 {
			recipient.LastHandover = &rows[0]
		}
		page = exporttransfer.TraegerRecipientPage{Recipient: s.view(recipient), Handovers: journalEntries(rows)}
		return nil
	})
	return page, err
}

// validValues normalizes and checks what the Träger entered.
func validValues(input exporttransfer.TraegerRecipientInput) (domain.RecipientValues, error) {
	values := domain.RecipientValues(input).Normalize()
	if invalid := values.InvalidFields(); len(invalid) > 0 {
		return values, &exporttransfer.RecipientError{Fields: invalid}
	}
	return values, nil
}

func storedOf(organizationID, recipientID int64, values domain.RecipientValues) domain.StoredRecipient {
	return domain.StoredRecipient{ID: recipientID, OrganizationID: organizationID, Name: values.Name, Enabled: values.Enabled,
		Host: values.Host, Port: values.Port, Username: values.Username, RemoteDirectory: values.RemoteDirectory,
		HostKeyFingerprint: values.HostKeyFingerprint}
}

// nameTaken keeps the identity of a taken name for the caller.
func nameTaken(err error) error {
	if errors.Is(err, domain.ErrNameTaken) {
		return exporttransfer.ErrRecipientNameTaken
	}
	return err
}

// CreateRecipient adds a recipient to the Träger. It receives no format
// until the Träger assigns one.
func (s *TraegerService) CreateRecipient(ctx context.Context, organizationID, actorAccountID int64, input exporttransfer.TraegerRecipientInput,
) (exporttransfer.TraegerRecipient, error) {
	values, err := validValues(input)
	if err != nil {
		return exporttransfer.TraegerRecipient{}, err
	}
	var view exporttransfer.TraegerRecipient
	err = s.within(ctx, organizationID, func(txCtx context.Context) error {
		next := storedOf(organizationID, 0, values)
		if next.SealedPassword, err = s.sealedPassword(domain.StoredRecipient{}, false, next, values.Password); err != nil {
			return err
		}
		id, err := s.recipients.InsertRecipient(txCtx, next, actorAccountID)
		if err != nil {
			return nameTaken(err)
		}
		saved, _, err := s.recipients.Recipient(txCtx, organizationID, id)
		view = s.view(saved)
		return err
	})
	return view, err
}

// SaveRecipient checks and stores one recipient. The password is sealed
// before it reaches the store. A stored password never follows a changed
// destination: changing address, port, user or fingerprint requires it
// again, so nobody can send the saved secret to a server of their own.
func (s *TraegerService) SaveRecipient(ctx context.Context, organizationID, actorAccountID, recipientID int64, input exporttransfer.TraegerRecipientInput,
) (exporttransfer.TraegerRecipient, error) {
	values, err := validValues(input)
	if err != nil {
		return exporttransfer.TraegerRecipient{}, err
	}
	var view exporttransfer.TraegerRecipient
	err = s.within(ctx, organizationID, func(txCtx context.Context) error {
		existing, found, err := s.recipients.LockRecipient(txCtx, organizationID, recipientID)
		if err != nil {
			return err
		}
		if !found {
			return exporttransfer.ErrRecipientNotFound
		}
		next := storedOf(organizationID, recipientID, values)
		if next.SealedPassword, err = s.sealedPassword(existing, found, next, values.Password); err != nil {
			return err
		}
		if _, err := s.recipients.UpdateRecipient(txCtx, next, actorAccountID); err != nil {
			return nameTaken(err)
		}
		saved, _, err := s.recipients.Recipient(txCtx, organizationID, recipientID)
		view = s.view(saved)
		return err
	})
	return view, err
}

// sealedPassword seals a new password, or keeps the stored one when the
// destination it was entered for stays the same.
func (s *TraegerService) sealedPassword(existing domain.StoredRecipient, found bool, next domain.StoredRecipient, password *string) ([]byte, error) {
	if password != nil {
		sealed, err := s.cipher.Seal([]byte(*password), domain.PasswordContext(next.OrganizationID))
		if err != nil {
			return nil, fmt.Errorf("%w: seal the recipient password", ErrUnavailable)
		}
		return sealed, nil
	}
	if !found || len(existing.SealedPassword) == 0 {
		return nil, nil
	}
	if existing.Host != next.Host || existing.Port != next.Port || existing.Username != next.Username ||
		existing.HostKeyFingerprint != next.HostKeyFingerprint {
		return nil, &exporttransfer.RecipientError{Fields: []string{domain.FieldPassword}}
	}
	return existing.SealedPassword, nil
}

// DeleteRecipient removes one recipient and its sealed password. Its
// formats go back to the download; its handovers stay in the journal.
func (s *TraegerService) DeleteRecipient(ctx context.Context, organizationID, recipientID int64) (exporttransfer.TraegerRecipientsPage, error) {
	var page exporttransfer.TraegerRecipientsPage
	err := s.within(ctx, organizationID, func(txCtx context.Context) error {
		deleted, err := s.recipients.DeleteRecipient(txCtx, organizationID, recipientID)
		if err != nil {
			return err
		}
		if !deleted {
			return exporttransfer.ErrRecipientNotFound
		}
		page, err = s.recipientsPage(txCtx, organizationID, maxJournalEntries)
		return err
	})
	return page, err
}

// validAssignments requires known formats, each named once, and positive
// recipient ids.
func validAssignments(assignments []exporttransfer.TraegerAssignment) bool {
	seen := map[string]bool{}
	for _, assignment := range assignments {
		if !domain.KnownFormat(assignment.Format) || seen[assignment.Format] ||
			(assignment.RecipientID != nil && *assignment.RecipientID <= 0) {
			return false
		}
		seen[assignment.Format] = true
	}
	return true
}

// AssignFormats sends each named format to its recipient or to the
// download, all or none, and answers the page afterwards.
func (s *TraegerService) AssignFormats(ctx context.Context, organizationID, actorAccountID int64, assignments []exporttransfer.TraegerAssignment,
) (exporttransfer.TraegerRecipientsPage, error) {
	if !validAssignments(assignments) {
		return exporttransfer.TraegerRecipientsPage{}, exporttransfer.ErrAssignmentInvalid
	}
	var page exporttransfer.TraegerRecipientsPage
	err := s.within(ctx, organizationID, func(txCtx context.Context) error {
		for _, assignment := range assignments {
			var recipientID int64
			if assignment.RecipientID != nil {
				recipientID = *assignment.RecipientID
			}
			assigned, err := s.recipients.AssignFormat(txCtx, organizationID, assignment.Format, recipientID, actorAccountID)
			if err != nil {
				return err
			}
			if !assigned {
				return exporttransfer.ErrRecipientNotFound
			}
		}
		var err error
		page, err = s.recipientsPage(txCtx, organizationID, maxJournalEntries)
		return err
	})
	return page, err
}

func journalEntries(rows []domain.TraegerJournalRow) []exporttransfer.TraegerTransferEntry {
	entries := make([]exporttransfer.TraegerTransferEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, journalEntry(row))
	}
	return entries
}

func journalEntry(row domain.TraegerJournalRow) exporttransfer.TraegerTransferEntry {
	return exporttransfer.TraegerTransferEntry{ID: row.ID, CreatedAt: row.CreatedAt, Format: row.Format,
		Filename: row.Filename, ByteSize: row.ByteSize, TargetHost: row.TargetHost, TargetDirectory: row.TargetDirectory,
		RecipientID: row.RecipientID, RecipientName: row.RecipientName, State: journalState(row), Reason: row.Reason}
}

// journalState names a row: settled, still within its fence, or unknown.
func journalState(row domain.TraegerJournalRow) string {
	switch {
	case row.DispatchState == "terminal" && row.Status == "success":
		return exporttransfer.TraegerTransferred
	case row.DispatchState == "terminal":
		return exporttransfer.TraegerFailed
	case row.DispatchState == "unknown" || row.DispatchExpired:
		return exporttransfer.TraegerUnknown
	default:
		return exporttransfer.TraegerInProgress
	}
}

// resolveTarget is the complete target of the ready recipient of a format.
func (s *TraegerService) resolveTarget(ctx context.Context, organizationID int64, format string) (domain.StoredRecipient, domain.Target, error) {
	recipient, found, err := s.recipients.RecipientForFormat(ctx, organizationID, format)
	if err != nil {
		return domain.StoredRecipient{}, domain.Target{}, err
	}
	if !found || !recipient.Enabled {
		return domain.StoredRecipient{}, domain.Target{}, errRecipientNotReady
	}
	password, opens := s.openPassword(recipient)
	if len(recipient.MissingFields(opens)) > 0 {
		return domain.StoredRecipient{}, domain.Target{}, errRecipientNotReady
	}
	return recipient, domain.Target{Host: recipient.Host, Port: recipient.Port, Username: recipient.Username, Password: password,
		RemoteDirectory: recipient.RemoteDirectory, HostKeyFingerprint: recipient.HostKeyFingerprint}, nil
}
