package staffmessaging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nativeMessageSource struct {
	communication.StaffMessageRecords
	create func(context.Context, *communication.StaffMessageRecord) error
}

func (s nativeMessageSource) CreateStaffMessage(ctx context.Context, input *communication.StaffMessageRecord) error {
	return s.create(ctx, input)
}

type nativeReadSource struct {
	communication.StaffReadRecords
	rows    []*communication.StaffInboxRecord
	failure error
	check   func(context.Context, int64, bool)
}

func (s nativeReadSource) ListStaffInbox(ctx context.Context, id int64, unread bool) ([]*communication.StaffInboxRecord, error) {
	s.check(ctx, id, unread)
	return s.rows, s.failure
}

func TestNativeStaffMessagePortPreservesCompleteValueAndFailureMutations(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	created := time.Date(2026, time.October, 17, 12, 30, 0, 0, time.UTC)
	failure := errors.New("append refused after metadata mutation")
	input := &StaffMessage{ID: 13, TenantID: 17, ThreadID: 19, SenderAccountID: 23, SenderName: "  original name  ", Body: "body", CreatedAt: created, UpdatedAt: created.Add(time.Second)}
	expected := communication.StaffMessageRecord(*input)
	source := nativeMessageSource{create: func(actual context.Context, row *communication.StaffMessageRecord) error {
		assert.Same(t, ctx, actual)
		assert.Equal(t, expected, *row)
		row.ID++
		row.UpdatedAt = created.Add(2 * time.Second)
		return failure
	}}
	err := NewMessageRepository(source).Create(ctx, input)
	assert.Same(t, failure, err)
	assert.Equal(t, expected.ID+1, input.ID)
	assert.Equal(t, created.Add(2*time.Second), input.UpdatedAt)
	assert.Equal(t, expected.SenderName, input.SenderName)
	assert.Nil(t, NewThreadRepository(nil))
	assert.Nil(t, NewMessageRepository(nil))
	assert.Nil(t, NewReadRepository(nil))
}

func TestNativeStaffInboxPortPreservesSelectorsPartialErrorsAndCollections(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("partial inbox")
	at := time.Date(2026, time.October, 17, 12, 30, 0, 0, time.UTC)
	sender := int64(len("staff-sender"))
	record := &communication.StaffInboxRecord{ThreadID: 13, TenantID: 17, CounterpartAccountID: 19, CounterpartName: "colleague", CounterpartRoleKind: "admin", LastMessageAt: &at, LastMessageBody: "body", LastSenderAccountID: &sender, UnreadCount: 3}
	for _, rows := range [][]*communication.StaffInboxRecord{nil, {}, {nil, record}} {
		source := nativeReadSource{rows: rows, failure: failure, check: func(actual context.Context, id int64, unread bool) {
			assert.Same(t, ctx, actual)
			assert.EqualValues(t, 23, id)
			assert.True(t, unread)
		}}
		actual, err := NewReadRepository(source).ListInbox(ctx, 23, true)
		assert.Same(t, failure, err)
		assert.Equal(t, rows == nil, actual == nil)
		require.Len(t, actual, len(rows))
		if len(rows) > 0 {
			assert.Nil(t, actual[0])
			assert.Equal(t, *record, communication.StaffInboxRecord(*actual[1]))
		}
	}
}
