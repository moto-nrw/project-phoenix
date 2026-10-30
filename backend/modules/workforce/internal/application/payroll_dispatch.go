package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
)

// Export ans Lohnbüro (#4805). The month close queues a dispatch in its own
// transaction; RunDue carries it out after the commit. Every step that
// writes commits on its own, fenced by the claim's lease, so a crashed worker
// leaves a row the next pass picks up again instead of a lost file.

const (
	payrollDispatchLease      = 5 * time.Minute
	payrollDispatchBatch      = 10
	payrollMailPollInterval   = time.Minute
	payrollDispatchRetainDays = 400
	// payrollExportActorRole names the automatic send in the export's access
	// audit; the account is the person who closed the month.
	payrollExportActorRole = "payroll_dispatch"
)

// payrollRetryBackoff is the wait before the second, third and fourth
// attempt of a failed SFTP send.
var payrollRetryBackoff = []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour}

// PayrollDispatchConfig is the Lohnbüro of the school.
type PayrollDispatchConfig struct {
	Auto    bool
	Channel string
	Format  string
	Email   string
}

// PayrollDispatchSettings answers the Lohnbüro settings of the tenant in the
// context. The root binds it to the Settings Platform.
type PayrollDispatchSettings interface {
	PayrollDispatchConfig(ctx context.Context) (PayrollDispatchConfig, error)
}

// PayrollTransferRequest is one SFTP send of the month file. Prepare builds
// the file in the transfer's own transaction, with its access audit.
type PayrollTransferRequest struct {
	Format         string
	Period         string
	Prepare        func(context.Context) (filename string, data []byte, err error)
	ActorAccountID int64
	ActorName      string
}

// PayrollTransferOutcome is the result of one SFTP attempt.
type PayrollTransferOutcome struct {
	Transferred bool
	// NotConfigured: the counterpart is off or incomplete; nothing was sent
	// and nothing was journaled.
	NotConfigured bool
	Reason        string
}

// PayrollEmailResult is one settled e-mail send for the transfer journal.
type PayrollEmailResult struct {
	Format, Period, Filename string
	ByteSize                 int64
	Recipient                string
	Delivered                bool
	ActorAccountID           int64
	ActorName                string
}

// PayrollTransfer is the Export Transfer journal and its SFTP transport.
type PayrollTransfer interface {
	Transfer(context.Context, PayrollTransferRequest) (PayrollTransferOutcome, error)
	RecordEmail(context.Context, PayrollEmailResult) error
}

// PayrollExport builds the month file; it is Workforce's own time export.
type PayrollExport interface {
	ExportStaffTime(ctx context.Context, request workforce.TimeExportRequest, actorAccountID int64, actorRole string) (*workforce.ExportFile, error)
	DatevReport(ctx context.Context, request workforce.TimeExportRequest) (*workforce.DatevExportReport, error)
}

// PayrollTokens creates download link tokens and their fingerprints.
type PayrollTokens interface {
	NewToken() (raw, fingerprint string, err error)
	Fingerprint(raw string) string
}

// PayrollRuntime opens the transactions of the dispatch. Within runs fn in a
// fresh transaction of the tenant in the context, WithinTenant in one of the
// given tenant; Detached masks an ambient transaction so each step commits on
// its own.
type PayrollRuntime struct {
	TenantID     func(context.Context) (int64, error)
	Within       func(context.Context, func(context.Context) error) error
	WithinTenant func(context.Context, int64, func(context.Context) error) error
	Detached     func(context.Context) context.Context
}

// PayrollDispatchDependencies are the collaborators of the dispatch.
type PayrollDispatchDependencies struct {
	Store    ports.PayrollDispatchStore
	Settings PayrollDispatchSettings
	Export   PayrollExport
	Transfer PayrollTransfer
	Mail     PayrollMail
	Schools  PayrollSchools
	Managers PayrollManagers
	Tokens   PayrollTokens
	// FrontendURL is the base of the download link and the notice's link.
	FrontendURL string
	Runtime  PayrollRuntime
	Now      func() time.Time
	Logger   *slog.Logger
}

