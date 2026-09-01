package filestore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/filestore"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Anhänge an Elternmitteilungen (#2890).
//
// Die Datei gehört diesem Modul, der Empfängerkreis der Mitteilung. Damit das
// keine Abhängigkeit in die falsche Richtung wird, fragt dieses Paket die
// Mitteilung über zwei Ports statt sie zu importieren: AnnouncementGuard für
// die Personalseite (existiert die Mitteilung, ist sie noch ein Entwurf) und
// AnnouncementAudience für die Elternseite (ist dieses Konto im
// Empfängerkreis). Verdrahtet werden beide im Kompositions-Wurzelpunkt
// (services.Factory).
//
// Warum der Empfängerkreis nicht hier nachgebaut wird: er existiert an
// users.parent_announcements bereits vollständig — Schule, Klasse, Gruppe,
// Angebot, einzelnes Kind, samt Auflösung über die Sorgeberechtigten-
// Berechtigungen. Ein zweiter Auflöser hätte nur eine Chance: irgendwann
// anders zu entscheiden als der erste.

// ErrAttachmentNotFound marks a missing attachment or an announcement the
// caller may not see — HTTP 404.
//
// Ein Konto außerhalb des Empfängerkreises bekommt bewusst 404 und nicht 403:
// 403 würde bestätigen, dass es die Mitteilung gibt, und ließe damit
// Mitteilungs-IDs über Schulgrenzen hinweg durchprobieren.
var ErrAttachmentNotFound = errors.New("announcement attachment not found")

// ErrAttachmentAnnouncementPublished marks an attempt to change the
// attachments of an already published announcement — HTTP 409.
var ErrAttachmentAnnouncementPublished = errors.New("announcement is published; attachments are fixed")

// ErrAttachmentLimitReached marks an upload past MaxAnnouncementAttachments —
// HTTP 409.
var ErrAttachmentLimitReached = errors.New("announcement attachment limit reached")

// AnnouncementGuard is the staff-side question this package cannot answer:
// does the announcement exist in the current tenant, and may its content
// still be changed. Implemented by services/announcement.
type AnnouncementGuard interface {
	// AnnouncementExists returns ErrAttachmentNotFound-shaped nil/false when
	// the announcement is absent from the current tenant.
	AnnouncementExists(ctx context.Context, announcementID int64) (bool, error)
	// AnnouncementEditable reports whether the announcement is still a draft.
	// A published announcement is immutable — the correction path is
	// unpublish, edit, republish.
	AnnouncementEditable(ctx context.Context, announcementID int64) (bool, error)
	// LockAnnouncementForAttachmentChange answers the same two questions
	// (exists, editable) while holding the announcement's row lock until the
	// caller's transaction ends. The write paths use it instead of the two
	// unlocked reads: check and write must not be separable, or two concurrent
	// uploads both pass the limit check and a publish can slip between the
	// check and the INSERT.
	LockAnnouncementForAttachmentChange(ctx context.Context, announcementID int64) (bool, bool, error)
	// ResetAnnouncementEngagement drops the read and acknowledgement rows of a
	// draft. Adding or removing an attachment changes what the parents are
	// asked to confirm, so a confirmation collected before the change must not
	// survive it — the same rule the body edit already follows.
	ResetAnnouncementEngagement(ctx context.Context, announcementID int64) error
}

// AnnouncementAudience is the parent-side question: is this guardian account
// in the audience of this announcement right now, and if so, which school does
// the announcement belong to. Implemented by services/parent, which owns the
// live/published/feature-flag checks as well.
//
// It returns the tenant id rather than a bare bool because the parents portal
// is cross-tenant: only the announcement itself knows which school's rows the
// attachment lookup may read.
//
// A caller outside the audience is answered with tenant id 0 and NO error —
// "not for you" is not a failure, and this package turns it into
// ErrAttachmentNotFound so the two are indistinguishable from outside.
type AnnouncementAudience interface {
	GuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64) (int64, error)
}

// CreateAttachmentInput carries the metadata of an already-stored upload.
type CreateAttachmentInput struct {
	AnnouncementID  int64
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
}

