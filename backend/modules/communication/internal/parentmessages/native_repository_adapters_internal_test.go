package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pure boundary probes use in-memory snapshot IDs, never database fixtures.
type nativeParentWriteProbe struct {
	communication.ParentMessageRecords
	create func(context.Context, *communication.PortalParentMessage) error
}

func (p nativeParentWriteProbe) ParentMessageCreate(ctx context.Context, row *communication.PortalParentMessage) error {
	return p.create(ctx, row)
}

func TestNativeParentWritesPreserveTypedPayloadAndAllPartialMetadata(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	at := time.Date(2026, time.October, 17, 12, 30, 0, 123456000, time.UTC)
	ref, actor := int64(67), int64(71)
	payload := map[string]any{"when": at, "whole": int64(9007199254740993), "decimal": json.Number("9007199254740993"), "nested": map[string]any{"answer": int64(42)}}
	row := ParentMessage{ID: 11, CreatedAt: at, UpdatedAt: at.Add(time.Second), TenantID: 13, ThreadID: 17, StudentID: 19, SenderAccountID: 23, SenderKind: "system", SenderName: "  original name  ", StaffNameVisible: true, Body: "original", Kind: "event", EventType: "request_status", EventActorKind: "guardian", RequestType: "care_schedule", RequestStatus: "offen", Payload: payload, RefTable: "requests", RefID: &ref, AppliedAt: &at, AppliedBy: &actor, DecisionReason: "reason", ReadByStaff: true, ReadByGuardian: true}
	expected := row
	expected.ID, expected.TenantID, expected.CreatedAt, expected.UpdatedAt, expected.Body = 73, 79, at.Add(time.Minute), at.Add(2*time.Minute), "normalized before refusal"
	cause := errors.New("original owner refusal")
	calls := 0
	write := func(got context.Context, record *communication.PortalParentMessage) error {
		calls++
		assert.Same(t, ctx, got)
		assert.Equal(t, json.RawMessage(`9007199254740993`), record.Payload["whole"])
		record.ID, record.TenantID, record.CreatedAt, record.UpdatedAt, record.Body = expected.ID, expected.TenantID, expected.CreatedAt, expected.UpdatedAt, expected.Body
		return cause
	}
	repo := NewMessageRepository(nativeParentWriteProbe{create: write})
	err := repo.Create(ctx, &row)
	assert.Same(t, cause, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, expected, row)
	payload["shared"] = "still the original map"
	assert.Equal(t, "still the original map", row.Payload["shared"])
}

func TestNativeParentWritesReturnSourcePayloadChangesEvenOnError(t *testing.T) {
	t.Parallel()
	cause := errors.New("original owner refusal")
	row := ParentMessage{Payload: map[string]any{"value": int64(9007199254740993)}}
	repo := NewMessageRepository(nativeParentWriteProbe{create: func(_ context.Context, record *communication.PortalParentMessage) error {
		// Mutating existing bytes also has to survive, not only replacing the map.
		record.Payload["value"][0] = '8'
		record.Payload["normalized"] = json.RawMessage(`true`)
		return cause
	}})
	err := repo.Create(context.Background(), &row)
	assert.Same(t, cause, err)
	assert.Equal(t, json.Number("8007199254740993"), row.Payload["value"])
	assert.Equal(t, true, row.Payload["normalized"])
}

func TestNativeParentWritesPreserveSourceNilAndEmptyPayloadTransitions(t *testing.T) {
	t.Parallel()
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "nil to empty", true: "empty to nil"}[empty], func(t *testing.T) {
			t.Parallel()
			var payload map[string]any
			if empty {
				payload = map[string]any{}
			}
			row := ParentMessage{Payload: payload}
			repo := NewMessageRepository(nativeParentWriteProbe{create: func(_ context.Context, record *communication.PortalParentMessage) error {
				if empty {
					record.Payload = nil
				} else {
					record.Payload = map[string]json.RawMessage{}
				}
				return nil
			}})
			require.NoError(t, repo.Create(context.Background(), &row))
			if empty {
				assert.Nil(t, row.Payload)
			} else {
				assert.NotNil(t, row.Payload)
				assert.Empty(t, row.Payload)
			}
		})
	}
}

