package filestorage

import "context"

// PhotoCleanup persists detached photo URLs in the existing file cleanup
// outbox. Queue joins the triggering transaction; Sweep holds each intent's
// row lock through idempotent removal and completion, like folder cleanup.
type UploadCleanup struct {
	queue func(context.Context, string) error
	sweep func(context.Context, func(context.Context, string) error) (int, error)
}

type UploadCleanupKind string

const CleanupStudentPhotos UploadCleanupKind = "student-photos"
const CleanupEnrollmentLegalDocuments UploadCleanupKind = "enrollment-legal-documents"

func NewUploadCleanup(queue func(context.Context, string) error, sweep func(context.Context, func(context.Context, string) error) (int, error)) *UploadCleanup {
	return &UploadCleanup{queue: queue, sweep: sweep}
}
func (c *UploadCleanup) Queue(ctx context.Context, url string) error { return c.queue(ctx, url) }
func (c *UploadCleanup) Sweep(ctx context.Context, remove func(context.Context, string) error) (int, error) {
	return c.sweep(ctx, remove)
}
