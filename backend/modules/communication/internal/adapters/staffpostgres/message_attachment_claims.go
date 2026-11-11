package staffpostgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// The photos and files a message carries (#6685). A send names its uploads;
// the store locks them before it inserts the message and claims them right
// after, so a message and its files commit together.

const lockMessageAttachmentsSQL = `SELECT attachment.id, attachment.filename_display AS filename,
	attachment.size_bytes, attachment.content_type,
	COALESCE(attachment.width, 0) AS width, COALESCE(attachment.height, 0) AS height
FROM users.message_attachments AS attachment
WHERE attachment.tenant_id = ? AND attachment.scope = ? AND attachment.uploaded_by_account_id = ?
  AND attachment.id = ANY(?)
FOR UPDATE OF attachment`

// reserveMessageAttachments locks the uploads the send in ctx names and
// returns their ids with the snapshot the message row stores. They must
// belong to the sender and the chat family; whether they are still free is
// the claim's check, so a replayed send finds the files its first attempt
// claimed.
func reserveMessageAttachments(ctx context.Context, db bun.IDB, tenantID int64, sender int64) ([]int64, json.RawMessage, error) {
	ids := communication.MessageAttachmentIDs(ctx)
	if len(ids) == 0 {
		return nil, nil, nil
	}
	if len(ids) > communication.MaxMessageAttachments {
		return nil, nil, communication.ErrMessageAttachmentLimit
	}
	var locked []communication.MessageAttachment
	if err := db.NewRaw(lockMessageAttachmentsSQL, tenantID, communication.MessageAttachmentScopeStaff, sender, pgdialect.Array(ids)).Scan(ctx, &locked); err != nil {
		return nil, nil, fmt.Errorf("reserve message attachments: %w", err)
	}
	snapshot, err := communication.ReservedMessageAttachments(ids, locked)
	return ids, snapshot, err
}

// claimMessageAttachments binds the reserved uploads to the message just
// inserted. An upload another message already carries refuses the send.
func claimMessageAttachments(ctx context.Context, db bun.IDB, claim *bun.UpdateQuery, tenantID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	result, err := claim.Set("claimed_at = clock_timestamp()").
		Where("attachment.tenant_id = ? AND attachment.claimed_at IS NULL", tenantID).
		Where("attachment.id = ANY(?)", pgdialect.Array(ids)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("claim message attachments: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("claim message attachments: %w", err)
	}
	if affected != int64(len(ids)) {
		return communication.ErrMessageAttachmentUnavailable
	}
	return nil
}
