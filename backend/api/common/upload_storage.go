package common

import (
	"context"
	"io"
	"time"
)

// UploadStorage supplies native byte operations. The composition root binds it;
// HTTP parsing, filename generation and response rendering stay in this consumer.
type UploadStorage interface {
	SaveNamed(io.Reader, string, string, bool) (string, error)
	Open(context.Context, string, string) (io.ReadSeekCloser, time.Time, error)
	RemoveUpload(string)
	PublicDir() (string, error)
	StoredPath(string, string, string) (string, error)
}