// AttachmentCleanupStore is the subset the storage coordinator needs to settle
// cleanup bookkeeping for attachments. It is a separate value from the
// service itself because the attachments live in their own tables: reusing the
// folder-file store here would settle the wrong intents.
type AttachmentCleanupStore interface {
	MarkFileDeleted(ctx context.Context, attachmentID int64) error
	MarkQueuedCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error
	ActivateQueuedCleanup(ctx context.Context, storedName string) error
}

// AttachmentService is the business boundary of the announcement attachments.
type AttachmentService interface {
	// AuthorizeAttachmentUpload answers "may this caller attach a file to this
	// announcement" WITHOUT writing anything, so a rejected request costs
	// nothing but the request itself.
	AuthorizeAttachmentUpload(ctx context.Context, announcementID int64) error
	CreateAttachment(ctx context.Context, input CreateAttachmentInput, actor Actor) (*filestore.AnnouncementAttachment, error)
	// ListAttachments returns the live attachments of an announcement for a
	// staff caller, oldest first.
	ListAttachments(ctx context.Context, announcementID int64) ([]*filestore.AnnouncementAttachment, error)
	ResolveAttachmentDownload(ctx context.Context, announcementID, attachmentID int64) (*filestore.AnnouncementAttachment, error)
	DeleteAttachment(ctx context.Context, announcementID, attachmentID int64, actor Actor) (*filestore.AnnouncementAttachment, error)
	ResolveAttachmentCleanup(ctx context.Context, announcementID, attachmentID int64) (*filestore.AnnouncementAttachment, error)

	// ListAttachmentsForGuardian returns the attachments a guardian account may
	// see, or ErrAttachmentNotFound when it is outside the audience.
	ListAttachmentsForGuardian(ctx context.Context, accountID, announcementID int64) (int64, []*filestore.AnnouncementAttachment, error)
	// ResolveGuardianAttachmentDownload resolves one attachment for a guardian
	// and reports the tenant its bytes are stored under.
	ResolveGuardianAttachmentDownload(ctx context.Context, accountID, announcementID, attachmentID int64) (int64, *filestore.AnnouncementAttachment, error)

	// QueueAttachmentCleanup records the intent to remove one object before it
	// is written.
	QueueAttachmentCleanup(ctx context.Context, announcementID int64, storedName string) error
	// QueueAttachmentCleanupForAnnouncement writes an intent for every
	// attachment of an announcement whose bytes are still on disk. The
	// announcement service calls it inside its delete transaction, BEFORE the
	// rows cascade away: afterwards nothing points at the bytes any more.
	QueueAttachmentCleanupForAnnouncement(ctx context.Context, announcementID int64) error
	// CountAttachments reports the live attachments of an announcement. The
	// announcement service asks at publish time so the e-mail can say that a
	// file is waiting in the portal.
	CountAttachments(ctx context.Context, announcementID int64) (int, error)
	ListDeletedAttachmentsPendingCleanups(ctx context.Context) ([]*filestore.AnnouncementAttachment, error)
	ListQueuedAttachmentCleanups(ctx context.Context) ([]*filestore.AnnouncementAttachmentCleanup, error)

	// AttachmentStore exposes the coordinator bookkeeping for attachments.
	AttachmentStore() AttachmentCleanupStore
	MarkAttachmentFileDeleted(ctx context.Context, attachmentID int64) error
	MarkQueuedAttachmentCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedAttachmentCleanupCompleteByFilename(ctx context.Context, storedName string) error
	ActivateQueuedAttachmentCleanup(ctx context.Context, storedName string) error
}

// --- staff side ---------------------------------------------------------------

func (s *service) AuthorizeAttachmentUpload(ctx context.Context, announcementID int64) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.requireAttachmentUpload(ctx, announcementID)
	})
}

// requireAttachmentUpload is the pre-flight check of AuthorizeAttachmentUpload:
// it reads without locking, because it decides nothing — it only spares the
// caller an upload that would be rejected anyway.
func (s *service) requireAttachmentUpload(ctx context.Context, announcementID int64) error {
	return s.checkAttachmentUpload(ctx, announcementID, false)
}

