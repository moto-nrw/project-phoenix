package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// Anhänge an Elternmitteilungen (#2890).
//
// Die Datei gehört diesem Modul, der Empfängerkreis der Mitteilung. Damit das
// keine Abhängigkeit in die falsche Richtung wird, fragt dieses Paket die
// Mitteilung über zwei Ports statt sie zu importieren: Announcements für die
// Personalseite (existiert die Mitteilung, ist sie noch ein Entwurf) und
// GuardianAudience für die Elternseite (ist dieses Konto im Empfängerkreis).
//
// Ein Konto außerhalb des Empfängerkreises bekommt bewusst 404 und nicht 403:
// 403 würde bestätigen, dass es die Mitteilung gibt, und ließe damit
// Mitteilungs-IDs über Schulgrenzen hinweg durchprobieren.

// AuthorizeAttachmentUpload answers "may this caller attach a file to this
// announcement" WITHOUT writing anything.
func (s *Service) AuthorizeAttachmentUpload(ctx context.Context, announcementID int64) error {
	return s.run("authorize_attachment_upload", func(o *op) error {
		return s.write(ctx, o, func(txCtx context.Context, o *op) error {
			return s.checkAttachmentUpload(txCtx, o, announcementID, false)
		})
	})
}

// checkAttachmentUpload is the upload gate. Without lock it reads without
// locking, because it decides nothing; with lock it takes the announcement's
// row lock, so the limit count and the draft state still hold at commit time.
func (s *Service) checkAttachmentUpload(ctx context.Context, o *op, announcementID int64, lock bool) error {
	if announcementID <= 0 {
		return fmt.Errorf("%w: announcement id is required", domain.ErrInvalid)
	}
	if err := s.requireEditableAnnouncement(ctx, announcementID, lock); err != nil {
		return err
	}
	count, stats, err := s.deps.Attachments.CountByOwner(ctx, announcementID)
	o.add(stats)
	if err != nil {
		return err
	}
	if count >= domain.MaxAnnouncementAttachments {
		return domain.ErrAttachmentLimitReached
	}
	return nil
}

// requireEditableAnnouncement resolves "exists" and "editable" and turns them
// into the caller's error. Nicht gefunden und veröffentlicht sind für den
// Aufrufer verschieden: „gibt es nicht" ist 404, „ist schon raus" ist 409.
func (s *Service) requireEditableAnnouncement(ctx context.Context, announcementID int64, lock bool) error {
	if s.deps.Announcements == nil {
		return errors.New("announcement guard is not wired; refusing attachment change")
	}
	var (
		exists, editable bool
		err              error
	)
	if lock {
		exists, editable, err = s.deps.Announcements.LockAnnouncementForAttachmentChange(ctx, announcementID)
		if err != nil {
			return err
		}
	} else {
		editable, err = s.deps.Announcements.AnnouncementEditable(ctx, announcementID)
		if err != nil {
			return err
		}
		exists = editable
		if !editable {
			if exists, err = s.deps.Announcements.AnnouncementExists(ctx, announcementID); err != nil {
				return err
			}
		}
	}
	if editable {
		return nil
	}
	if !exists {
		return domain.ErrAttachmentNotFound
	}
	return domain.ErrAttachmentPublished
}

// requireAnnouncement refuses a read for an announcement the current tenant
// does not have.
func (s *Service) requireAnnouncement(ctx context.Context, announcementID int64) error {
	if s.deps.Announcements == nil {
		return errors.New("announcement guard is not wired; refusing attachment read")
	}
	exists, err := s.deps.Announcements.AnnouncementExists(ctx, announcementID)
	if err != nil {
		return err
	}
	if !exists {
		return domain.ErrAttachmentNotFound
	}
	return nil
}

