package filestore

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/database/repositories/documents"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/filestore"
	"github.com/uptrace/bun"
)

// AnnouncementAttachmentRepository implementiert
// filestore.AnnouncementAttachmentRepository über dem gemeinsamen
// Dokument-Repository (#2890). Alles außer der Zählung ist geerbt: die ~300
// Zeilen Abfrage-Handwerk für Soft Delete und Cleanup-Intents existieren
// genau einmal.
type AnnouncementAttachmentRepository struct {
	*documents.Repository[*filestore.AnnouncementAttachment, *filestore.AnnouncementAttachmentCleanup]
	db *bun.DB
}

// NewAnnouncementAttachmentRepository erzeugt das Metadaten-Repository der
// Mitteilungs-Anhänge.
func NewAnnouncementAttachmentRepository(db *bun.DB) filestore.AnnouncementAttachmentRepository {
	return &AnnouncementAttachmentRepository{
		Repository: documents.NewRepository[*filestore.AnnouncementAttachment, *filestore.AnnouncementAttachmentCleanup](db, documents.Config{
			Table:        "documents.announcement_attachments",
			Alias:        "announcement_attachment",
			OwnerColumn:  "announcement_id",
			CleanupTable: "documents.announcement_attachment_cleanup",
			CleanupAlias: "announcement_attachment_cleanup",
		}),
		db: db,
	}
}

// CountByOwnerID zählt die lebenden Anhänge einer Mitteilung. Der Dienst
// prüft damit die Obergrenze, ohne die Zeilen zu laden.
func (r *AnnouncementAttachmentRepository) CountByOwnerID(ctx context.Context, announcementID int64) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*filestore.AnnouncementAttachment)(nil)).
		ModelTableExpr(`documents.announcement_attachments AS "announcement_attachment"`).
		Where(`"announcement_attachment".announcement_id = ?`, announcementID)
	query = base.WithTenantFilter(ctx, query, "announcement_attachment")
	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count announcement attachments", Err: base.TranslateNotFound(err)}
	}
	return count, nil
}
