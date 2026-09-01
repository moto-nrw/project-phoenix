package filestore

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/documents"
)

// Anhänge an Elternmitteilungen (#2890).
//
// Warum hier und nicht bei der Mitteilung: ein Anhang ist eine gespeicherte
// Datei und erbt alles, was models/documents an einer Datei schon weiß — Soft
// Delete der Metadaten, file_deleted_at für die Bytes, ein Cleanup-Intent, der
// einen abgebrochenen Upload überlebt. Die Mitteilung steuert nur bei, WER die
// Datei sehen darf; diese Frage beantwortet sie über einen Port
// (services/filestore.AnnouncementAudience), nicht über einen Import.
//
// Der Anhang hängt bewusst an der Mitteilung und ist keine vierte Sichtbarkeit
// der Dateiablage: die zugesagten Empfängerkreise (Schule, Klasse, Gruppe,
// Angebot, einzelnes Kind) gibt es an users.parent_announcements bereits als
// Zieltypen. Eine dauerhafte Eltern-Ablage mit Ordnern ist damit ausdrücklich
// NICHT erledigt.

// AnnouncementAttachmentCategory ist die einzige Kategorie, die ein Anhang
// trägt. Das gemeinsame Dokument-Repository filtert nach Kategorie; ein
// Bestand ohne Kategorien hat eben genau eine.
const AnnouncementAttachmentCategory = "announcement_attachment"

// MaxAnnouncementAttachments begrenzt die Anhänge je Mitteilung. Die Grenze
// ist eine Produktentscheidung, keine technische: eine Elternmitteilung mit
// mehr als einer Handvoll Dateien ist keine Mitteilung mehr, und die Eltern
// laden sie einzeln herunter.
const MaxAnnouncementAttachments = 5

// AnnouncementAttachment ist eine an eine Elternmitteilung gehängte Datei.
// Nur Metadaten — die Bytes liegen im Storage-Backend unter dem gespeicherten
// (UUID-)Namen und werden ausschließlich über die Download-Handler
// ausgeliefert, die den Empfängerkreis prüfen.
type AnnouncementAttachment struct {
	documents.File `bun:"schema:documents,table:announcement_attachments"`
	AnnouncementID int64 `bun:"announcement_id,notnull" json:"announcement_id"`
}

// GetOwnerID erfüllt documents.Entity: Besitzer eines Anhangs ist seine
// Mitteilung.
func (a *AnnouncementAttachment) GetOwnerID() int64 { return a.AnnouncementID }

// Validate stellt sicher, dass die Zeile speicherbar ist.
func (a *AnnouncementAttachment) Validate() error {
	if a.AnnouncementID <= 0 {
		return errors.New("announcement_id is required")
	}
	if a.Category != AnnouncementAttachmentCategory {
		return errors.New("unknown attachment category")
	}
	return documents.ValidateFile(&a.File)
}

// AnnouncementAttachmentCleanup hält einen Upload fest, dessen Metadaten nicht
// committen konnten, oder dessen Mitteilung gelöscht wurde, bevor die Bytes
// weg waren. Kein Fremdschlüssel auf die Mitteilung: der Cascade darf den
// Intent nicht mitnehmen, sonst bleiben die Bytes für immer liegen.
type AnnouncementAttachmentCleanup struct {
	documents.FileCleanup `bun:"schema:documents,table:announcement_attachment_cleanup"`
}

// AnnouncementAttachmentRepository persistiert die Metadaten der Anhänge. Es
// ist das gemeinsame Dokument-Repository (Soft Delete, Cleanup-Intents) über
// den Tabellen dieser Mitteilungs-Anhänge.
type AnnouncementAttachmentRepository interface {
	Create(ctx context.Context, attachment *AnnouncementAttachment) error
	FindForOwner(ctx context.Context, announcementID, attachmentID int64) (*AnnouncementAttachment, error)
	FindForOwnerIncludingDeleted(ctx context.Context, announcementID, attachmentID int64) (*AnnouncementAttachment, error)
	ListByOwnerID(ctx context.Context, announcementID int64, categories []string) ([]*AnnouncementAttachment, error)
	// ListPendingFileCleanupByOwnerID liefert die Anhänge einer Mitteilung,
	// deren Bytes noch liegen — inklusive soft-gelöschter Zeilen. Das ist die
	// Liste, für die vor dem Löschen der Mitteilung Intents geschrieben werden.
	ListPendingFileCleanupByOwnerID(ctx context.Context, announcementID int64) ([]*AnnouncementAttachment, error)
	ListDeletedPendingFileCleanups(ctx context.Context) ([]*AnnouncementAttachment, error)
	SoftDelete(ctx context.Context, attachment *AnnouncementAttachment, deletedBy int64) error
	MarkFileDeleted(ctx context.Context, attachmentID int64) error
	QueueFileCleanup(ctx context.Context, cleanup *AnnouncementAttachmentCleanup) error
	ListQueuedFileCleanups(ctx context.Context) ([]*AnnouncementAttachmentCleanup, error)
	MarkQueuedFileCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedFileCleanupCompleteByFilename(ctx context.Context, filename string) error
	ActivateQueuedFileCleanupByFilename(ctx context.Context, filename string) error
	// CountByOwnerID zählt die lebenden Anhänge einer Mitteilung, damit der
	// Dienst die Obergrenze prüfen kann, ohne die Zeilen zu laden.
	CountByOwnerID(ctx context.Context, announcementID int64) (int, error)
}
