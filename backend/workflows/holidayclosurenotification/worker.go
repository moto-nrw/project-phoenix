package holidayclosurenotification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// The worker (ADR 0196 sections 2, 5 and 6) processes only confirmed
// commands. It leases one in a short Träger transaction, processes it in a
// second one that holds the command under its lease and Enrollment's lock,
// and writes the outcome there, so the told days, the send records, the
// mails and the outcome commit together. An expired lease is taken over; a
// worker that lost its lease writes nothing. A failure goes back to pending
// with a growing wait and turns into a visible "blocked" after
// maxAttempts; it never ends silently.

const (
	leaseFor      = 5 * time.Minute
	maxAttempts   = 5
	retryBase     = time.Minute
	maxPerTraeger = 50
)

// ErrBlocked reports that a command could not be processed and waits for a
// click; the scheduler raises the alarm for it.
var ErrBlocked = errors.New("holiday closure notification: a confirmed command is blocked")

// ProcessDue processes the due commands of every Träger, follows the mails
// of prepared commands and retires finished ones.
func (s *Service) ProcessDue(ctx context.Context) error {
	var organizations []int64
	if err := s.Units.WithinAdmin(ctx, func(ctx context.Context) error {
		var err error
		organizations, err = s.Queue.DueHolidayClosureTraeger(ctx)
		return err
	}); err != nil {
		return err
	}
	var errs []error
	for _, organizationID := range organizations {
		if err := s.processTraeger(ctx, organizationID); err != nil {
			errs = append(errs, fmt.Errorf("organization %d: %w", organizationID, err))
		}
	}
	return errors.Join(errs...)
}

