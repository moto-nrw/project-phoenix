package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type ParentConversationSource interface {
	communication.PortalConversationCore
}

// BindParentConversationCore adapts complete native writes and receipt facts.
func BindParentConversationCore(appendMessage func(context.Context, *ParentMessage) error, markRead func(context.Context, int64, int64, int64, bool, []*ParentMessage) (bool, error), decorate func(context.Context, int64, int64, []*ParentMessage), broadcast, broadcastRead func(int64, int64, int64, int64)) ParentConversationSource {
	return communication.NewPortalConversationCore(func(ctx context.Context, row *communication.PortalParentMessage) error {
		return appendPortalMessage(ctx, appendMessage, row)
	}, func(ctx context.Context, a, b, c int64, staff bool, rows []*communication.PortalParentMessage) (bool, error) {
		return markRead(ctx, a, b, c, staff, parentCursorFacts(rows))
	}, func(ctx context.Context, a, b int64, rows []*communication.PortalParentMessage) {
		decoratePortalReceipts(ctx, decorate, a, b, rows)
	}, broadcast, broadcastRead)
}
func appendPortalMessage(ctx context.Context, appendMessage func(context.Context, *ParentMessage) error, row *communication.PortalParentMessage) error {
	value, err := ParentMessageFromRecord(row)
	if err != nil {
		return err
	}
	err = appendMessage(ctx, value)
	mapped, mapErr := ParentMessageToRecord(value)
	if row != nil && mapped != nil {
		*row = *mapped
	}
	if err != nil {
		return err
	}
	return mapErr
}
func decoratePortalReceipts(ctx context.Context, decorate func(context.Context, int64, int64, []*ParentMessage), threadID, other int64, rows []*communication.PortalParentMessage) {
	values := parentCursorFacts(rows)
	decorate(ctx, threadID, other, values)
	for i, v := range values {
		if v != nil && rows[i] != nil {
			rows[i].ReadByStaff = v.ReadByStaff
			rows[i].ReadByGuardian = v.ReadByGuardian
		}
	}
}

// Receipt policy consumes only cursor/actor facts. Payload can contain opaque
// RawMessage values and must never be encoded or decoded on this path.
func parentCursorFacts(rows []*communication.PortalParentMessage) []*ParentMessage {
	if rows == nil {
		return nil
	}
	values := make([]*ParentMessage, len(rows))
	for i, v := range rows {
		if v != nil {
			values[i] = &ParentMessage{ID: v.ID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, TenantID: v.TenantID, ThreadID: v.ThreadID, StudentID: v.StudentID, SenderAccountID: v.SenderAccountID, SenderKind: v.SenderKind, SenderName: v.SenderName, StaffNameVisible: v.StaffNameVisible, Body: v.Body, Kind: v.Kind, EventType: v.EventType, EventActorKind: v.EventActorKind, RequestType: v.RequestType, RequestStatus: v.RequestStatus, RefTable: v.RefTable, RefID: v.RefID, AppliedAt: v.AppliedAt, AppliedBy: v.AppliedBy, DecisionReason: v.DecisionReason, GroupThreadID: v.GroupThreadID, GroupThreadTitle: v.GroupThreadTitle, ReadByStaff: v.ReadByStaff, ReadByGuardian: v.ReadByGuardian}
		}
	}
	return values
}