// UploadAttachment stores one attachment: intent, object, metadata, settle.
func (s *Service) UploadAttachment(ctx context.Context, announcementID int64, upload domain.Upload, actor domain.Actor) (attachment domain.Document, err error) {
	err = s.run("upload_attachment", func(o *op) error {
		tenantID := s.deps.Tx.TenantID(ctx)
		if tenantID <= 0 {
			return fmt.Errorf("%w: no tenant context", domain.ErrInvalid)
		}
		if err := requireActor(actor); err != nil {
			return err
		}
		upload.FilenameDisplay = strings.TrimSpace(upload.FilenameDisplay)
		if upload.FilenameDisplay == "" {
			return fmt.Errorf("%w: filename is required", domain.ErrInvalid)
		}
		if err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
			return s.checkAttachmentUpload(txCtx, o, announcementID, false)
		}); err != nil {
			return err
		}
		storedName, err := s.deps.StoredNames(upload.Extension)
		if err != nil {
			return err
		}
		k := s.attachments()
		if err := s.queueIntent(ctx, k, announcementID, storedName, s.deps.Now().Add(domain.CleanupDelay)); err != nil {
			s.deps.Logger.Error("announcement attachment cleanup intent failed",
				"announcement_id", announcementID,
				"error", err)
			return err
		}
		uploadCtx, cancel := context.WithTimeout(ctx, domain.UploadDeadline)
		defer cancel()
		size, err := s.storeObject(ctx, uploadCtx, k, tenantID, announcementID, storedName, upload)
		if err != nil {
			return err
		}
		attachment, err = s.createAttachment(uploadCtx, o, announcementID, storedName, size, upload, actor)
		if err != nil {
			rejectedBeforeCommit := errors.Is(err, domain.ErrInvalid) ||
				errors.Is(err, domain.ErrAttachmentNotFound) ||
				errors.Is(err, domain.ErrAttachmentPublished) ||
				errors.Is(err, domain.ErrAttachmentLimitReached)
			s.releaseFailedUpload(ctx, k, tenantID, announcementID, storedName, rejectedBeforeCommit, err)
			return err
		}
		return nil
	})
	return attachment, err
}

func (s *Service) createAttachment(ctx context.Context, o *op, announcementID int64, storedName string, size int64, upload domain.Upload, actor domain.Actor) (attachment domain.Document, err error) {
	err = s.write(ctx, o, func(txCtx context.Context, o *op) error {
		// Die Prüfung wiederholt sich innerhalb der Transaktion, diesmal unter
		// der Zeilensperre der Mitteilung: zwischen der Vorabprüfung und hier
		// kann die Mitteilung veröffentlicht worden sein, und ein zweiter
		// Upload kann zeitgleich denselben freien Platz sehen.
		if err := s.checkAttachmentUpload(txCtx, o, announcementID, true); err != nil {
			return err
		}
		document := domain.NewDocument{
			OwnerID: announcementID, Category: domain.AttachmentCategory,
			FilenameDisplay: upload.FilenameDisplay, FilenameStored: storedName,
			SizeBytes: size, ContentType: upload.ContentType, UploadedBy: actor.AccountID,
		}
		if err := document.Validate(); err != nil {
			return fmt.Errorf("%w: %s", domain.ErrInvalid, err.Error())
		}
		created, stats, err := s.deps.Attachments.Create(txCtx, document)
		o.add(stats)
		if err != nil {
			return err
		}
		stats, err = s.deps.AttachmentCleanups.MarkCompleteByFilename(txCtx, storedName)
		o.add(stats)
		if err != nil {
			return fmt.Errorf("complete attachment upload cleanup intent: %w", err)
		}
		if err := s.deps.Announcements.ResetAnnouncementEngagement(txCtx, announcementID); err != nil {
			return err
		}
		attachment = created
		return s.record(txCtx, actor, ports.EventAttachmentUploaded, nil, &announcementID, &created.ID,
			fmt.Sprintf("Anhang „%s“ an Elternmitteilung gehängt (%d Bytes)", created.FilenameDisplay, created.SizeBytes))
	})
	return attachment, err
}