func TestNativeParentWritesForwardNilRecordsAndMissingSources(t *testing.T) {
	t.Parallel()
	assert.Nil(t, NewMessageRepository(nil))
	assert.Nil(t, NewThreadRepository(nil))
	assert.Nil(t, NewReadRepository(nil))
	cause := errors.New("source rejected nil")
	calls := 0
	write := func(_ context.Context, record *communication.PortalParentMessage) error {
		calls++
		assert.Nil(t, record)
		return cause
	}
	repo := NewMessageRepository(nativeParentWriteProbe{create: write})
	assert.Same(t, cause, repo.Create(context.Background(), nil))
	assert.Equal(t, 1, calls)
}

type nativeParentInboxProbe struct {
	communication.ParentReadRecords
	list func(context.Context, int64, bool, bool) ([]*communication.PortalInboxThread, error)
}

func (p nativeParentInboxProbe) ParentReadListInboxForStaff(ctx context.Context, id int64, all, unread bool) ([]*communication.PortalInboxThread, error) {
	return p.list(ctx, id, all, unread)
}

func TestNativeParentInboxRetainsCompleteSnapshotSelectorsAndPartialError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	at := time.Date(2026, time.October, 17, 12, 30, 0, 0, time.UTC)
	group := int64(31)
	source := communication.PortalInboxThread{ThreadID: 11, TenantID: 13, StudentID: 17, StudentName: "child", SchoolClass: "class", SchoolName: "school", GroupID: &group, GroupName: "group", GuardianAccountID: 19, GuardianName: "guardian", RelationshipType: "parent", LastMessageAt: &at, LastSenderKind: "system", LastMessageBody: "body", LastMessageKind: "event", LastEventType: "request_status", LastRequestType: "care_schedule", LastRequestStatus: "offen", LastMessagePayload: map[string]json.RawMessage{"large": json.RawMessage(`9007199254740993`)}, LastMessageReadByStaff: true, UnreadCount: 3, ReadBound: &communication.PortalReadCursorBound{ThreadID: 11, ReadAt: at, MessageID: 23}}
	expected := InboxThread{ThreadID: 11, TenantID: 13, StudentID: 17, StudentName: "child", SchoolClass: "class", SchoolName: "school", GroupID: &group, GroupName: "group", GuardianAccountID: 19, GuardianName: "guardian", RelationshipType: "parent", LastMessageAt: &at, LastSenderKind: "system", LastMessageBody: "body", LastMessageKind: "event", LastEventType: "request_status", LastRequestType: "care_schedule", LastRequestStatus: "offen", LastMessagePayload: map[string]any{"large": json.Number("9007199254740993")}, LastMessageReadByStaff: true, UnreadCount: 3}
	cause := errors.New("partial owner result")
	repo := NewReadRepository(nativeParentInboxProbe{list: func(got context.Context, id int64, all, unread bool) ([]*communication.PortalInboxThread, error) {
		assert.Same(t, ctx, got)
		assert.Equal(t, int64(29), id)
		assert.False(t, all)
		assert.True(t, unread)
		return []*communication.PortalInboxThread{nil, &source}, cause
	}})
	rows, err := repo.ListInboxForStaff(ctx, 29, false, true)
	assert.Same(t, cause, err)
	require.Len(t, rows, 2)
	assert.Nil(t, rows[0])
	require.NotNil(t, rows[1].ReadBound)
	assert.Equal(t, source.ReadBound.ThreadID, rows[1].ReadBound.ThreadID)
	assert.Equal(t, source.ReadBound.ReadAt, rows[1].ReadBound.ReadAt)
	assert.Equal(t, source.ReadBound.MessageID, rows[1].ReadBound.MessageID)
	actual := *rows[1]
	actual.ReadBound = nil
	assert.Equal(t, expected, actual)
}

func TestNativeParentInboxRetainsNilAndEmptyCollections(t *testing.T) {
	t.Parallel()
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "nil", true: "empty"}[empty], func(t *testing.T) {
			t.Parallel()
			repo := NewReadRepository(nativeParentInboxProbe{list: func(context.Context, int64, bool, bool) ([]*communication.PortalInboxThread, error) {
				if empty {
					return []*communication.PortalInboxThread{}, nil
				}
				return nil, nil
			}})
			rows, err := repo.ListInboxForStaff(context.Background(), 0, true, false)
			require.NoError(t, err)
			if empty {
				assert.NotNil(t, rows)
				assert.Empty(t, rows)
			} else {
				assert.Nil(t, rows)
			}
		})
	}
}
