// Package application is the business boundary of the school file storage
// (#2596, #2890, migrated under #2707): folder visibility, who may upload and
// delete, the storage quota, the upload protocol that never leaves an orphan
// behind, and the audit trail.
//
// Authority, in one paragraph. Admins and files:manage holders ("managers")
// see every folder, create and change folders, and upload or delete any file.
// Everybody else sees the folders whose visibility admits them, and, only
// while files.staff_upload_enabled is on, may upload into those folders and
// delete their own uploads. Nothing else exists: no per-file rights, no
// delegated folder ownership.
//
// The upload order is forced by HTTP, not by preference. Multipart input
// cannot be replayed after a transaction commits, so the object must be
// written before its metadata exists. The window in which a crash leaves
// bytes with no row is closed by a durable cleanup intent written BEFORE the
// object and settled afterwards:
//
//	queue intent -> write object -> commit metadata -> settle intent
//
// An intent that is never settled becomes eligible for the sweep after a
// delay that is deliberately longer than the upload deadline, so the sweep can
// never delete an upload that is still in flight.
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// Dependencies are the seams composition binds. Announcements and
// GuardianAudience may stay nil in setups that do not serve attachments;
// every path that needs them refuses loudly rather than deciding on its own.
type Dependencies struct {
	Folders            ports.FolderStore
	Files              ports.DocumentStore
	FileCleanups       ports.CleanupStore
	Attachments        ports.DocumentStore
	AttachmentCleanups ports.CleanupStore
	FileObjects        ports.Objects
	AttachmentObjects  ports.Objects
	Identity           ports.Identity
	People             ports.People
	Settings           ports.Settings
	Events             ports.Events
	Announcements      ports.Announcements
	GuardianAudience   ports.GuardianAudience
	Tx                 ports.Transaction
	StoredNames        ports.StoredNames
	Observe            ports.Observer
	Logger             ports.Logger
	Now                func() time.Time
}

// Service implements the file storage over its ports.
type Service struct {
	deps Dependencies
}

// New wires the service. Every persistence, identity and runtime dependency
// is required so a missing production binding fails at startup.
func New(deps Dependencies) *Service {
	if deps.Folders == nil || deps.Files == nil || deps.FileCleanups == nil ||
		deps.Attachments == nil || deps.AttachmentCleanups == nil ||
		deps.FileObjects == nil || deps.AttachmentObjects == nil ||
		deps.Identity == nil || deps.People == nil || deps.Settings == nil ||
		deps.Events == nil || deps.Tx == nil || deps.StoredNames == nil ||
		deps.Observe == nil || deps.Logger == nil || deps.Now == nil {
		panic("file storage application: all dependencies are required")
	}
	return &Service{deps: deps}
}

// op accumulates the persistence stats of one observed operation.
type op struct {
	stats domain.OperationStats
}

func (o *op) add(stats domain.OperationStats) { o.stats.Add(stats) }

// run observes one operation without opening a transaction of its own; the
// stores join the caller's ambient transaction when there is one.
func (s *Service) run(operation string, fn func(*op) error) (err error) {
	started := s.deps.Now()
	o := &op{}
	defer func() {
		if err != nil {
			o.stats.Rows = 0
		}
		s.deps.Observe(ports.Observation{Operation: operation, Duration: s.deps.Now().Sub(started), Stats: o.stats, Err: err})
	}()
	return fn(o)
}

// write joins the caller's transaction or opens one for the tenant in context.
func (s *Service) write(ctx context.Context, o *op, fn func(context.Context, *op) error) error {
	return s.deps.Tx.RunWrite(ctx, func(txCtx context.Context) error { return fn(txCtx, o) })
}

// --- authority ---------------------------------------------------------------

func requireManager(actor domain.Actor) error {
	if !actor.Manager {
		return fmt.Errorf("%w: files:manage required", domain.ErrForbidden)
	}
	return nil
}

func requireActor(actor domain.Actor) error {
	if actor.AccountID <= 0 {
		return fmt.Errorf("%w: actor account id is required", domain.ErrInvalid)
	}
	return nil
}

