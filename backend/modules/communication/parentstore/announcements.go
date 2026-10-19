package parentstore

import (
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
	"github.com/uptrace/bun"
)

func NewParentAnnouncementRepository(db *bun.DB, audience recordports.AnnouncementAudienceReads) recordports.ParentAnnouncementRepository {
	return parentpostgres.NewParentAnnouncementRepository(db, audience)
}
