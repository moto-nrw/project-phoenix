// Package application holds the Export Transfer workflow: resolve the
// counterpart, send the finished file, record the attempt.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/ports"
)

// ErrUnavailable marks a transfer that could not be attempted for a reason
// that is NOT the school's configuration — a settings store that will not
// answer, a journal that will not accept the entry.
var ErrUnavailable = errors.New("export transfer unavailable")

// Service performs one transfer at a time. It has no queue and no retry: the
// feature is a manual action, and a silent background retry of a payroll file
// is exactly the behaviour nobody asked for.
type Service struct {
	targets   ports.TargetResolver
	uploader  ports.Uploader
	journal   ports.Journal
	lifecycle ports.TransactionLifecycle
	logger    *slog.Logger
}

func New(targets ports.TargetResolver, uploader ports.Uploader, journal ports.Journal, lifecycle ports.TransactionLifecycle, logger *slog.Logger) *Service {
	return &Service{targets: targets, uploader: uploader, journal: journal, lifecycle: lifecycle, logger: logger}
}

func (s *Service) log() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

func (s *Service) State(ctx context.Context) (domain.TargetState, error) {
	state, err := s.targets.State(ctx)
	if err != nil {
		return domain.TargetState{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return state, nil
}

func (s *Service) Transfer(ctx context.Context, request domain.Request) (domain.Result, error) {
	target, err := s.targets.Resolve(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrNotConfigured) {
			// No journal entry: nothing was attempted, nothing left the
			// school, and there is no destination to record. The trail
			// answers "which files went where" — a form that was never
			// filled in is not an answer to that question.
			return domain.Result{Filename: request.Filename, Reason: domain.ReasonNotConfigured}, nil
		}
		return domain.Result{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if !s.lifecycle.Active(ctx) {
		return domain.Result{}, fmt.Errorf("%w: transaction lifecycle is required", ErrUnavailable)
	}

	commitUpload, rollbackUpload, uploadErr := s.uploader.Prepare(ctx, target, request.Filename, request.Data)
	if uploadErr == nil {
		// The remote replacement stays reversible until the surrounding audit
		// transaction has a final outcome. A failed insert, handler rollback or
		// commit failure restores the prior file; a commit only discards the
		// rollback copy.
		s.lifecycle.AfterCommit(ctx, func() {
			if err := commitUpload(); err != nil {
				s.log().Error("failed to finalize export transfer", "error", err.Error())
			}
		})
		s.lifecycle.AfterRollback(ctx, func() {
			if err := rollbackUpload(); err != nil {
				s.log().Error("failed to roll back export transfer", "error", err.Error())
			}
		})
	}

	result := domain.Result{
		Success:         uploadErr == nil,
		Filename:        request.Filename,
		ByteSize:        int64(len(request.Data)),
		TargetHost:      target.Host,
		TargetDirectory: target.RemoteDirectory,
	}
	if uploadErr != nil {
		result.Reason = failureReason(uploadErr)
		// The transport's own text stays in the log and goes nowhere near the
		// response or the journal.
		s.log().Warn("export transfer failed",
			"export_kind", request.Kind,
			"format", request.Format,
			"target_host", target.Host,
			"reason", result.Reason,
			"error", uploadErr.Error(),
		)
	}

	entry := domain.JournalEntry{
		ActorAccountID:  request.ActorAccountID,
		ActorName:       request.ActorName,
		Kind:            request.Kind,
		Format:          request.Format,
		Filename:        result.Filename,
		ByteSize:        result.ByteSize,
		TargetHost:      target.Host,
		TargetPort:      target.Port,
		TargetDirectory: target.RemoteDirectory,
		Success:         result.Success,
		Reason:          result.Reason,
	}
	if err := s.journal.Record(ctx, entry); err != nil {
		// Returning an error makes the surrounding transaction roll back;
		// its rollback hook restores the previous remote state, so an
		// unrecorded transfer is never reported as a success.
		return domain.Result{}, fmt.Errorf("%w: record attempt: %w", ErrUnavailable, err)
	}
	return result, nil
}

// failureReason maps a transport error onto one stable reason code. An error
// that names no reason is an internal one: an unrecognized failure must never
// be presented as a known, harmless condition.
func failureReason(err error) string {
	var reasoned ports.ReasonedError
	if errors.As(err, &reasoned) {
		return reasoned.TransferReason()
	}
	return domain.ReasonInternal
}