// ListAttachments returns the live attachments of an announcement for a staff
// caller, oldest first, and whether the announcement may still be changed.
//
// Nur die fachlichen Absagen bedeuten „nicht änderbar"; ein Datenbank- oder
// Transaktionsfehler darf nicht als editable: false durchgehen. Die erreichte
// Höchstzahl ist ausdrücklich KEINE solche Absage: der Entwurf bleibt
// änderbar, nur eine weitere Datei passt nicht mehr dazu.
func (s *Service) ListAttachments(ctx context.Context, announcementID int64) (attachments []domain.Document, editable bool, err error) {
	err = s.run("list_attachments", func(o *op) error {
		if announcementID <= 0 {
			return fmt.Errorf("%w: announcement id is required", domain.ErrInvalid)
		}
		if err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
			if err := s.requireAnnouncement(txCtx, announcementID); err != nil {
				return err
			}
			found, stats, err := s.deps.Attachments.ListByOwner(txCtx, announcementID)
			o.add(stats)
			if err != nil {
				return err
			}
			attachments = found
			return nil
		}); err != nil {
			return err
		}
		editable = true
		err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
			return s.checkAttachmentUpload(txCtx, o, announcementID, false)
		})
		switch {
		case err == nil, errors.Is(err, domain.ErrAttachmentLimitReached):
		case errors.Is(err, domain.ErrAttachmentPublished), errors.Is(err, domain.ErrAttachmentNotFound), errors.Is(err, domain.ErrInvalid):
			editable = false
		default:
			return err
		}
		return nil
	})
	return attachments, editable, err
}

// OpenAttachment resolves one attachment for a staff caller and opens its bytes.
func (s *Service) OpenAttachment(ctx context.Context, announcementID, attachmentID int64) (attachment domain.Document, object ports.Object, err error) {
	err = s.run("open_attachment", func(o *op) error {
		if err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
			if err := s.requireAnnouncement(txCtx, announcementID); err != nil {
				return err
			}
			found, ok, stats, err := s.deps.Attachments.FindForOwner(txCtx, announcementID, attachmentID, false)
			o.add(stats)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrAttachmentNotFound
			}
			attachment = found
			return nil
		}); err != nil {
			return err
		}
		var err error
		object, err = s.openObject(ctx, s.attachments(), s.deps.Tx.TenantID(ctx), attachment)
		return err
	})
	return attachment, object, err
}

// DeleteAttachment soft-deletes an attachment of a draft and removes its bytes
// after commit.
func (s *Service) DeleteAttachment(ctx context.Context, announcementID, attachmentID int64, actor domain.Actor) error {
	return s.run("delete_attachment", func(o *op) error {
		if err := requireActor(actor); err != nil {
			return err
		}
		var document domain.Document
		if err := s.write(ctx, o, func(txCtx context.Context, o *op) error {
			if err := s.requireEditableAnnouncement(txCtx, announcementID, true); err != nil {
				return err
			}
			live, found, stats, err := s.deps.Attachments.FindForOwner(txCtx, announcementID, attachmentID, false)
			o.add(stats)
			if err != nil {
				return err
			}
			if !found {
				// Already soft-deleted: retry the byte removal instead of a
				// 404 the caller cannot act on.
				deleted, found, stats, err := s.deps.Attachments.FindForOwner(txCtx, announcementID, attachmentID, true)
				o.add(stats)
				if err != nil {
					return err
				}
				if !found {
					return domain.ErrAttachmentNotFound
				}
				document = deleted
				return nil
			}
			deleted, stats, err := s.deps.Attachments.SoftDelete(txCtx, live.ID, actor.AccountID)
			o.add(stats)
			if err != nil {
				return err
			}
			if err := s.deps.Announcements.ResetAnnouncementEngagement(txCtx, announcementID); err != nil {
				return err
			}
			document = deleted
			return s.record(txCtx, actor, ports.EventAttachmentDeleted, nil, &announcementID, &deleted.ID,
				fmt.Sprintf("Anhang „%s“ von Elternmitteilung entfernt", deleted.FilenameDisplay))
		}); err != nil {
			return err
		}
		if document.BytesPresent() {
			s.scheduleCleanup(ctx, s.attachments(), document, "delete")
		}
		return nil
	})
}

// --- parent side --------------------------------------------------------------

// ListGuardianAttachments returns the attachments a guardian account may see
// and the school they are stored under, or ErrAttachmentNotFound when the
// account is outside the audience.
func (s *Service) ListGuardianAttachments(ctx context.Context, accountID, announcementID int64) (tenantID int64, attachments []domain.Document, err error) {
	err = s.run("list_guardian_attachments", func(o *op) error {
		var err error
		if tenantID, err = s.guardianTenant(ctx, accountID, announcementID); err != nil {
			return err
		}
		if err := s.deps.Tx.RunInTenant(ctx, tenantID, func(txCtx context.Context) error {
			found, stats, err := s.deps.Attachments.ListByOwner(txCtx, announcementID)
			o.add(stats)
			if err != nil {
				return err
			}
			attachments = found
			return nil
		}); err != nil {
			return err
		}
		return s.revalidateGuardianAccess(ctx, accountID, announcementID, tenantID)
	})
	return tenantID, attachments, err
}

