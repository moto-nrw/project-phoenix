package announcement

import (
	"context"
	"fmt"
)

// Anhänge an Elternmitteilungen (#2890) — die Seite, die dieses Paket beiträgt.
//
// Die Datei selbst gehört der Dateiablage (services/filestore): sie kennt
// Storage, Cleanup-Intents und Audit für Dateien. Dieses Paket beantwortet nur
// die Fragen, die es allein beantworten kann — gibt es die Mitteilung, ist sie
// noch ein Entwurf — und sagt vor dem Löschen Bescheid, damit die Bytes nicht
// zurückbleiben.

// AttachmentPurger is the file side's promise to reclaim the bytes of an
// announcement's attachments. Delete calls it INSIDE its own transaction, so
// the intents commit together with the deletion they precede.
//
// Implemented by services/filestore. It stays an interface here so the
// communication module never depends on the file storage.
type AttachmentPurger interface {
	QueueAttachmentCleanupForAnnouncement(ctx context.Context, announcementID int64) error
}

// AttachmentSupport is what the file storage and the composition root need
// from this service to serve announcement attachments (#2890).
type AttachmentSupport interface {
	AnnouncementExists(ctx context.Context, announcementID int64) (bool, error)
	AnnouncementEditable(ctx context.Context, announcementID int64) (bool, error)
	ResetAnnouncementEngagement(ctx context.Context, announcementID int64) error
	SetAttachmentPurger(purger AttachmentPurger)
}

// SetAttachmentPurger injects the file side. A service without one refuses to
// delete an announcement that could still own attachments rather than
// orphaning their bytes silently.
func (s *service) SetAttachmentPurger(purger AttachmentPurger) {
	s.attachments = purger
}

// AnnouncementExists reports whether the announcement exists in the current
// tenant. It is the file side's 404 test: an attachment route must not
// distinguish "no such announcement" from "no such attachment".
func (s *service) AnnouncementExists(ctx context.Context, announcementID int64) (bool, error) {
	a, err := s.repo.FindByID(ctx, announcementID)
	if err != nil {
		return false, fmt.Errorf("announcement: load for attachment check: %w", err)
	}
	return a != nil, nil
}

// AnnouncementEditable reports whether the announcement is still a draft.
//
// A published announcement is immutable — the same rule the body edit follows.
// Attachments are part of what the parents were shown and, for an Elternbrief,
// part of what they confirmed; letting one appear or vanish underneath a
// confirmation would make the confirmation mean nothing. The correction path
// stays: zurückziehen, ändern, erneut veröffentlichen.
//
// A system-authored announcement (#2601) is never editable either: nobody
// authored it, so nobody may attach to it.
func (s *service) AnnouncementEditable(ctx context.Context, announcementID int64) (bool, error) {
	a, err := s.repo.FindByID(ctx, announcementID)
	if err != nil {
		return false, fmt.Errorf("announcement: load for attachment check: %w", err)
	}
	if a == nil {
		return false, nil
	}
	return !a.IsPublished() && !a.IsSystem(), nil
}

// ResetAnnouncementEngagement drops the reads, acknowledgements and poll
// answers of a draft after its attachments changed.
func (s *service) ResetAnnouncementEngagement(ctx context.Context, announcementID int64) error {
	if err := s.repo.ClearEngagement(ctx, announcementID); err != nil {
		return fmt.Errorf("announcement: clear engagement after attachment change: %w", err)
	}
	return nil
}

// purgeAttachments queues the cleanup intents for every attachment whose bytes
// are still stored, before the announcement row (and with it the attachment
// rows) is removed.
func (s *service) purgeAttachments(ctx context.Context, announcementID int64) error {
	if s.attachments == nil {
		return fmt.Errorf("announcement: attachment purger is not wired; refusing to delete announcement %d", announcementID)
	}
	if err := s.attachments.QueueAttachmentCleanupForAnnouncement(ctx, announcementID); err != nil {
		return fmt.Errorf("announcement: queue attachment cleanup: %w", err)
	}
	return nil
}
