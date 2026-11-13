package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/auditlog/consents"
)

type consentEventAppender interface {
	Append(context.Context, any) error
}

// ConsentRecorder converts effective consent changes into events for the
// shared Audit append command.
type ConsentRecorder struct{ command consentEventAppender }

func NewConsentRecorder(command consentEventAppender) *ConsentRecorder {
	return &ConsentRecorder{command: command}
}

func (r *ConsentRecorder) RecordTransitions(ctx context.Context, before, after *consents.ConsentSnapshot, source string, actorAccountID *int64, changedAt time.Time) error {
	if r == nil || r.command == nil {
		return fmt.Errorf("student consent recorder: appender not wired")
	}
	events, err := r.transitionEvents(before, after, source, actorAccountID, changedAt)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := r.command.Append(ctx, event); err != nil {
			return fmt.Errorf("student consent recorder: record %s transition: %w", event.ConsentKey, err)
		}
	}
	return nil
}

var _ consents.ConsentTransitionBatches = (*ConsentRecorder)(nil)

type consentEventBatchAppender interface {
	AppendAll(context.Context, []any) error
}

// RecordTransitionsAll is RecordTransitions for many students: the events of
// every transition, in input order, go to the Audit command in one AppendAll
// where it can, one by one otherwise.
func (r *ConsentRecorder) RecordTransitionsAll(ctx context.Context, transitions []consents.ConsentTransition) error {
	var events []any
	for _, transition := range transitions {
		changes, err := r.transitionEvents(transition.Before, transition.After, transition.Source, transition.ActorAccountID, transition.ChangedAt)
		if err != nil {
			return err
		}
		for _, change := range changes {
			events = append(events, change)
		}
	}
	if len(events) == 0 {
		return nil
	}
	if batch, ok := r.command.(consentEventBatchAppender); ok {
		if err := batch.AppendAll(ctx, events); err != nil {
			return fmt.Errorf("student consent recorder: record transitions: %w", err)
		}
		return nil
	}
	for _, event := range events {
		change := event.(*consents.ConsentChange)
		if err := r.command.Append(ctx, change); err != nil {
			return fmt.Errorf("student consent recorder: record %s transition: %w", change.ConsentKey, err)
		}
	}
	return nil
}

// transitionEvents are the events of one effective consent change.
func (r *ConsentRecorder) transitionEvents(before, after *consents.ConsentSnapshot, source string, actorAccountID *int64, changedAt time.Time) ([]*consents.ConsentChange, error) {
	if r == nil || r.command == nil {
		return nil, fmt.Errorf("student consent recorder: appender not wired")
	}
	if after == nil || after.StudentID <= 0 {
		return nil, fmt.Errorf("student consent recorder: persisted student is required")
	}
	if before == nil {
		before = &consents.ConsentSnapshot{}
	}
	fields := []struct {
		key           string
		before, after *time.Time
	}{
		{"agb", before.AGBAcceptedAt, after.AGBAcceptedAt},
		{"data_processing", before.DataProcessingAcceptedAt, after.DataProcessingAcceptedAt},
		{"email_contact", before.EmailContactAcceptedAt, after.EmailContactAcceptedAt},
		{"photo", before.PhotoConsentGivenAt, after.PhotoConsentGivenAt},
	}
	var events []*consents.ConsentChange
	for _, field := range fields {
		if (field.before != nil) == (field.after != nil) {
			continue
		}
		action, instant := "withdrawn", changedAt
		if field.after != nil {
			action, instant = "granted", *field.after
		}
		events = append(events, &consents.ConsentChange{StudentID: after.StudentID, ConsentKey: field.key, Action: action, Source: source, ActorAccountID: actorAccountID, ChangedAt: instant})
	}
	return events, nil
}
