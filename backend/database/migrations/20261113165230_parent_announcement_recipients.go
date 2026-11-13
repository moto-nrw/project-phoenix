package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentAnnouncementRecipientsVersion = "20261113165230"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentAnnouncementRecipientsVersion,
		Description: "Ankündigungen: Empfänger beim Veröffentlichen festhalten (#6988)",
		DependsOn: []string{
			parentAnnouncementsVersion, // the announcements that record their recipients (1.15.161)
		},
	})
	Migrations.MustRegister(parentAnnouncementRecipientsUp, parentAnnouncementRecipientsDown)
}

// parentAnnouncementRecipientsUpSQL records who an announcement reached when
// it was published (#6988).
//
//   - recipient_account_ids holds the guardian accounts the publication
//     reached. Statistics and the staff recipient list count these accounts,
//     so a child who changes class after the publication does not move the
//     e-mail recipients of the old class out of "Wer hat noch nicht
//     bestätigt" and the parents of the new class in.
//   - NULL means "not recorded": a draft, or an announcement published before
//     this column existed; those keep the audience of today. An empty array
//     is a recorded publication that reached nobody.
//   - The IDs live on the announcement row, so they go with it, are retracted
//     with it (unpublish sets NULL) and follow its retention and the school
//     erasure without a table of their own. An erased account keeps an inert
//     ID here: the readers require an active school membership, which the
//     erasure removes.
const parentAnnouncementRecipientsUpSQL = `
ALTER TABLE users.parent_announcements ADD COLUMN recipient_account_ids BIGINT[];
COMMENT ON COLUMN users.parent_announcements.recipient_account_ids IS
	'Guardian accounts the publication reached (#6988); NULL for drafts and for announcements published before recording.';
`

const parentAnnouncementRecipientsDownSQL = `ALTER TABLE users.parent_announcements DROP COLUMN IF EXISTS recipient_account_ids;`

func parentAnnouncementRecipientsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentAnnouncementRecipientsUpSQL); err != nil {
		return fmt.Errorf("add parent announcement recipients: %w", err)
	}
	return nil
}

func parentAnnouncementRecipientsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentAnnouncementRecipientsDownSQL); err != nil {
		return fmt.Errorf("drop parent announcement recipients: %w", err)
	}
	return nil
}