// requireAttachmentUploadLocked is the binding check inside the write
// transaction: it takes the announcement's row lock, so the limit count and the
// draft state it reads still hold when the INSERT below commits.
func (s *service) requireAttachmentUploadLocked(ctx context.Context, announcementID int64) error {
	return s.checkAttachmentUpload(ctx, announcementID, true)
}

func (s *service) checkAttachmentUpload(ctx context.Context, announcementID int64, lock bool) error {
	if announcementID <= 0 {
		return fmt.Errorf("%w: announcement id is required", ErrInvalid)
	}
	if err := s.requireEditableAnnouncementState(ctx, announcementID, lock); err != nil {
		return err
	}
	count, err := s.attachments.CountByOwnerID(ctx, announcementID)
	if err != nil {
		return err
	}
	if count >= filestore.MaxAnnouncementAttachments {
		return ErrAttachmentLimitReached
	}
	return nil
}

func (s *service) CreateAttachment(ctx context.Context, input CreateAttachmentInput, actor Actor) (*filestore.AnnouncementAttachment, error) {
	if s.events == nil {
		return nil, errors.New("file event repository is not wired; refusing unaudited change")
	}
	if actor.AccountID <= 0 {
		return nil, fmt.Errorf("%w: actor account id is required", ErrInvalid)
	}
	input.FilenameDisplay = strings.TrimSpace(input.FilenameDisplay)
	if input.FilenameDisplay == "" {
		return nil, fmt.Errorf("%w: filename is required", ErrInvalid)
	}

	attachment := &filestore.AnnouncementAttachment{AnnouncementID: input.AnnouncementID}
	attachment.Category = filestore.AnnouncementAttachmentCategory
	attachment.FilenameDisplay = input.FilenameDisplay
	attachment.FilenameStored = input.FilenameStored
	attachment.SizeBytes = input.SizeBytes
	attachment.ContentType = input.ContentType
	attachment.UploadedBy = actor.AccountID
	if err := attachment.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}

	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Die Prüfung wiederholt sich innerhalb der Transaktion, diesmal unter
		// der Zeilensperre der Mitteilung: zwischen der Vorabprüfung und hier
		// kann die Mitteilung veröffentlicht worden sein, und ein zweiter
		// Upload kann zeitgleich denselben freien Platz sehen.
		if err := s.requireAttachmentUploadLocked(ctx, input.AnnouncementID); err != nil {
			return err
		}
		if err := s.attachments.Create(ctx, attachment); err != nil {
			return err
		}
		if err := s.attachments.MarkQueuedFileCleanupCompleteByFilename(ctx, input.FilenameStored); err != nil {
			return fmt.Errorf("complete attachment upload cleanup intent: %w", err)
		}
		if err := s.announcements.ResetAnnouncementEngagement(ctx, input.AnnouncementID); err != nil {
			return err
		}
		return s.recordAttachmentEvent(ctx, actor, auditModels.FileEventAnnouncementAttachmentUploaded,
			input.AnnouncementID, attachment.ID,
			fmt.Sprintf("Anhang „%s“ an Elternmitteilung gehängt (%d Bytes)", attachment.FilenameDisplay, attachment.SizeBytes))
	})
	if err != nil {
		return nil, err
	}
	return attachment, nil
}

func (s *service) ListAttachments(ctx context.Context, announcementID int64) ([]*filestore.AnnouncementAttachment, error) {
	if announcementID <= 0 {
		return nil, fmt.Errorf("%w: announcement id is required", ErrInvalid)
	}
	var rows []*filestore.AnnouncementAttachment
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.requireAnnouncement(ctx, announcementID); err != nil {
			return err
		}
		found, listErr := s.attachments.ListByOwnerID(ctx, announcementID, []string{filestore.AnnouncementAttachmentCategory})
		if listErr != nil {
			return listErr
		}
		rows = found
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *service) ResolveAttachmentDownload(ctx context.Context, announcementID, attachmentID int64) (*filestore.AnnouncementAttachment, error) {
	var attachment *filestore.AnnouncementAttachment
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.requireAnnouncement(ctx, announcementID); err != nil {
			return err
		}
		found, findErr := s.attachments.FindForOwner(ctx, announcementID, attachmentID)
		if findErr != nil {
			return findErr
		}
		attachment = found
		return nil
	})
	if err != nil {
		return nil, err
	}
	return attachment, nil
}

