package careplanhttp

import (
	context "context"
	io "io"
	slog "log/slog"
	http "net/http"
)

// StudentDocumentFileStore is the existing cleanup bookkeeping the byte
// coordinator settles after the document service commits its metadata.
type StudentDocumentFileStore interface {
	MarkFileDeleted(context.Context, int64) error
	MarkQueuedCleanupCompleteByFilename(context.Context, string) error
	ActivateQueuedCleanup(context.Context, string) error
}

// StudentDocumentFiles is the byte lifecycle this adapter consumes. Root
// supplies the native File Storage coordinator rather than a private import.
type StudentDocumentFiles interface {
	NewStoredName(string) (string, error)
	Save(context.Context, int64, string, io.Reader) (int64, error)
	Serve(http.ResponseWriter, *http.Request, int64, string, string, string) bool
	Remove(context.Context, int64, string) error
	CleanupDocument(context.Context, int64, int64, int64, string, string)
	ReleaseFailedUpload(context.Context, int64, int64, string, bool, error)
}

type StudentDocumentStorage func(kind string, store StudentDocumentFileStore, logger *slog.Logger) (StudentDocumentFiles, error)
