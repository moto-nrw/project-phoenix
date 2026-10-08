// Package objects owns every byte the application writes outside the
// database. Before it existed, six upload families each resolved their own
// directory and called os.* inline, so save, serve and delete of the same
// file could disagree about where it lived. Everything now goes through one
// Backend, which makes that class of bug structural rather than a matter of
// review discipline, and makes an object-store implementation a drop-in
// replacement instead of a sweep through every handler.
package objects

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is returned by Open and Stat when the key does not exist.
// Callers translate it to 404 rather than 500.
var ErrNotFound = errors.New("storage: object not found")

// ErrInvalidKey marks a key that failed validation (traversal, empty
// segment, absolute path). It never reaches the underlying medium.
var ErrInvalidKey = errors.New("storage: invalid key")

// Object carries an open byte stream and the timestamp captured when it opened.
// The caller closes Content. This is a byte capability, not a row repository.
type Object struct {
	Content    io.ReadSeekCloser
	ModifiedAt time.Time
}

// SaveOptions controls the visibility of a newly written object.
//
// Private is not a hint: it is the difference between a file that only the
// owning process can read and one that a misconfigured static mount could
// hand out. Personnel files and child documents are written private; avatars
// and login images are not.
type SaveOptions struct {
	Private bool
}

// Backend stores and retrieves opaque byte objects addressed by key.
//
// A key is a slash-separated relative path ("staff-documents/12/uuid.pdf").
// Implementations MUST reject keys that escape their root and MUST NOT
// interpret the key beyond that check — the caller owns the layout.
type Backend interface {
	// Save writes r under key, replacing any existing object, and returns
	// the number of bytes stored. A failed write leaves no partial object.
	WriteObject(ctx context.Context, key string, r io.Reader, opts SaveOptions) (int64, error)
	// Open returns the stored object. The caller closes it.
	OpenObject(ctx context.Context, key string) (*Object, error)
	// Remove deletes the object. Removing a missing object is not an error,
	// so cleanup retries are idempotent.
	RemoveObject(ctx context.Context, key string) error
	// Stat returns the stored size in bytes.
	ObjectSize(ctx context.Context, key string) (int64, error)
}