// requireAnnouncement refuses a read for an announcement the current tenant
// does not have.
func (s *service) requireAnnouncement(ctx context.Context, announcementID int64) error {
	if s.announcements == nil {
		return errors.New("announcement guard is not wired; refusing attachment read")
	}
	exists, err := s.announcements.AnnouncementExists(ctx, announcementID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrAttachmentNotFound
	}
	return nil
}

func (s *service) DeleteAttachment(ctx context.Context, announcementID, attachmentID int64, actor Actor) (*filestore.AnnouncementAttachment, error) {
	if s.events == nil {
		return nil, errors.New("file event repository is not wired; refusing unaudited change")
	}
	if actor.AccountID <= 0 {
		return nil, fmt.Errorf("%w: actor account id is required", ErrInvalid)
	}
	var deleted *filestore.AnnouncementAttachment
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.requireEditableAnnouncement(ctx, announcementID); err != nil {
			return err
		}
		attachment, err := s.attachments.FindForOwner(ctx, announcementID, attachmentID)
		if err != nil {
			return err
		}
		if err := s.attachments.SoftDelete(ctx, attachment, actor.AccountID); err != nil {
			return err
		}
		if err := s.announcements.ResetAnnouncementEngagement(ctx, announcementID); err != nil {
			return err
		}
		deleted = attachment
		return s.recordAttachmentEvent(ctx, actor, auditModels.FileEventAnnouncementAttachmentDeleted,
			announcementID, attachment.ID,
			fmt.Sprintf("Anhang „%s“ von Elternmitteilung entfernt", attachment.FilenameDisplay))
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func (s *service) ResolveAttachmentCleanup(ctx context.Context, announcementID, attachmentID int64) (*filestore.AnnouncementAttachment, error) {
	return s.attachments.FindForOwnerIncludingDeleted(ctx, announcementID, attachmentID)
}

// requireEditableAnnouncement is the shared draft check of the write paths. It
// always locks: every caller writes afterwards in the same transaction.
func (s *service) requireEditableAnnouncement(ctx context.Context, announcementID int64) error {
	return s.requireEditableAnnouncementState(ctx, announcementID, true)
}

// requireEditableAnnouncementState resolves "exists" and "editable" and turns
// them into the caller's error. With lock set, both facts are read while
// holding the announcement's row lock and therefore still hold at commit time.
func (s *service) requireEditableAnnouncementState(ctx context.Context, announcementID int64, lock bool) error {
	if s.announcements == nil {
		return errors.New("announcement guard is not wired; refusing attachment change")
	}
	var (
		exists, editable bool
		err              error
	)
	if lock {
		exists, editable, err = s.announcements.LockAnnouncementForAttachmentChange(ctx, announcementID)
		if err != nil {
			return err
		}
	} else {
		editable, err = s.announcements.AnnouncementEditable(ctx, announcementID)
		if err != nil {
			return err
		}
		exists = editable
		if !editable {
			if exists, err = s.announcements.AnnouncementExists(ctx, announcementID); err != nil {
				return err
			}
		}
	}
	if editable {
		return nil
	}
	// Nicht gefunden und veröffentlicht sind für den Aufrufer verschieden:
	// "gibt es nicht" ist 404, "ist schon raus" ist 409 mit einer Meldung, aus
	// der hervorgeht, was zu tun ist.
	if !exists {
		return ErrAttachmentNotFound
	}
	return ErrAttachmentAnnouncementPublished
}

// --- parent side --------------------------------------------------------------

func (s *service) ListAttachmentsForGuardian(ctx context.Context, accountID, announcementID int64) (int64, []*filestore.AnnouncementAttachment, error) {
	tenantID, err := s.guardianTenant(ctx, accountID, announcementID)
	if err != nil {
		return 0, nil, err
	}
	var rows []*filestore.AnnouncementAttachment
	err = tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		found, listErr := s.attachments.ListByOwnerID(ctx, announcementID, []string{filestore.AnnouncementAttachmentCategory})
		if listErr != nil {
			return listErr
		}
		rows = found
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	if err := s.revalidateGuardianAccess(ctx, accountID, announcementID, tenantID); err != nil {
		return 0, nil, err
	}
	return tenantID, rows, nil
}

func (s *service) ResolveGuardianAttachmentDownload(ctx context.Context, accountID, announcementID, attachmentID int64) (int64, *filestore.AnnouncementAttachment, error) {
	tenantID, err := s.guardianTenant(ctx, accountID, announcementID)
	if err != nil {
		return 0, nil, err
	}
	var attachment *filestore.AnnouncementAttachment
	err = tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		found, findErr := s.attachments.FindForOwner(ctx, announcementID, attachmentID)
		if findErr != nil {
			return findErr
		}
		attachment = found
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	if err := s.revalidateGuardianAccess(ctx, accountID, announcementID, tenantID); err != nil {
		return 0, nil, err
	}
	return tenantID, attachment, nil
}

// revalidateGuardianAccess asks the audience a second time, after the rows have
// been read and before anything is handed out.
//
// Die erste Prüfung und das Lesen laufen in getrennten Transaktionen — zwischen
// beiden kann die Mitteilung zurückgezogen worden, abgelaufen oder die Funktion
// der Schule abgeschaltet worden sein. Die Antwort hängt deshalb an der
// späteren Prüfung: was danach committet, kann keine Prüfung mehr einholen, aber
// nichts geht mehr raus, das zum Zeitpunkt der Antwort schon entzogen war.
//
// Eine abweichende Schule wird wie „nicht sichtbar" behandelt: dieselbe Zeile
// unter einer anderen Schule wäre eine Verwechslung, keine Berechtigung.
func (s *service) revalidateGuardianAccess(ctx context.Context, accountID, announcementID, expectedTenantID int64) error {
	tenantID, err := s.guardianTenant(ctx, accountID, announcementID)
	if err != nil {
		return err
	}
	if tenantID != expectedTenantID {
		return ErrAttachmentNotFound
	}
	return nil
}

// guardianTenant resolves the announcement's school after the audience check.
// The tenant transaction that follows is opened for exactly that school, so a
// guardian of several schools can never read across them.
func (s *service) guardianTenant(ctx context.Context, accountID, announcementID int64) (int64, error) {
	if accountID <= 0 || announcementID <= 0 {
		return 0, ErrAttachmentNotFound
	}
	if s.audience == nil {
		return 0, errors.New("announcement audience is not wired; refusing attachment read")
	}
	tenantID, err := s.audience.GuardianAnnouncementTenant(ctx, accountID, announcementID)
	if err != nil {
		return 0, err
	}
	if tenantID <= 0 {
		return 0, ErrAttachmentNotFound
	}
	return tenantID, nil
}

// --- cleanup ------------------------------------------------------------------

func (s *service) QueueAttachmentCleanup(ctx context.Context, announcementID int64, storedName string) error {
	if announcementID <= 0 || strings.TrimSpace(storedName) == "" {
		return fmt.Errorf("%w: cleanup attachment details are required", ErrInvalid)
	}
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.queueAttachmentCleanup(ctx, announcementID, storedName, time.Now().Add(cleanupDelay))
	})
}

