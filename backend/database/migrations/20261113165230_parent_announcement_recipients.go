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
			parentAnnouncementsVersion, // the announcements that record their reached children (1.15.161)
		},
	})
	Migrations.MustRegister(parentAnnouncementRecipientsUp, parentAnnouncementRecipientsDown)
}

// parentAnnouncementRecipientsUpSQL records which children an announcement
// reached when it was published (#6988).
//
//   - recipient_student_ids holds the children the targets reached at the
//     publication. Statistics, the recipient list, the feed, the letter
//     status and the reminders follow these children, so a child who changes
//     class after a letter went out keeps it and the new class does not get
//     it. Who may read for a child stays live: the guardian relationship and
//     its parent_portal permissions decide as before.
//   - NULL means "not recorded": a draft, or an announcement published before
//     this column existed; those keep matching the targets of today. An empty
//     array is a recorded publication that reached no child.
//   - The IDs live on the announcement row, so they go with it, are retracted
//     with it (unpublish sets NULL) and follow its retention and the school
//     erasure. A child ID holds no personal data of its own; a deleted child
//     no longer joins.
const parentAnnouncementRecipientsUpSQL = `
ALTER TABLE users.parent_announcements ADD COLUMN recipient_student_ids BIGINT[];
COMMENT ON COLUMN users.parent_announcements.recipient_student_ids IS
	'Children the publication reached (#6988); NULL for drafts and for announcements published before recording.';
`

const parentAnnouncementRecipientsDownSQL = `ALTER TABLE users.parent_announcements DROP COLUMN IF EXISTS recipient_student_ids;`

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
