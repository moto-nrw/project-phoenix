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

// TraegerService is the Träger scope of Export Transfer (ADR 0218). It
// shares the transport and the fence protocol of the school scope but reads
// only the recipient of the Träger and writes only the Träger journal.
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
var errRecipientNotReady = errors.New("export transfer: the recipient of the Träger is not ready")

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
		s.log().Warn("export transfer recipient password does not open", "organization_id", recipient.OrganizationID)
		return "", false
	}
	return string(plain), true
}

func (s *TraegerService) view(recipient domain.StoredRecipient, found bool) exporttransfer.TraegerRecipient {
	if !found {
		return exporttransfer.TraegerRecipient{Port: domain.DefaultPort, MissingFields: domain.StoredRecipient{Port: domain.DefaultPort}.MissingFields(false)}
	}
	_, opens := s.openPassword(recipient)
	missing := recipient.MissingFields(opens)
	updated := recipient.UpdatedAt
	return exporttransfer.TraegerRecipient{
		Configured: true, Enabled: recipient.Enabled, Ready: recipient.Enabled && len(missing) == 0,
		Host: recipient.Host, Port: recipient.Port, Username: recipient.Username,
		RemoteDirectory: recipient.RemoteDirectory, HostKeyFingerprint: recipient.HostKeyFingerprint,
		PasswordSet: opens, MissingFields: missing, UpdatedAt: &updated,
	}
}

// Recipient answers the credential-free view of the recipient and its
// newest handovers in one Träger transaction.
func (s *TraegerService) Recipient(ctx context.Context, organizationID int64, journalLimit int) (exporttransfer.TraegerRecipientPage, error) {
	if journalLimit <= 0 || journalLimit > maxJournalEntries {
		journalLimit = maxJournalEntries
	}
	var page exporttransfer.TraegerRecipientPage
	err := s.within(ctx, organizationID, func(txCtx context.Context) error {
		recipient, found, err := s.recipients.Recipient(txCtx, organizationID)
		if err != nil {
			return err
		}
		rows, err := s.journal.List(txCtx, organizationID, journalLimit)
		page = exporttransfer.TraegerRecipientPage{Recipient: s.view(recipient, found), Handovers: journalEntries(rows)}
		return err
	})
	return page, err
}

// SaveRecipient checks and stores the recipient. The password is sealed
// before it reaches the store. A stored password never follows a changed
// destination: changing address, port, user or fingerprint requires it
// again, so nobody can send the saved secret to a server of their own.
func (s *TraegerService) SaveRecipient(ctx context.Context, organizationID, actorAccountID int64, input exporttransfer.TraegerRecipientInput,
) (exporttransfer.TraegerRecipient, error) {
	values := domain.RecipientValues(input).Normalize()
	if invalid := values.InvalidFields(); len(invalid) > 0 {
		return exporttransfer.TraegerRecipient{}, &exporttransfer.RecipientError{Fields: invalid}
	}
	var view exporttransfer.TraegerRecipient
	err := s.within(ctx, organizationID, func(txCtx context.Context) error {
		existing, found, err := s.recipients.LockRecipient(txCtx, organizationID)
		if err != nil {
			return err
		}
		next := domain.StoredRecipient{OrganizationID: organizationID, Enabled: values.Enabled, Host: values.Host, Port: values.Port,
			Username: values.Username, RemoteDirectory: values.RemoteDirectory, HostKeyFingerprint: values.HostKeyFingerprint}
		if next.SealedPassword, err = s.sealedPassword(existing, found, next, values.Password); err != nil {
			return err
		}
		if err := s.recipients.SaveRecipient(txCtx, next, actorAccountID); err != nil {
			return err
		}
		saved, found, err := s.recipients.Recipient(txCtx, organizationID)
		view = s.view(saved, found)
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

// DeleteRecipient removes the recipient and its sealed password.
func (s *TraegerService) DeleteRecipient(ctx context.Context, organizationID int64) (exporttransfer.TraegerRecipient, error) {
	var view exporttransfer.TraegerRecipient
	err := s.within(ctx, organizationID, func(txCtx context.Context) error {
		if err := s.recipients.DeleteRecipient(txCtx, organizationID); err != nil {
			return err
		}
		recipient, found, err := s.recipients.Recipient(txCtx, organizationID)
		view = s.view(recipient, found)
		return err
	})
	return view, err
}

func journalEntries(rows []domain.TraegerJournalRow) []exporttransfer.TraegerTransferEntry {
	entries := make([]exporttransfer.TraegerTransferEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, exporttransfer.TraegerTransferEntry{ID: row.ID, CreatedAt: row.CreatedAt, Format: row.Format,
			Filename: row.Filename, ByteSize: row.ByteSize, TargetHost: row.TargetHost, TargetDirectory: row.TargetDirectory,
			State: journalState(row), Reason: row.Reason})
	}
	return entries
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

// resolveTarget is the complete target of a ready recipient.
func (s *TraegerService) resolveTarget(ctx context.Context, organizationID int64) (domain.Target, error) {
	recipient, found, err := s.recipients.Recipient(ctx, organizationID)
	if err != nil {
		return domain.Target{}, err
	}
	if !found || !recipient.Enabled {
		return domain.Target{}, errRecipientNotReady
	}
	password, opens := s.openPassword(recipient)
	if len(recipient.MissingFields(opens)) > 0 {
		return domain.Target{}, errRecipientNotReady
	}
	return domain.Target{Host: recipient.Host, Port: recipient.Port, Username: recipient.Username, Password: password,
		RemoteDirectory: recipient.RemoteDirectory, HostKeyFingerprint: recipient.HostKeyFingerprint}, nil
}