// QueueAttachmentCleanupForAnnouncement runs inside the caller's transaction
// on purpose: it must commit together with the deletion it precedes. Rolling
// back the delete must roll back the intents too, or the next sweep would
// remove the bytes of an announcement that still exists.
func (s *service) QueueAttachmentCleanupForAnnouncement(ctx context.Context, announcementID int64) error {
	if announcementID <= 0 {
		return fmt.Errorf("%w: announcement id is required", ErrInvalid)
	}
	pending, err := s.attachments.ListPendingFileCleanupByOwnerID(ctx, announcementID)
	if err != nil {
		return err
	}
	// Sofort fällig statt nach cleanupDelay: hier läuft kein Upload mehr, auf
	// den der Sweep Rücksicht nehmen müsste — die Mitteilung ist weg.
	now := time.Now()
	for _, attachment := range pending {
		if err := s.queueAttachmentCleanup(ctx, announcementID, attachment.FilenameStored, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) queueAttachmentCleanup(ctx context.Context, announcementID int64, storedName string, retryAfter time.Time) error {
	cleanup := &filestore.AnnouncementAttachmentCleanup{}
	cleanup.OwnerID = announcementID
	cleanup.FilenameStored = storedName
	cleanup.RetryAfter = retryAfter
	return s.attachments.QueueFileCleanup(ctx, cleanup)
}

// CountAttachments runs in the caller's transaction: the announcement service
// asks inside its publish transaction, where the announcement's tenant is
// already the active one.
func (s *service) CountAttachments(ctx context.Context, announcementID int64) (int, error) {
	if announcementID <= 0 {
		return 0, fmt.Errorf("%w: announcement id is required", ErrInvalid)
	}
	return s.attachments.CountByOwnerID(ctx, announcementID)
}

func (s *service) ListDeletedAttachmentsPendingCleanups(ctx context.Context) ([]*filestore.AnnouncementAttachment, error) {
	return s.attachments.ListDeletedPendingFileCleanups(ctx)
}

func (s *service) ListQueuedAttachmentCleanups(ctx context.Context) ([]*filestore.AnnouncementAttachmentCleanup, error) {
	return s.attachments.ListQueuedFileCleanups(ctx)
}

func (s *service) MarkAttachmentFileDeleted(ctx context.Context, attachmentID int64) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.attachments.MarkFileDeleted(ctx, attachmentID)
	})
}

