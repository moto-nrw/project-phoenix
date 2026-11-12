package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/exporttransfer"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/ports"
)

// Transfer hands one finance export of a Träger to the recipient of its
// format (ADR 0218 section 5, ADR 0221). Each step commits in its own Träger transaction, apart from
// any request transaction:
//
//  1. intent: resolve the recipient, let the producer make and record the
//     file, begin the journal entry;
//  2. fence: claim the one-shot dispatch lease;
//  3. send, only after both commits;
//  4. complete the journal and let the producer settle its record in the
//     same transaction.
//
// Nothing retries: a failed or unknown handover is a person's next action.
func (s *TraegerService) Transfer(ctx context.Context, request exporttransfer.TraegerTransferRequest) (exporttransfer.Outcome, error) {
	if request.OrganizationID <= 0 || request.PrepareFile == nil {
		return exporttransfer.Outcome{}, fmt.Errorf("%w: a Träger and a file producer are required", ErrUnavailable)
	}
	detached := s.runtime.Detached(ctx)
	intent, err := s.beginTraegerIntent(detached, request)
	if errors.Is(err, errRecipientNotReady) {
		return exporttransfer.Outcome{Reason: domain.ReasonNotConfigured}, nil
	}
	if err != nil {
		// A producer refusal keeps its identity for the caller.
		return exporttransfer.Outcome{}, err
	}
	// Bound the whole dispatch from before its claim commit, as the school
	// scope does: a paused caller cannot resume past the fence.
	dispatchCtx, stopDispatch := context.WithTimeout(detached, time.Minute)
	defer stopDispatch()
	var token string
	if err := s.within(dispatchCtx, request.OrganizationID, func(txCtx context.Context) error {
		var err error
		token, err = s.journal.Claim(txCtx, request.OrganizationID, intent.id)
		return err
	}); err != nil {
		return exporttransfer.Outcome{}, fmt.Errorf("%w: claim intent: %w", ErrUnavailable, err)
	}
	outcome, uncertain := s.send(dispatchCtx, request, intent)
	if uncertain {
		// The file may have arrived: the journal stays dispatching until
		// recovery records it as unknown, and the producer keeps its handover
		// pending, so no retry goes out without a person's repeat.
		return exporttransfer.Outcome{}, fmt.Errorf("%w: the counterpart did not answer the rename", exporttransfer.ErrUnknownSettlement)
	}
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(detached), 5*time.Second)
	defer cancel()
	if err := s.within(finalizeCtx, request.OrganizationID, func(txCtx context.Context) error {
		if err := s.journal.Complete(txCtx, request.OrganizationID, intent.id, token, outcome.Transferred, outcome.Reason); err != nil {
			return err
		}
		if request.Settle == nil {
			return nil
		}
		return request.Settle(txCtx, outcome)
	}); err != nil {
		return exporttransfer.Outcome{}, fmt.Errorf("%w: record outcome: %w", exporttransfer.ErrUnknownSettlement, err)
	}
	return outcome, nil
}

// traegerIntent is a committed intent: the journal row, the file and where
// it goes.
type traegerIntent struct {
	id       int64
	target   domain.Target
	filename string
	data     []byte
}

func (s *TraegerService) beginTraegerIntent(ctx context.Context, request exporttransfer.TraegerTransferRequest) (traegerIntent, error) {
	var intent traegerIntent
	err := s.within(ctx, request.OrganizationID, func(txCtx context.Context) error {
		recipient, target, err := s.resolveTarget(txCtx, request.OrganizationID, request.Format)
		if err != nil {
			return err
		}
		filename, data, err := request.PrepareFile(txCtx, exporttransfer.Destination{RecipientID: recipient.ID, RecipientName: recipient.Name,
			Host: target.Host, Directory: target.RemoteDirectory})
		if err != nil {
			return err
		}
		id, err := s.journal.Begin(txCtx, domain.TraegerJournalEntry{
			OrganizationID: request.OrganizationID, RecipientID: recipient.ID, RecipientName: recipient.Name,
			ActorAccountID: request.ActorAccountID,
			Kind:           request.Kind, Format: request.Format, Filename: filename, ByteSize: int64(len(data)),
			TargetHost: target.Host, TargetPort: target.Port, TargetDirectory: target.RemoteDirectory,
			Reason: domain.ReasonInternal,
		})
		if err != nil {
			return fmt.Errorf("%w: record intent: %w", ErrUnavailable, err)
		}
		intent = traegerIntent{id: id, target: target, filename: filename, data: data}
		return nil
	})
	return intent, err
}

// send uploads after both commits; a failure is an outcome, not an error.
// uncertain reports a failure after which the file may still have arrived.
func (s *TraegerService) send(ctx context.Context, request exporttransfer.TraegerTransferRequest, intent traegerIntent) (exporttransfer.Outcome, bool) {
	commit, _, err := s.uploader.Prepare(ctx, intent.target, intent.filename, intent.data)
	if err == nil {
		// The file was replaced atomically; commit only drops the rollback copy.
		if commitErr := commit(); commitErr != nil {
			s.log().Error("failed to finalize Träger export transfer",
				"organization_id", request.OrganizationID,
				"error", commitErr.Error(),
			)
		}
	}
	outcome := exporttransfer.Outcome{Transferred: err == nil, Filename: intent.filename, ByteSize: int64(len(intent.data)),
		TargetHost: intent.target.Host, TargetDirectory: intent.target.RemoteDirectory}
	if err != nil {
		outcome.Reason = failureReason(err)
		s.log().Warn("Träger export transfer failed",
			"organization_id", request.OrganizationID,
			"format", request.Format,
			"target_host", intent.target.Host,
			"reason", outcome.Reason,
			"error", err.Error(),
		)
	}
	var uncertain ports.UncertainError
	return outcome, errors.As(err, &uncertain) && uncertain.OutcomeUncertain()
}

// RecipientRekey re-seals the recipient passwords of one Träger on the
// active bank key (ADR 0183, ADR 0218 section 3, ADR 0221). It runs in the
// Träger transaction the maintenance command opens.
type RecipientRekey struct {
	Recipients ports.RecipientStore
	Rewrapper  ports.PasswordRewrapper
}

// Batch handles the recipients of the Träger after cursor, by id; an empty
// batch ends the source.
func (r RecipientRekey) Batch(ctx context.Context, organizationID, cursor int64, batchSize int) (exporttransfer.RekeyResult, error) {
	if organizationID <= 0 {
		return exporttransfer.RekeyResult{}, nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	recipients, err := r.Recipients.LockRecipientsAfter(ctx, organizationID, cursor, batchSize)
	if err != nil {
		return exporttransfer.RekeyResult{}, err
	}
	var result exporttransfer.RekeyResult
	for _, recipient := range recipients {
		result.Scanned++
		result.LastID = recipient.ID
		if len(recipient.SealedPassword) == 0 {
			continue
		}
		rewrapped, err := r.Rewrapper.Rewrap(recipient.SealedPassword, domain.PasswordContext(organizationID))
		if err != nil {
			return exporttransfer.RekeyResult{}, errors.New("export transfer rekey: a recipient password does not open")
		}
		if string(rewrapped) == string(recipient.SealedPassword) {
			continue
		}
		if err := r.Recipients.ReplaceSealedPassword(ctx, organizationID, recipient.ID, rewrapped); err != nil {
			return exporttransfer.RekeyResult{}, err
		}
		result.Changed++
	}
	return result, nil
}