// viewer resolves the caller against whom folder visibility is decided. A
// caller without an active mapping to the school sees nothing: a token can
// outlive a membership change, so the tenant claim alone is not enough.
func (s *Service) viewer(ctx context.Context, actor domain.Actor) (domain.Viewer, bool, error) {
	active, err := s.deps.Identity.HasActiveMembership(ctx, actor.AccountID)
	if err != nil {
		return domain.Viewer{}, false, err
	}
	if !active {
		return domain.Viewer{}, false, nil
	}
	viewer := domain.Viewer{AccountID: actor.AccountID, Manager: actor.Manager}
	if !actor.Manager {
		if viewer.RoleIDs, err = s.deps.Identity.ListAccountRoleIDs(ctx, actor.AccountID); err != nil {
			return domain.Viewer{}, false, err
		}
	}
	return viewer, true, nil
}

// requireVisible loads the folder and refuses when the actor may not see it.
// A folder the actor cannot see is reported as missing, not forbidden: the
// share list must not be probeable by ID.
func (s *Service) requireVisible(ctx context.Context, o *op, folderID int64, actor domain.Actor) (domain.Folder, error) {
	folder, found, stats, err := s.deps.Folders.FindByID(ctx, folderID)
	o.add(stats)
	if err != nil {
		return domain.Folder{}, err
	}
	if !found {
		return domain.Folder{}, domain.ErrNotFound
	}
	viewer, active, err := s.viewer(ctx, actor)
	if err != nil {
		return domain.Folder{}, err
	}
	if !active {
		return domain.Folder{}, domain.ErrNotFound
	}
	visible, stats, err := s.deps.Folders.IsVisible(ctx, folderID, viewer)
	o.add(stats)
	if err != nil {
		return domain.Folder{}, err
	}
	if !visible {
		return domain.Folder{}, domain.ErrNotFound
	}
	return folder, nil
}

// canUpload answers whether the actor may add files to a folder they can
// already see.
func (s *Service) canUpload(ctx context.Context, actor domain.Actor) (bool, error) {
	if actor.Manager {
		return true, nil
	}
	return s.deps.Settings.StaffUploadEnabled(ctx)
}

// canDelete answers, for an already-listed file, whether the actor may delete
// it: manager, or own upload while staff uploads are enabled.
func (s *Service) canDelete(ctx context.Context, document domain.Document, actor domain.Actor) (bool, error) {
	if actor.Manager {
		return true, nil
	}
	if document.UploadedBy != actor.AccountID {
		return false, nil
	}
	return s.deps.Settings.StaffUploadEnabled(ctx)
}

// --- audit -------------------------------------------------------------------

func (s *Service) record(ctx context.Context, actor domain.Actor, action string, folderID, announcementID, fileID *int64, detail string) error {
	if err := s.deps.Events.Record(ctx, ports.Event{
		FolderID: folderID, AnnouncementID: announcementID, FileID: fileID,
		Action: action, Actor: actor, Detail: detail,
	}); err != nil {
		return fmt.Errorf("write file event: %w", err)
	}
	return nil
}

// --- object bookkeeping shared by files and attachments -----------------------

// kind bundles the stores and object prefix of one document family.
type kind struct {
	name     string
	store    ports.DocumentStore
	cleanups ports.CleanupStore
	objects  ports.Objects
}

func (s *Service) files() kind {
	return kind{name: "files", store: s.deps.Files, cleanups: s.deps.FileCleanups, objects: s.deps.FileObjects}
}

func (s *Service) attachments() kind {
	return kind{name: "announcement-attachments", store: s.deps.Attachments, cleanups: s.deps.AttachmentCleanups, objects: s.deps.AttachmentObjects}
}

// scheduleCleanup removes a soft-deleted document's bytes after the
// surrounding transaction committed. Failures are logged, never propagated:
// by then the request has been answered.
func (s *Service) scheduleCleanup(ctx context.Context, k kind, document domain.Document, source string) {
	tenantID := s.deps.Tx.TenantID(ctx)
	detached := s.deps.Tx.Detach(ctx)
	s.deps.Tx.AfterCommit(ctx, func() {
		s.cleanupDocument(detached, k, tenantID, document, source)
	})
}

func (s *Service) cleanupDocument(ctx context.Context, k kind, tenantID int64, document domain.Document, source string) {
	if err := k.objects.Remove(ctx, tenantID, document.FilenameStored); err != nil {
		s.deps.Logger.Warn("document cleanup failed",
			"kind", k.name,
			"owner_id", document.OwnerID,
			"document_id", document.ID,
			"source", source,
			"error", err,
		)
		return
	}
	err := s.deps.Tx.RunWrite(ctx, func(txCtx context.Context) error {
		_, err := k.store.MarkBytesDeleted(txCtx, document.ID)
		return err
	})
	if err != nil {
		s.deps.Logger.Error("document cleanup status update failed",
			"kind", k.name,
			"owner_id", document.OwnerID,
			"document_id", document.ID,
			"source", source,
			"error", err,
		)
	}
}

