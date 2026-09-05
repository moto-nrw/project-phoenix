package repositories

import (
	"context"
	"fmt"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	fileRepo "github.com/moto-nrw/project-phoenix/database/repositories/filestore"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	fileModels "github.com/moto-nrw/project-phoenix/models/filestore"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type FileStoreTestRepositories struct {
	Folders       fileModels.FolderRepository
	Files         fileModels.FileRepository
	Attachments   fileModels.AnnouncementAttachmentRepository
	Events        auditModels.FileEventRepository
	Announcements usersModels.ParentAnnouncementRepository
}

func NewTestAuditStore(db *bun.DB) auditModels.AppendStore {
	return auditRepo.NewAppender(newTestAuditRuntime(db))
}

func newTestAuditRuntime(db *bun.DB) auditRepo.Runtime {
	return func(ctx context.Context) (bun.IDB, int64) {
		tenantID := auditModels.TenantIDFromContext(ctx)
		raw, ok := auditModels.TransactionFromContext(ctx)
		if !ok {
			return db, tenantID
		}
		switch tx := raw.(type) {
		case bun.Tx:
			return tx, tenantID
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID
			}
		}
		panic(fmt.Sprintf("test audit: unsupported transaction %T", raw))
	}
}

func NewFileStoreTestRepositories(db *bun.DB, command auditModels.Command) (FileStoreTestRepositories, error) {
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return FileStoreTestRepositories{}, err
	}
	return FileStoreTestRepositories{
		Folders: personFolderRepository{FolderRepository: fileRepo.NewFolderRepository(db), persons: persons},
		Files:   fileRepo.NewFileRepository(db), Attachments: fileRepo.NewAnnouncementAttachmentRepository(db),
		Events: NewFileEventTestRepository(db, command), Announcements: usersRepo.NewParentAnnouncementRepository(db),
	}, nil
}

// NewFileEventTestRepository retains the Audit command write boundary while
// composing only the file-event query contract.
func NewFileEventTestRepository(db *bun.DB, command auditModels.Command) auditModels.FileEventRepository {
	return fileEventCommand{
		auditRepo.NewFileEventRepository(newTestAuditRuntime(db)),
		command,
	}
}