// PayrollDispatch carries the month file to the payroll office.
type PayrollDispatch struct {
	deps PayrollDispatchDependencies
}

// NewPayrollDispatch requires every collaborator: a missing transport is a
// configuration error, not a reason to drop a payroll file silently.
func NewPayrollDispatch(deps PayrollDispatchDependencies) (*PayrollDispatch, error) {
	runtime := deps.Runtime
	switch {
	case deps.Store == nil || deps.Settings == nil || deps.Export == nil:
		return nil, errors.New("payroll dispatch: store, settings and export are required")
	case deps.Transfer == nil || deps.Mail == nil || deps.Schools == nil || deps.Managers == nil || deps.Tokens == nil:
		return nil, errors.New("payroll dispatch: transfer, mail, schools, managers and tokens are required")
	case strings.TrimSpace(deps.FrontendURL) == "":
		return nil, errors.New("payroll dispatch: the frontend URL of the download link is required")
	case runtime.TenantID == nil || runtime.Within == nil || runtime.WithinTenant == nil || runtime.Detached == nil:
		return nil, errors.New("payroll dispatch: the transaction runtime is required")
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &PayrollDispatch{deps: deps}, nil
}

// QueueAfterMonthClose joins the close's transaction. A school without the
// automatic send queues nothing; a missing e-mail address is recorded as a
// blocked dispatch with its notice right away, so the leadership sees it.
func (s *PayrollDispatch) QueueAfterMonthClose(ctx context.Context, trigger workforce.PayrollDispatchTrigger) error {
	if trigger.ClosedStaff <= 0 {
		return nil
	}
	config, err := s.deps.Settings.PayrollDispatchConfig(ctx)
	if err != nil {
		return fmt.Errorf("payroll dispatch: read the Lohnbüro settings: %w", err)
	}
	if !config.Auto {
		return nil
	}
	if trigger.ActorAccountID <= 0 {
		return errors.New("payroll dispatch: the closing account is required")
	}
	if err := validPayrollConfig(config); err != nil {
		return err
	}
	sequence, err := s.deps.Store.NextPayrollDispatchSequence(ctx, trigger.Year, trigger.Month)
	if err != nil {
		return fmt.Errorf("payroll dispatch: number the dispatch: %w", err)
	}
	now := s.deps.Now()
	dispatch := domain.PayrollDispatch{
		Year: trigger.Year, Month: trigger.Month, Sequence: sequence,
		Channel: config.Channel, Format: config.Format, Recipient: strings.TrimSpace(config.Email),
		ActorAccountID: trigger.ActorAccountID, ActorName: trigger.ActorName,
		State: workforce.PayrollDispatchPending, NextAttemptAt: now,
	}
	if dispatch.Channel == workforce.PayrollChannelEmail && dispatch.Recipient == "" {
		dispatch.State, dispatch.Reason, dispatch.FinishedAt = workforce.PayrollDispatchBlocked, workforce.PayrollReasonEmailMissing, &now
	}
	if err := s.deps.Store.InsertPayrollDispatch(ctx, dispatch); err != nil {
		return fmt.Errorf("payroll dispatch: queue: %w", err)
	}
	if dispatch.State == workforce.PayrollDispatchBlocked {
		return s.notify(ctx, dispatch)
	}
	return nil
}

func validPayrollConfig(config PayrollDispatchConfig) error {
	switch config.Channel {
	case workforce.PayrollChannelSFTP, workforce.PayrollChannelEmail:
	default:
		return fmt.Errorf("payroll dispatch: unknown channel %q", config.Channel)
	}
	switch config.Format {
	case "csv", "xlsx", "datev_lodas", "datev_lug":
	default:
		return fmt.Errorf("payroll dispatch: unknown format %q", config.Format)
	}
	return nil
}

// Overview reads the newest dispatch in the caller's transaction.
func (s *PayrollDispatch) Overview(ctx context.Context) (workforce.PayrollDispatchOverview, error) {
	latest, err := s.deps.Store.LatestPayrollDispatch(ctx)
	if err != nil || latest == nil {
		return workforce.PayrollDispatchOverview{}, err
	}
	view := workforce.PayrollDispatchView{
		Year: latest.Year, Month: latest.Month, Correction: latest.Sequence > 0,
		Channel: latest.Channel, Format: latest.Format, Recipient: latest.Recipient,
		State: latest.State, Reason: latest.Reason, Attempts: latest.Attempts, Filename: latest.Filename,
		CreatedAt: latest.CreatedAt, FinishedAt: latest.FinishedAt,
		DownloadExpiresAt: latest.DownloadExpiresAt, DownloadedAt: latest.DownloadedAt,
	}
	return workforce.PayrollDispatchOverview{Latest: &view}, nil
}

// RunDue claims the due dispatches, works each in its own transactions and
// purges what the retention allows. One dispatch failing does not stop the
// others; the first error is reported after all ran.
func (s *PayrollDispatch) RunDue(ctx context.Context) (int, error) {
	ctx = s.deps.Runtime.Detached(ctx)
	now := s.deps.Now()
	var claims []domain.PayrollDispatch
	if err := s.deps.Runtime.Within(ctx, func(txCtx context.Context) error {
		var err error
		claims, err = s.deps.Store.ClaimPayrollDispatches(txCtx, now, payrollDispatchLease, payrollDispatchBatch)
		return err
	}); err != nil {
		return 0, fmt.Errorf("payroll dispatch: claim: %w", err)
	}
	finished := 0
	var firstErr error
	for _, claim := range claims {
		done, err := s.work(ctx, claim)
		if done {
			finished++
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	cutoff := now.Add(-payrollDispatchRetainDays * 24 * time.Hour)
	if err := s.deps.Runtime.Within(ctx, func(txCtx context.Context) error {
		return s.deps.Store.PurgePayrollDispatches(txCtx, now, cutoff)
	}); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("payroll dispatch: purge: %w", err)
	}
	return finished, firstErr
}

// work runs one claimed pass. A pass that fails for a reason of its own (the
// database, the export) counts as a failed attempt, so a broken month cannot
// keep the worker busy for ever.
func (s *PayrollDispatch) work(ctx context.Context, claim domain.PayrollDispatch) (bool, error) {
	var (
		done bool
		err  error
	)
	switch {
	case claim.State == workforce.PayrollDispatchMailing:
		done, err = s.settleMail(ctx, claim)
	case claim.Channel == workforce.PayrollChannelEmail:
		done, err = s.sendLink(ctx, claim)
	default:
		done, err = s.sendSFTP(ctx, claim)
	}
	if err == nil {
		return done, nil
	}
	s.deps.Logger.Error("payroll dispatch pass failed",
		"dispatch_id", claim.ID,
		"error", err.Error(),
	)
	if claim.State == workforce.PayrollDispatchMailing {
		// The mail is committed; only its settlement failed. The lease
		// expires and the next pass asks again.
		return false, err
	}
	failed, retryErr := s.failAttempt(ctx, claim, workforce.PayrollReasonInternal)
	if retryErr != nil {
		return false, errors.Join(err, retryErr)
	}
	return failed, err
}

// buildFile runs the DATEV check of the manual export before a DATEV file
// and builds the file under the closer's name. blocked names why nothing may
// be sent.
func (s *PayrollDispatch) buildFile(ctx context.Context, dispatch domain.PayrollDispatch) (file *workforce.ExportFile, blocked string, err error) {
	if blocked, err = s.datevCheck(ctx, dispatch); err != nil || blocked != "" {
		return nil, blocked, err
	}
	return s.exportFile(ctx, dispatch)
}

// exportFile builds the file with its access audit. The export refuses a
// DATEV file it cannot fill completely, which blocks the dispatch as well.
func (s *PayrollDispatch) exportFile(ctx context.Context, dispatch domain.PayrollDispatch) (*workforce.ExportFile, string, error) {
	request := workforce.TimeExportRequest{Year: dispatch.Year, Month: dispatch.Month, Format: dispatch.Format}
	file, err := s.deps.Export.ExportStaffTime(ctx, request, dispatch.ActorAccountID, payrollExportActorRole)
	if errors.Is(err, workforce.ErrPayrollConfigIncomplete) {
		return nil, workforce.PayrollReasonDatevIncomplete, nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("build the month file: %w", err)
	}
	file.Filename = payrollFilename(file.Filename, dispatch.Sequence)
	return file, "", nil
}

// payrollFilename marks a correction in the name, before the extension:
// datev_lodas_2026-09.txt becomes datev_lodas_2026-09_korrektur-1.txt.
func payrollFilename(filename string, sequence int) string {
	if sequence <= 0 {
		return filename
	}
	extension := path.Ext(filename)
	return fmt.Sprintf("%s_korrektur-%d%s", strings.TrimSuffix(filename, extension), sequence, extension)
}

// sendSFTP checks the DATEV configuration, then lets Export Transfer build the
// file with its audit and send it. Each attempt is one journal entry.
func (s *PayrollDispatch) sendSFTP(ctx context.Context, claim domain.PayrollDispatch) (bool, error) {
	var outcome PayrollTransferOutcome
	var blocked string
	err := s.deps.Runtime.Within(ctx, func(txCtx context.Context) error {
		var err error
		if blocked, err = s.datevCheck(txCtx, claim); err != nil || blocked != "" {
			return err
		}
		outcome, err = s.deps.Transfer.Transfer(txCtx, PayrollTransferRequest{
			Format: claim.Format, Period: claim.Period(),
			ActorAccountID: claim.ActorAccountID, ActorName: claim.ActorName,
			Prepare: func(prepareCtx context.Context) (string, []byte, error) {
				file, reason, err := s.exportFile(prepareCtx, claim)
				if err == nil && reason != "" {
					err = errPayrollBlockedWhileBuilding
				}
				if err != nil {
					return "", nil, err
				}
				claim.Filename = file.Filename
				return file.Filename, file.Data, nil
			},
		})
		return err
	})
	if errors.Is(err, errPayrollBlockedWhileBuilding) {
		blocked, err = workforce.PayrollReasonDatevIncomplete, nil
	}
	if err != nil {
		return false, err
	}
	now := s.deps.Now()
	switch {
	case blocked != "":
		return s.finishWithNotice(ctx, claim, workforce.PayrollDispatchBlocked, blocked)
	case outcome.NotConfigured:
		return s.finishWithNotice(ctx, claim, workforce.PayrollDispatchBlocked, workforce.PayrollReasonSFTPNotConfigured)
	case outcome.Transferred:
		claim.Attempts++
		claim.State, claim.Reason, claim.FinishedAt = workforce.PayrollDispatchSent, "", &now
		return s.finish(ctx, claim, nil)
	default:
		reason := outcome.Reason
		if reason == "" {
			reason = workforce.PayrollReasonInternal
		}
		return s.failAttempt(ctx, claim, reason)
	}
}

// errPayrollBlockedWhileBuilding rolls the transfer's intent back when the
// export itself refuses the DATEV file after the check passed.
var errPayrollBlockedWhileBuilding = errors.New("payroll dispatch: the DATEV configuration became incomplete")

// datevCheck is the check of the manual export dialog (datev-report): a
// DATEV file goes out only when nobody is skipped and every Lohnart has its
// number. Other formats need no check.
func (s *PayrollDispatch) datevCheck(ctx context.Context, claim domain.PayrollDispatch) (string, error) {
	if !strings.HasPrefix(claim.Format, "datev_") {
		return "", nil
	}
	report, err := s.deps.Export.DatevReport(ctx, workforce.TimeExportRequest{Year: claim.Year, Month: claim.Month, Format: claim.Format})
	if errors.Is(err, workforce.ErrPayrollConfigIncomplete) {
		return workforce.PayrollReasonDatevIncomplete, nil
	}
	if err != nil {
		return "", fmt.Errorf("check the DATEV configuration: %w", err)
	}
	if len(report.StaffSkipped) > 0 || len(report.UnconfiguredCategories) > 0 {
		return workforce.PayrollReasonDatevIncomplete, nil
	}
	return "", nil
}

// sendLink builds the file, keeps it behind a fresh one-time link and
// enqueues the link mail, all in one transaction with the claim's outcome.
// Delivery retries the mail; settleMail reads its result.
func (s *PayrollDispatch) sendLink(ctx context.Context, claim domain.PayrollDispatch) (bool, error) {
	done := false
	err := s.deps.Runtime.Within(ctx, func(txCtx context.Context) error {
		file, blocked, err := s.buildFile(txCtx, claim)
		if err != nil {
			return err
		}
		now := s.deps.Now()
		if blocked != "" {
			claim.State, claim.Reason, claim.FinishedAt = workforce.PayrollDispatchBlocked, blocked, &now
			done = true
			return s.finishInTx(txCtx, claim, true)
		}
		tenantID, err := s.deps.Runtime.TenantID(txCtx)
		if err != nil {
			return err
		}
		raw, fingerprint, err := s.deps.Tokens.NewToken()
		if err != nil {
			return fmt.Errorf("create the download link: %w", err)
		}
		expires := now.Add(workforce.PayrollDownloadLifetime)
		outboxID, err := s.enqueueLink(txCtx, payrollLink{
			TenantID: tenantID, DispatchID: claim.ID, Recipient: claim.Recipient,
			Year: claim.Year, Month: claim.Month, Correction: claim.Sequence > 0,
			Token: raw, ExpiresAt: expires,
		})
		if err != nil {
			return fmt.Errorf("enqueue the link mail: %w", err)
		}
		claim.Attempts++
		claim.State, claim.Reason = workforce.PayrollDispatchMailing, ""
		claim.Filename, claim.ContentType, claim.FileData, claim.ByteSize = file.Filename, file.ContentType, file.Data, int64(len(file.Data))
		claim.DownloadTokenHash, claim.DownloadExpiresAt, claim.MailOutboxID = fingerprint, &expires, &outboxID
		claim.NextAttemptAt = now.Add(payrollMailPollInterval)
		return s.finishInTx(txCtx, claim, false)
	})
	return done, err
}

// settleMail reads the link mail's state. A sent mail finishes the dispatch;
// a mail Delivery gave up on fails it, drops the file and tells the
// managers. Either result goes into the transfer journal.
func (s *PayrollDispatch) settleMail(ctx context.Context, claim domain.PayrollDispatch) (bool, error) {
	if claim.MailOutboxID == nil {
		return false, errors.New("payroll dispatch: a mailed dispatch without its mail")
	}
	done := false
	err := s.deps.Runtime.Within(ctx, func(txCtx context.Context) error {
		state, err := s.deps.Mail.State(txCtx, *claim.MailOutboxID)
		if err != nil {
			return err
		}
		now := s.deps.Now()
		switch state {
		case PayrollMailSent:
			claim.State, claim.FinishedAt = workforce.PayrollDispatchSent, &now
		case PayrollMailFailed:
			claim.State, claim.Reason, claim.FinishedAt, claim.FileData = workforce.PayrollDispatchFailed, workforce.PayrollReasonMailFailed, &now, nil
		default:
			claim.NextAttemptAt = now.Add(payrollMailPollInterval)
			return s.finishInTx(txCtx, claim, false)
		}
		done = true
		if err := s.deps.Transfer.RecordEmail(txCtx, PayrollEmailResult{
			Format: claim.Format, Period: claim.Period(), Filename: claim.Filename, ByteSize: claim.ByteSize,
			Recipient: claim.Recipient, Delivered: state == PayrollMailSent,
			ActorAccountID: claim.ActorAccountID, ActorName: claim.ActorName,
		}); err != nil {
			return fmt.Errorf("journal the e-mail send: %w", err)
		}
		return s.finishInTx(txCtx, claim, state == PayrollMailFailed)
	})
	return done, err
}

// failAttempt counts one failed attempt: back to pending with the next wait,
// or failed for good with the notice after the fourth.
func (s *PayrollDispatch) failAttempt(ctx context.Context, claim domain.PayrollDispatch, reason string) (bool, error) {
	claim.Attempts++
	claim.Reason = reason
	now := s.deps.Now()
	if claim.Attempts >= workforce.PayrollDispatchMaxAttempts {
		claim.State, claim.FinishedAt = workforce.PayrollDispatchFailed, &now
		return s.finish(ctx, claim, &claim)
	}
	claim.State = workforce.PayrollDispatchPending
	claim.NextAttemptAt = now.Add(payrollRetryBackoff[min(claim.Attempts, len(payrollRetryBackoff))-1])
	return s.finish(ctx, claim, nil)
}

func (s *PayrollDispatch) finishWithNotice(ctx context.Context, claim domain.PayrollDispatch, state, reason string) (bool, error) {
	now := s.deps.Now()
	claim.State, claim.Reason, claim.FinishedAt = state, reason, &now
	return s.finish(ctx, claim, &claim)
}

// finish writes the pass outcome in a fresh transaction; noticed names the
// dispatch whose notice commits with it.
func (s *PayrollDispatch) finish(ctx context.Context, claim domain.PayrollDispatch, noticed *domain.PayrollDispatch) (bool, error) {
	err := s.deps.Runtime.Within(ctx, func(txCtx context.Context) error {
		return s.finishInTx(txCtx, claim, noticed != nil)
	})
	return err == nil && claim.FinishedAt != nil, err
}

func (s *PayrollDispatch) finishInTx(ctx context.Context, claim domain.PayrollDispatch, notice bool) error {
	kept, err := s.deps.Store.FinishPayrollDispatch(ctx, claim, s.deps.Now())
	if err != nil {
		return fmt.Errorf("payroll dispatch: record the outcome: %w", err)
	}
	if !kept {
		return errors.New("payroll dispatch: the lease expired before the outcome was recorded")
	}
	if notice {
		return s.notify(ctx, claim)
	}
	return nil
}

func (s *PayrollDispatch) notify(ctx context.Context, dispatch domain.PayrollDispatch) error {
	tenantID, err := s.deps.Runtime.TenantID(ctx)
	if err != nil {
		return err
	}
	s.deps.Logger.Warn("payroll dispatch not sent",
		"dispatch_id", dispatch.ID,
		"state", dispatch.State,
		"reason", dispatch.Reason,
	)
	if err := s.notifyManagers(ctx, payrollProblem{
		TenantID: tenantID, DispatchID: dispatch.ID, Year: dispatch.Year, Month: dispatch.Month,
		Correction: dispatch.Sequence > 0, Reason: dispatch.Reason,
	}); err != nil {
		return fmt.Errorf("payroll dispatch: notify the managers: %w", err)
	}
	return nil
}

// Download spends a link of the given tenant once. Unknown, expired and used
// links answer the same error.
func (s *PayrollDispatch) Download(ctx context.Context, tenantID int64, token string) (workforce.PayrollDownload, error) {
	if tenantID <= 0 || token == "" {
		return workforce.PayrollDownload{}, workforce.ErrPayrollDownloadUnavailable
	}
	var download workforce.PayrollDownload
	err := s.deps.Runtime.WithinTenant(s.deps.Runtime.Detached(ctx), tenantID, func(txCtx context.Context) error {
		dispatch, err := s.deps.Store.OpenPayrollDownload(txCtx, s.deps.Tokens.Fingerprint(token), s.deps.Now())
		if err != nil {
			return err
		}
		if dispatch == nil {
			return workforce.ErrPayrollDownloadUnavailable
		}
		s.deps.Logger.Info("payroll download opened",
			"dispatch_id", dispatch.ID,
		)
		download = workforce.PayrollDownload{Filename: dispatch.Filename, ContentType: dispatch.ContentType, Data: dispatch.FileData}
		return nil
	})
	return download, err
}