// OpenGuardianAttachment resolves one attachment for a guardian and opens its
// bytes under the announcement's school.
func (s *Service) OpenGuardianAttachment(ctx context.Context, accountID, announcementID, attachmentID int64) (attachment domain.Document, object ports.Object, err error) {
	err = s.run("open_guardian_attachment", func(o *op) error {
		tenantID, err := s.guardianTenant(ctx, accountID, announcementID)
		if err != nil {
			return err
		}
		if err := s.deps.Tx.RunInTenant(ctx, tenantID, func(txCtx context.Context) error {
			found, ok, stats, err := s.deps.Attachments.FindForOwner(txCtx, announcementID, attachmentID, false)
			o.add(stats)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrAttachmentNotFound
			}
			attachment = found
			return nil
		}); err != nil {
			return err
		}
		if err := s.revalidateGuardianAccess(ctx, accountID, announcementID, tenantID); err != nil {
			return err
		}
		object, err = s.openObject(ctx, s.attachments(), tenantID, attachment)
		return err
	})
	return attachment, object, err
}

// revalidateGuardianAccess asks the audience a second time, after the rows
// have been read and before anything is handed out. Die erste Prüfung und das
// Lesen laufen in getrennten Transaktionen; zwischen beiden kann die Mitteilung
// zurückgezogen worden sein. Eine abweichende Schule wird wie „nicht sichtbar"
// behandelt.
func (s *Service) revalidateGuardianAccess(ctx context.Context, accountID, announcementID, expectedTenantID int64) error {
	tenantID, err := s.guardianTenant(ctx, accountID, announcementID)
	if err != nil {
		return err
	}
	if tenantID != expectedTenantID {
		return domain.ErrAttachmentNotFound
	}
	return nil
}

// guardianTenant resolves the announcement's school after the audience check.
func (s *Service) guardianTenant(ctx context.Context, accountID, announcementID int64) (int64, error) {
	if accountID <= 0 || announcementID <= 0 {
		return 0, domain.ErrAttachmentNotFound
	}
	if s.deps.GuardianAudience == nil {
		return 0, errors.New("announcement audience is not wired; refusing attachment read")
	}
	tenantID, err := s.deps.GuardianAudience.GuardianAnnouncementTenant(ctx, accountID, announcementID)
	if err != nil {
		return 0, err
	}
	if tenantID <= 0 {
		return 0, domain.ErrAttachmentNotFound
	}
	return tenantID, nil
}

// --- announcement-side promises ------------------------------------------------

// QueueAttachmentCleanupForAnnouncement writes an intent for every attachment
// of an announcement whose bytes are still on disk. It runs inside the
// caller's transaction on purpose: it must commit together with the deletion
// it precedes. Sofort fällig statt nach CleanupDelay: hier läuft kein Upload
// mehr, auf den der Sweep Rücksicht nehmen müsste.
func (s *Service) QueueAttachmentCleanupForAnnouncement(ctx context.Context, announcementID int64) error {
	return s.run("queue_attachment_cleanup_for_announcement", func(o *op) error {
		if announcementID <= 0 {
			return fmt.Errorf("%w: announcement id is required", domain.ErrInvalid)
		}
		pending, stats, err := s.deps.Attachments.ListPendingCleanupByOwner(ctx, announcementID)
		o.add(stats)
		if err != nil {
			return err
		}
		now := s.deps.Now()
		for _, attachment := range pending {
			stats, err := s.deps.AttachmentCleanups.Queue(ctx, announcementID, attachment.FilenameStored, now)
			o.add(stats)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// CountAttachments reports the live attachments of an announcement in the
// caller's transaction; the announcement service asks at publish time.
func (s *Service) CountAttachments(ctx context.Context, announcementID int64) (count int, err error) {
	err = s.run("count_attachments", func(o *op) error {
		if announcementID <= 0 {
			return fmt.Errorf("%w: announcement id is required", domain.ErrInvalid)
		}
		var stats domain.OperationStats
		var err error
		count, stats, err = s.deps.Attachments.CountByOwner(ctx, announcementID)
		o.add(stats)
		return err
	})
	return count, err
}
