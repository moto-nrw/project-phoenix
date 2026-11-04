package staffstore

import (
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffpostgres"
	"github.com/uptrace/bun"
)

func NewStaffMessageThreadRepository(db *bun.DB) communication.StaffConversationRecords {
	return staffpostgres.NewThreadStore(postgresDatabase(db))
}
func NewStaffMessageRepository(db *bun.DB) communication.StaffMessageRecords {
	return staffpostgres.NewMessageStore(postgresDatabase(db))
}

// NewStaffMessageReadRepository binds the existing cursors and inbox to the
// exact colleague relation supplied by People Directory at the root.
func NewStaffMessageReadRepository(db *bun.DB, colleagues communication.StaffColleagueReads, inbox communication.StaffInboxReads) communication.StaffReadRecords {
	if colleagues == nil {
		panic("communication staff store: colleague directory is required")
	}
	return staffpostgres.NewReadRecords(colleagues, staffpostgres.NewReadCursorStore(postgresDatabase(db)), inbox)
}
