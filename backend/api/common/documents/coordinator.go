// Package documents owns the part of a document upload that is neither
// business logic nor persistence: getting bytes onto (and off) the storage
// backend without ever leaving an orphan behind.
//
// The order is forced by HTTP, not by preference. Multipart input cannot be
// replayed after a transaction commits, so the file must be written before
// its metadata exists. That opens a window in which a crash leaves bytes with
// no row pointing at them, which for a medical certificate is exactly the
// kind of leftover that must not happen. The window is closed by writing a
// durable cleanup intent BEFORE the object and settling it afterwards:
//
//	queue intent -> write object -> commit metadata -> settle intent
//
// Any step can die. An intent that is never settled becomes eligible for the
// cleanup scheduler after a delay that is deliberately longer than the
// request deadline, so the scheduler can never delete an upload that is still
// in flight.
package documents

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/internal/storage"
)

// Store is the subset of a domain's document service the coordinator needs to
// settle cleanup bookkeeping. Each domain implements it over its own tables.
type Store interface {
	// MarkFileDeleted records that a document's bytes are gone.
	MarkFileDeleted(ctx context.Context, documentID int64) error
	// MarkQueuedCleanupComplete settles one orphan intent by ID.
	MarkQueuedCleanupComplete(ctx context.Context, cleanupID int64) error
	// MarkQueuedCleanupCompleteByFilename settles the intent of one object.
	MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error
	// ActivateQueuedCleanup makes an intent eligible for retry right away.
	ActivateQueuedCleanup(ctx context.Context, storedName string) error
}

// Coordinator moves document bytes for one domain.
type Coordinator struct {
	// Kind is the storage key prefix, e.g. "student-documents".
	Kind    string
	Backend storage.Backend
	Store   Store
	Logger  *slog.Logger
}

func (c *Coordinator) logger() *slog.Logger {
	if c.Logger == nil {
		return slog.Default()
	}
	return c.Logger
}

func (c *Coordinator) key(tenantID int64, storedName string) (string, error) {
	return storage.TenantKey(c.Kind, tenantID, storedName)
}

// NewStoredName generates the storage name for a validated content type. The
// display name never reaches the filesystem: a UUID cannot collide, cannot
// carry a traversal payload, and leaks nothing about the content.
func NewStoredName(extension string) (string, error) {
	if extension == "" {
		return "", errors.New("unsupported document content type")
	}
	id, err := uuid.NewV4()
	if err != nil {
		return "", errors.New("failed to generate document filename")
	}
	return id.String() + extension, nil
}

// Save writes the uploaded object privately and returns the stored size.
func (c *Coordinator) Save(ctx context.Context, tenantID int64, storedName string, source io.Reader) (int64, error) {
	key, err := c.key(tenantID, storedName)
	if err != nil {
		return 0, err
	}
	return c.Backend.Save(ctx, key, source, storage.SaveOptions{Private: true})
}

// Serve streams the object as a download named displayName. It returns false
// when nothing was written, so the caller can still render an error.
func (c *Coordinator) Serve(w http.ResponseWriter, r *http.Request, tenantID int64, storedName, displayName, contentType string) bool {
	return c.serve(w, r, tenantID, storedName, displayName, contentType, false)
}

// InlineViewable reports whether a browser can render the content type
// itself. Office containers are deliberately absent: the browser would offer
// a download anyway, and an inline disposition for them only invites a
// plugin to run.
func InlineViewable(contentType string) bool {
	switch contentType {
	case "application/pdf", "image/png", "image/jpeg", "image/jpg":
		return true
	default:
		return false
	}
}

// ServeInline streams the object for display in the browser (PDF viewer,
// image) instead of as a download. Only InlineViewable content types are
// served inline; everything else falls back to Serve.
//
// The response carries a sandboxing CSP: an inline document is user-uploaded
// content rendered on the application's own origin, and `sandbox` denies it
// scripts, forms and same-origin access, so a PDF with embedded JavaScript
// cannot act as the signed-in user.
func (c *Coordinator) ServeInline(w http.ResponseWriter, r *http.Request, tenantID int64, storedName, displayName, contentType string) bool {
	return c.serve(w, r, tenantID, storedName, displayName, contentType, InlineViewable(contentType))
}

