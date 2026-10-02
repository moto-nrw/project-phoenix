package announcement

import (
	"context"
	"fmt"
	"io"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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
	// CountAttachments reports how many live attachments an announcement has,
	// so the e-mail can say that a file is waiting in the portal.
	CountAttachments(ctx context.Context, announcementID int64) (int, error)
	// AttachmentDigests returns the SHA-256 of every live attachment; an
	// Erklärung freezes them when it is published (#3430).
	AttachmentDigests(ctx context.Context, announcementID int64, digest func(io.Reader) (string, int64, error)) ([]AttachmentDigest, error)
}

// AttachmentDigest is the content identity of one attachment, the same
// unnamed shape File Storage returns.
type AttachmentDigest = struct {
	AttachmentID int64
	Filename     string
	ContentType  string
	SizeBytes    int64
	SHA256       string
}

// hasAttachments reports whether the announcement carries files, for the
// e-mail hint. A failure here is logged and read as "no attachment": the mail
// is a pointer to the portal either way, and a missing sentence must never
// stop an Elternbrief from going out.
func (s *service) hasAttachments(ctx context.Context, announcementID int64) bool {
	if s.attachments == nil {
		return false
	}
	count, err := s.attachments.CountAttachments(ctx, announcementID)
	if err != nil {
		s.logger.Warn("announcement attachment count failed, mail omits the attachment hint",
			"announcement_id", announcementID,
			"error", err,
		)
		return false
	}
	return count > 0
}

// AttachmentSupport is what the file storage and the composition root need
// from this service to serve announcement attachments (#2890).
type AttachmentSupport interface {
	AnnouncementExists(ctx context.Context, announcementID int64) (bool, error)
	AnnouncementEditable(ctx context.Context, announcementID int64) (bool, error)
	LockAnnouncementForAttachmentChange(ctx context.Context, announcementID int64) (bool, bool, error)
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
	return s.attachmentsEditable(ctx, a)
}

// LockAnnouncementForAttachmentChange locks the announcement row for the rest
// of the caller's transaction and reports, in one read, whether it exists and
// whether it is still editable.
//
// Die Anhang-Schreibpfade prüfen erst („Entwurf? unter der Grenze?") und
// schreiben dann. Beides muss dieselbe Zeile sperren, sonst laufen zwei
// gleichzeitige Uploads beide durch die Prüfung und legen den sechsten Anhang
// an, oder ein Upload committet hinter einer Veröffentlichung, die zwischen
// Prüfung und INSERT durchging. Die Veröffentlichung nimmt dieselbe Zeilensperre
// (sie schreibt published_at), damit sind beide Reihenfolgen serialisiert.
func (s *service) LockAnnouncementForAttachmentChange(ctx context.Context, announcementID int64) (bool, bool, error) {
	a, err := s.repo.FindByIDForUpdate(ctx, announcementID)
	if err != nil {
		return false, false, fmt.Errorf("announcement: lock for attachment change: %w", err)
	}
	if a == nil {
		return false, false, nil
	}
	editable, err := s.attachmentsEditable(ctx, a)
	return true, editable, err
}

// attachmentsEditable: a draft accepts attachment changes, except a
// declaration that was published before (#3430). Its earlier versions name
// these files by digest, and the proof for those versions must still be able
// to produce them. A changed document needs a new Erklärung.
func (s *service) attachmentsEditable(ctx context.Context, a *usersModels.ParentAnnouncement) (bool, error) {
	if a.IsPublished() || a.IsSystem() {
		return false, nil
	}
	if !a.IsDeclaration() {
		return true, nil
	}
	version, err := s.repo.LatestDeclarationVersion(ctx, a.GetTenantID(), a.ID)
	if err != nil {
		return false, fmt.Errorf("announcement: load declaration version for attachment check: %w", err)
	}
	return version == nil, nil
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
