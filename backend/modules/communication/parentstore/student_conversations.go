package parentstore

import (
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/uptrace/bun"
)

func NewStudentConversations(db *bun.DB) *parentpostgres.StudentConversationStore {
	return parentpostgres.NewStudentConversationStore(postgresDatabase(db))
}
