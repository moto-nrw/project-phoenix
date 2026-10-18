// Package application holds the Export Transfer workflow: resolve the
// counterpart, send the finished file, record the attempt.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

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
	targets  ports.TargetResolver
	uploader ports.Uploader
	journal  ports.Journal
	runtime  ports.CommandRuntime
	logger   *slog.Logger
}

func New(targets ports.TargetResolver, uploader ports.Uploader, journal ports.Journal, runtime ports.CommandRuntime, logger *slog.Logger) *Service {
	return &Service{targets: targets, uploader: uploader, journal: journal, runtime: runtime, logger: logger}
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
			return domain.Result{Filename: request.Filename, Reason: domain.ReasonNotConfigured}, nil
		}
		return domain.Result{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if !s.runtime.Active(ctx) {
		return domain.Result{}, fmt.Errorf("%w: transaction lifecycle is required", ErrUnavailable)
	}
	// The command, not the response middleware, owns the commit boundary.
	detached := s.runtime.Detached(ctx)
	entry := domain.JournalEntry{
		ActorAccountID: request.ActorAccountID, ActorName: request.ActorName,
		Kind: request.Kind, Format: request.Format, Filename: request.Filename,
		ByteSize: int64(len(request.Data)), TargetHost: target.Host, TargetPort: target.Port,
		TargetDirectory: target.RemoteDirectory, Reason: domain.ReasonInternal,
	}
	var id int64
	err = s.runtime.WithinTenant(detached, func(txCtx context.Context) error {
		var err error
		id, err = s.journal.Begin(txCtx, entry)
		return err
	})
	if err != nil {
		return domain.Result{}, fmt.Errorf("%w: record intent: %w", ErrUnavailable, err)
	}
	// Bound the whole dispatch from before its claim commit. A paused caller
	// cannot resume past the two-minute fence and start a fresh upload.
	dispatchCtx, stopDispatch := context.WithTimeout(detached, time.Minute)
	defer stopDispatch()
	var token string
	if err := s.runtime.WithinTenant(dispatchCtx, func(txCtx context.Context) error {
		var err error
		token, err = s.journal.Claim(txCtx, id)
		return err
	}); err != nil {
		return domain.Result{}, fmt.Errorf("%w: claim intent: %w", ErrUnavailable, err)
	}
	// No remote operation can happen until both commits above have succeeded.
	commit, _, uploadErr := s.uploader.Prepare(dispatchCtx, target, request.Filename, request.Data)
	if uploadErr == nil {
		// Prepare has already atomically replaced the file. Commit only discards
		// its rollback copy; a cleanup failure must not misreport a failed upload.
		if err := commit(); err != nil {
			s.log().Error("failed to finalize export transfer", "error", err.Error())
		}
	}
	result := domain.Result{Success: uploadErr == nil, Filename: request.Filename,
		ByteSize: entry.ByteSize, TargetHost: target.Host, TargetDirectory: target.RemoteDirectory}
	entry.Success = result.Success
	entry.Reason = ""
	if uploadErr != nil {
		result.Reason = failureReason(uploadErr)
		entry.Reason = result.Reason
		s.logTransferFailure(request, result, uploadErr)
	}
	// A cancelled request must still persist the transport result. A crash or
	// unavailable database leaves the committed attempt for unknown settlement.
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(detached), 5*time.Second)
	defer cancel()
	if err := s.runtime.WithinTenant(finalizeCtx, func(txCtx context.Context) error {
		return s.journal.Complete(txCtx, id, token, entry)
	}); err != nil {
		return domain.Result{}, fmt.Errorf("%w: record outcome: %w", ErrUnavailable, err)
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

func (s *Service) logTransferFailure(request domain.Request, result domain.Result, err error) {
	s.log().Warn("export transfer failed",
		"export_kind", request.Kind,
		"format", request.Format,
		"target_host", result.TargetHost,
		"reason", result.Reason,
		"error", err.Error(),
	)
}
