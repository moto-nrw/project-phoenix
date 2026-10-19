package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog/consents"

	"github.com/stretchr/testify/require"
)

type consentAppender struct {
	events []*consents.ConsentChange
	err    error
}

func (a *consentAppender) Append(_ context.Context, event any) error {
	if a.err != nil {
		return a.err
	}
	a.events = append(a.events, event.(*consents.ConsentChange))
	return nil
}

func TestConsentRecorderRetainsStateTransitionsAndEventTimes(t *testing.T) {
	t.Parallel()
	granted := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	changed := granted.Add(time.Hour)
	before := &consents.ConsentSnapshot{StudentID: 42, PhotoConsentGivenAt: &granted, AGBAcceptedAt: &granted}
	after := &consents.ConsentSnapshot{StudentID: 42, AGBAcceptedAt: &changed, DataProcessingAcceptedAt: &granted}
	appender := &consentAppender{}
	recorder := newConsentTestCommand(t, appender)
	require.NoError(t, recorder.RecordTransitions(context.Background(), before, after, consents.ConsentSourceImport, nil, changed))
	require.Len(t, appender.events, 2, "changing a timestamp without changing consent must not append an event")
	require.Equal(t, "data_processing", appender.events[0].ConsentKey)
	require.Equal(t, "granted", appender.events[0].Action)
	require.Equal(t, granted, appender.events[0].ChangedAt)
	require.Equal(t, "photo", appender.events[1].ConsentKey)
	require.Equal(t, "withdrawn", appender.events[1].Action)
	require.Equal(t, changed, appender.events[1].ChangedAt)
	require.Equal(t, int64(42), appender.events[1].StudentID)
	require.Equal(t, consents.ConsentSourceImport, appender.events[1].Source)
}

func TestConsentRecorderPropagatesAppendFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("append failed")
	now := time.Now()
	recorder := newConsentTestCommand(t, &consentAppender{err: failure})
	require.ErrorIs(t, recorder.RecordTransitions(context.Background(), nil, &consents.ConsentSnapshot{StudentID: 1, AGBAcceptedAt: &now}, consents.ConsentSourceImport, nil, now), failure)
}

func newConsentTestCommand(t *testing.T, store *consentAppender) *ConsentRecorder {
	t.Helper()
	command, err := NewCommand(store, func(AppendObservation) {})
	require.NoError(t, err)
	return NewConsentRecorder(command)
}
