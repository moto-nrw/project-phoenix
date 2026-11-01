package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{Version: "1.15.721", Description: "Idempotent message send attempts (#5419)", DependsOn: []string{parentConferencesVersion}})
	Migrations.MustRegister(messageSendIdempotencyUp, messageSendIdempotencyDown)
}

// Keys live with their message and follow its existing retention/erasure lifecycle.
const messageSendIdempotencyUpSQL = `
ALTER TABLE users.parent_messages ADD COLUMN client_message_id VARCHAR(128);
CREATE UNIQUE INDEX parent_messages_client_send ON users.parent_messages (tenant_id, sender_account_id, client_message_id) WHERE client_message_id IS NOT NULL;
ALTER TABLE users.staff_messages ADD COLUMN client_message_id VARCHAR(128);
CREATE UNIQUE INDEX staff_messages_client_send ON users.staff_messages (tenant_id, sender_account_id, client_message_id) WHERE client_message_id IS NOT NULL;
ALTER TABLE users.parent_group_messages ADD COLUMN client_message_id VARCHAR(128);
CREATE UNIQUE INDEX parent_group_messages_client_send ON users.parent_group_messages (tenant_id, sender_account_id, client_message_id) WHERE client_message_id IS NOT NULL;
`

const messageSendIdempotencyDownSQL = `
ALTER TABLE users.parent_messages DROP COLUMN client_message_id;
ALTER TABLE users.staff_messages DROP COLUMN client_message_id;
ALTER TABLE users.parent_group_messages DROP COLUMN client_message_id;
`

func messageSendIdempotencyUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, messageSendIdempotencyUpSQL); err != nil {
			return fmt.Errorf("add message send keys: %w", err)
		}
		return nil
	})
}

func messageSendIdempotencyDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, messageSendIdempotencyDownSQL); err != nil {
			return fmt.Errorf("remove message send keys: %w", err)
		}
		return nil
	})
}