func (c *Coordinator) serve(w http.ResponseWriter, r *http.Request, tenantID int64, storedName, displayName, contentType string, inline bool) bool {
	key, err := c.key(tenantID, storedName)
	if err != nil {
		return false
	}
	object, err := c.Backend.Open(r.Context(), key)
	if err != nil {
		return false
	}
	defer func() {
		if err := object.Close(); err != nil {
			c.logger().Error("document close error", "error", err)
		}
	}()

	disposition := "attachment"
	if inline {
		disposition = "inline"
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{
		"filename": displayName,
	}))
	w.Header().Set("Content-Type", contentType)
	// no-store: a revoked permission or a deleted document must not be
	// served from a cache the application no longer controls.
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, displayName, object.ModTime(), object)
	return true
}

// Remove deletes the stored object. A missing object counts as removed so
// retries converge.
func (c *Coordinator) Remove(ctx context.Context, tenantID int64, storedName string) error {
	key, err := c.key(tenantID, storedName)
	if err != nil {
		return err
	}
	return c.Backend.Remove(ctx, key)
}

// CleanupDocument removes a soft-deleted document's bytes and records that
// they are gone. Failures are logged, never propagated: the caller is a
// post-commit hook whose transaction is already finished.
func (c *Coordinator) CleanupDocument(ctx context.Context, tenantID, ownerID, documentID int64, storedName, source string) {
	if err := c.Remove(ctx, tenantID, storedName); err != nil {
		c.logger().Warn("document cleanup failed",
			"kind", c.Kind,
			"owner_id", ownerID,
			"document_id", documentID,
			"source", source,
			"error", err,
		)
		return
	}
	if err := c.Store.MarkFileDeleted(ctx, documentID); err != nil {
		c.logger().Error("document cleanup status update failed",
			"kind", c.Kind,
			"owner_id", ownerID,
			"document_id", documentID,
			"source", source,
			"error", err,
		)
	}
}

// ReleaseFailedUpload disposes of the bytes of an upload whose metadata could
// not be persisted, and settles its queued intent: complete when the object is
// gone, re-activated when removal failed so the scheduler retries.
//
// It does that ONLY when the caller can prove the metadata never landed —
// rejectedBeforeCommit. Everything else is in doubt and must be left alone:
// a transaction whose COMMIT failed to acknowledge (a dropped connection is
// the classic case, not just a cancelled context) may well have committed on
// the server, and removing the object here would strand a live document row
// whose download 404s forever with no sweep able to notice.
//
// Leaving it alone is always safe, which is why it is the default. The queued
// intent decides instead: a committed transaction settled it in the same
// transaction, so the object stays; a rolled-back one leaves it queued and the
// scheduler reclaims the bytes once the upload deadline has passed. The only
// cost of being wrong in this direction is a few minutes of delay.
func (c *Coordinator) ReleaseFailedUpload(ctx context.Context, tenantID, ownerID int64, storedName string, rejectedBeforeCommit bool, cause error) {
	if !rejectedBeforeCommit {
		c.logger().Warn("document upload outcome undecided, leaving cleanup to the queued intent",
			"kind", c.Kind,
			"owner_id", ownerID,
			"error", cause,
		)
		return
	}
	if err := c.Remove(ctx, tenantID, storedName); err != nil {
		c.logger().Error("document cleanup failed after upload error",
			"kind", c.Kind,
			"owner_id", ownerID,
			"error", err,
		)
		if activateErr := c.Store.ActivateQueuedCleanup(ctx, storedName); activateErr != nil {
			c.logger().Error("document cleanup intent activation failed",
				"kind", c.Kind,
				"owner_id", ownerID,
				"error", activateErr,
			)
		}
		return
	}
	if err := c.Store.MarkQueuedCleanupCompleteByFilename(ctx, storedName); err != nil {
		c.logger().Warn("document cleanup intent completion failed",
			"kind", c.Kind,
			"owner_id", ownerID,
			"error", err,
		)
	}
}