// processTraeger works through the Träger's due commands, then settles the
// waiting ones and retires the finished.
func (s *Service) processTraeger(ctx context.Context, organizationID int64) error {
	var errs []error
	for range maxPerTraeger {
		var event schoolcalendar.HolidayClosureEvent
		var leased bool
		if err := s.Units.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
			var err error
			event, leased, err = s.Queue.LeaseHolidayClosureEvent(ctx, organizationID, leaseFor)
			return err
		}); err != nil {
			return err
		}
		if !leased {
			break
		}
		if err := s.process(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	if err := s.settle(ctx, organizationID); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// process tells the parents of one leased command.
func (s *Service) process(ctx context.Context, event schoolcalendar.HolidayClosureEvent) error {
	var outcome schoolcalendar.HolidayClosureOutcome
	err := s.Units.WithinTraeger(ctx, event.OrganizationID, func(ctx context.Context) error {
		if err := s.Queue.LockHolidayClosurePreparation(ctx, event.OrganizationID, []int64{event.SchoolID}); err != nil {
			return err
		}
		if err := s.Queue.LockLeasedHolidayClosureEvent(ctx, event.OrganizationID, event.ID, event.LeaseToken); err != nil {
			return err
		}
		confirmed := operation(event.SchoolID, event.OperationID, event.SpanFrom, event.SpanTo)
		confirmed.Revision, confirmed.Token = &event.Revision, event.ImpactToken
		confirmed.ConstrainRanges = true
		for _, r := range event.Ranges {
			confirmed.Ranges = append(confirmed.Ranges, enrollment.HolidayClosureRange{From: r.StartsOn, To: r.EndsOn})
		}
		result, err := s.Closures.NotifyHolidayClosureOperation(ctx, event.OrganizationID, confirmed)
		if errors.Is(err, enrollment.ErrHolidayClosureChanged) {
			outcome = schoolcalendar.HolidayClosureOutcome{State: schoolcalendar.HolidayClosureEventNeedsConfirmation}
		} else if err != nil {
			return err
		} else {
			outcome = schoolcalendar.HolidayClosureOutcome{State: schoolcalendar.HolidayClosureEventNoMail}
			if result.Mails > 0 {
				outcome.State = schoolcalendar.HolidayClosureEventPrepared
			}
			s.Logger.Info("holiday closure event processed",
				"event_id", event.ID,
				"organization_id", event.OrganizationID,
				"school_id", event.SchoolID,
				"families", result.Families,
				"mails", result.Mails,
				"retried", result.Retried,
			)
		}
		return s.Queue.FinishHolidayClosureEvent(ctx, event.OrganizationID, event.ID, event.LeaseToken, outcome)
	})
	if err == nil || errors.Is(err, schoolcalendar.ErrHolidayClosureLeaseLost) {
		return nil
	}
	return s.fail(ctx, event, err)
}

// fail writes a failure back under the still valid lease: pending again
// with a growing wait, or blocked after maxAttempts.
func (s *Service) fail(ctx context.Context, event schoolcalendar.HolidayClosureEvent, cause error) error {
	outcome := schoolcalendar.HolidayClosureOutcome{State: schoolcalendar.HolidayClosureEventPending, ErrorCode: errorCode(cause),
		RetryAt: time.Now().Add(retryBase << min(event.Attempts, 6))}
	if event.Attempts >= maxAttempts || errors.Is(cause, schoolcalendar.ErrHolidayClosureNoticeNotFound) {
		outcome = schoolcalendar.HolidayClosureOutcome{State: schoolcalendar.HolidayClosureEventBlocked, ErrorCode: errorCode(cause)}
	}
	s.Logger.Error("holiday closure event failed",
		"event_id", event.ID,
		"organization_id", event.OrganizationID,
		"attempts", event.Attempts,
		"state", outcome.State,
		"error_code", outcome.ErrorCode,
		"error", cause.Error(),
	)
	if err := s.Units.WithinTraeger(ctx, event.OrganizationID, func(ctx context.Context) error {
		return s.Queue.FinishHolidayClosureEvent(ctx, event.OrganizationID, event.ID, event.LeaseToken, outcome)
	}); err != nil && !errors.Is(err, schoolcalendar.ErrHolidayClosureLeaseLost) {
		return errors.Join(cause, err)
	}
	if outcome.State == schoolcalendar.HolidayClosureEventBlocked {
		return fmt.Errorf("%w: event %d: %s", ErrBlocked, event.ID, outcome.ErrorCode)
	}
	return nil
}

// errorCode names a failure technically, without personal data.
func errorCode(err error) string {
	switch {
	case errors.Is(err, schoolcalendar.ErrHolidayClosureNoticeNotFound):
		return "school_outside_traeger"
	case errors.Is(err, schoolcalendar.ErrFederalStateUnavailable):
		return "federal_state_unavailable"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "timeout"
	default:
		return "processing_failed"
	}
}

// settle moves prepared and unknown commands to the state their mails
// reached and retires finished commands after 30 days.
func (s *Service) settle(ctx context.Context, organizationID int64) error {
	return s.Units.WithinTraeger(ctx, organizationID, func(ctx context.Context) error {
		events, err := s.Queue.WaitingHolidayClosureEvents(ctx, organizationID)
		if err != nil {
			return err
		}
		operationIDs := make([]string, len(events))
		for i, event := range events {
			operationIDs[i] = event.OperationID
		}
		deliveries, err := s.Closures.HolidayClosureOperationsDelivery(ctx, organizationID, operationIDs)
		if err != nil {
			return err
		}
		var settlements []schoolcalendar.HolidayClosureSettlement
		for _, event := range events {
			if state := settledState(deliveries[event.OperationID]); state != "" && state != event.State {
				settlements = append(settlements, schoolcalendar.HolidayClosureSettlement{EventID: event.ID, State: state})
			}
		}
		if err := s.Queue.SettleHolidayClosureEvents(ctx, organizationID, settlements); err != nil {
			return err
		}

		_, err = s.Queue.RetireHolidayClosureEvents(ctx, organizationID)
		return err
	})
}

// settledState names the state of a command from its mails: a known
// failure first, then an unknown outcome, then waiting; "Gesendet" only
// when every mail was accepted.
func settledState(delivery enrollment.HolidayClosureDelivery) string {
	switch {
	case delivery.Failed > 0:
		return schoolcalendar.HolidayClosureEventFailed
	case delivery.Unknown > 0:
		return schoolcalendar.HolidayClosureEventUnknown
	case delivery.Queued > 0:
		return schoolcalendar.HolidayClosureEventPrepared
	case delivery.Sent > 0:
		return schoolcalendar.HolidayClosureEventCompleted
	default:
		return ""
	}
}