// storeObject writes the upload bytes within the upload deadline. On a failed
// or late write the queued intent is activated rather than settled: a
// half-written object must stay reachable for the sweep.
func (s *Service) storeObject(ctx, uploadCtx context.Context, k kind, tenantID, ownerID int64, storedName string, upload domain.Upload) (int64, error) {
	size, err := k.objects.Save(uploadCtx, tenantID, storedName, upload.Content)
	if err == nil && uploadCtx.Err() != nil {
		err = errors.New("upload exceeded its deadline")
	}
	if err == nil {
		return size, nil
	}
	activateErr := s.deps.Tx.RunWrite(ctx, func(txCtx context.Context) error {
		_, err := k.cleanups.ActivateByFilename(txCtx, storedName)
		return err
	})
	if activateErr != nil {
		s.deps.Logger.Error("cleanup intent activation failed after write error",
			"kind", k.name,
			"owner_id", ownerID,
			"error", activateErr)
	}
	return 0, fmt.Errorf("store upload: %w", err)
}

// releaseFailedUpload disposes of the bytes of an upload whose metadata could
// not be persisted, and settles its queued intent: complete when the object is
// gone, re-activated when removal failed so the sweep retries.
//
// It does that ONLY when the caller can prove the metadata never landed
// (rejectedBeforeCommit). Everything else is in doubt and must be left alone:
// a transaction whose COMMIT failed to acknowledge may well have committed on
// the server, and removing the object here would strand a live row whose
// download 404s forever. The queued intent decides instead: a committed
// transaction settled it, so the object stays; a rolled-back one leaves it
// queued and the sweep reclaims the bytes once the deadline has passed.
func (s *Service) releaseFailedUpload(ctx context.Context, k kind, tenantID, ownerID int64, storedName string, rejectedBeforeCommit bool, cause error) {
	if !rejectedBeforeCommit {
		s.deps.Logger.Warn("document upload outcome undecided, leaving cleanup to the queued intent",
			"kind", k.name,
			"owner_id", ownerID,
			"error", cause,
		)
		return
	}
	if err := k.objects.Remove(ctx, tenantID, storedName); err != nil {
		s.deps.Logger.Error("document cleanup failed after upload error",
			"kind", k.name,
			"owner_id", ownerID,
			"error", err,
		)
		activateErr := s.deps.Tx.RunWrite(ctx, func(txCtx context.Context) error {
			_, err := k.cleanups.ActivateByFilename(txCtx, storedName)
			return err
		})
		if activateErr != nil {
			s.deps.Logger.Error("document cleanup intent activation failed",
				"kind", k.name,
				"owner_id", ownerID,
				"error", activateErr,
			)
		}
		return
	}
	completeErr := s.deps.Tx.RunWrite(ctx, func(txCtx context.Context) error {
		_, err := k.cleanups.MarkCompleteByFilename(txCtx, storedName)
		return err
	})
	if completeErr != nil {
		s.deps.Logger.Warn("document cleanup intent completion failed",
			"kind", k.name,
			"owner_id", ownerID,
			"error", completeErr,
		)
	}
}

// queueIntent records the intent to remove one object before it is written.
func (s *Service) queueIntent(ctx context.Context, k kind, ownerID int64, storedName string, retryAfter time.Time) error {
	if ownerID <= 0 || strings.TrimSpace(storedName) == "" {
		return fmt.Errorf("%w: cleanup intent details are required", domain.ErrInvalid)
	}
	return s.deps.Tx.RunWrite(ctx, func(txCtx context.Context) error {
		_, err := k.cleanups.Queue(txCtx, ownerID, storedName, retryAfter)
		return err
	})
}

// openObject resolves the bytes of a document. Missing bytes are
// domain.ErrObjectNotFound: the row exists, the object does not.
func (s *Service) openObject(ctx context.Context, k kind, tenantID int64, document domain.Document) (ports.Object, error) {
	object, err := k.objects.Open(ctx, tenantID, document.FilenameStored)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrObjectNotFound, err)
	}
	return object, nil
}