func (s *service) MarkQueuedAttachmentCleanupComplete(ctx context.Context, cleanupID int64) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.attachments.MarkQueuedFileCleanupComplete(ctx, cleanupID)
	})
}

func (s *service) MarkQueuedAttachmentCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.attachments.MarkQueuedFileCleanupCompleteByFilename(ctx, storedName)
	})
}

func (s *service) ActivateQueuedAttachmentCleanup(ctx context.Context, storedName string) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.attachments.ActivateQueuedFileCleanupByFilename(ctx, storedName)
	})
}

// attachmentStore adapts the attachment half of the service to the storage
// coordinator's bookkeeping interface.
type attachmentStore struct{ s *service }

func (a attachmentStore) MarkFileDeleted(ctx context.Context, attachmentID int64) error {
	return a.s.MarkAttachmentFileDeleted(ctx, attachmentID)
}

func (a attachmentStore) MarkQueuedCleanupComplete(ctx context.Context, cleanupID int64) error {
	return a.s.MarkQueuedAttachmentCleanupComplete(ctx, cleanupID)
}

func (a attachmentStore) MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	return a.s.MarkQueuedAttachmentCleanupCompleteByFilename(ctx, storedName)
}

func (a attachmentStore) ActivateQueuedCleanup(ctx context.Context, storedName string) error {
	return a.s.ActivateQueuedAttachmentCleanup(ctx, storedName)
}

// AttachmentStore returns the coordinator bookkeeping for attachments.
func (s *service) AttachmentStore() AttachmentCleanupStore { return attachmentStore{s: s} }

// --- audit --------------------------------------------------------------------

func (s *service) recordAttachmentEvent(ctx context.Context, actor Actor, action string, announcementID, attachmentID int64, detail string) error {
	if s.events == nil {
		return errors.New("file event repository is not wired; refusing unaudited change")
	}
	name := strings.TrimSpace(actor.Name)
	if name == "" {
		name = "Unbekannt"
	}
	event := &auditModels.FileEvent{
		AnnouncementID: &announcementID,
		FileID:         &attachmentID,
		Action:         action,
		ActorName:      name,
		Detail:         detail,
	}
	if actor.AccountID > 0 {
		id := actor.AccountID
		event.ActorAccountID = &id
	}
	if err := s.events.Create(ctx, event); err != nil {
		return fmt.Errorf("write attachment event: %w", err)
	}
	return nil
}
