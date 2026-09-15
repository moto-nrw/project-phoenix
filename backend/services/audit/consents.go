package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog/consents"
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
	if after == nil || after.StudentID <= 0 {
		return fmt.Errorf("student consent recorder: persisted student is required")
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
	for _, field := range fields {
		if (field.before != nil) == (field.after != nil) {
			continue
		}
		action, instant := "withdrawn", changedAt
		if field.after != nil {
			action, instant = "granted", *field.after
		}
		event := &consents.ConsentChange{StudentID: after.StudentID, ConsentKey: field.key, Action: action, Source: source, ActorAccountID: actorAccountID, ChangedAt: instant}
		if err := r.command.Append(ctx, event); err != nil {
			return fmt.Errorf("student consent recorder: record %s transition: %w", field.key, err)
		}
	}
	return nil
}
