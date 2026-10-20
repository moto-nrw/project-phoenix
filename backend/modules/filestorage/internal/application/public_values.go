package application

import (
	"errors"
	"io"
	"sync"

	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

func domainUpload(value filestorage.Upload) domain.Upload {
	return domain.Upload{FilenameDisplay: value.Filename, ContentType: value.ContentType, Extension: value.Extension, Content: value.Content}
}

func toFolderOverview(overview domain.Overview) filestorage.FolderOverview {
	result := filestorage.FolderOverview{
		Folders:            make([]filestorage.Folder, 0, len(overview.Folders)),
		CanManage:          overview.CanManage,
		CanUpload:          overview.CanUpload,
		StaffUploadEnabled: overview.StaffUploadEnabled,
		UsedBytes:          overview.UsedBytes,
		MaxBytes:           overview.MaxBytes,
	}
	for _, view := range overview.Folders {
		result.Folders = append(result.Folders, toFolder(view))
	}
	return result
}

func toFolder(view domain.FolderView) filestorage.Folder {
	return filestorage.Folder{
		ID:         view.ID,
		Name:       view.Name,
		Visibility: view.Visibility,
		FileCount:  view.FileCount,
		RoleIDs:    append([]int64{}, view.Audience.RoleIDs...),
		AccountIDs: append([]int64{}, view.Audience.AccountIDs...),
		CreatedAt:  view.CreatedAt,
	}
}

func toFile(document domain.Document, canDelete bool) filestorage.File {
	return filestorage.File{
		ID: document.ID, FolderID: document.OwnerID, Filename: document.FilenameDisplay,
		SizeBytes: document.SizeBytes, ContentType: document.ContentType,
		UploadedAt: document.CreatedAt, UploadedBy: document.UploadedBy, CanDelete: canDelete,
	}
}

func toAttachment(document domain.Document) filestorage.Attachment {
	return filestorage.Attachment{
		ID: document.ID, AnnouncementID: document.OwnerID, Filename: document.FilenameDisplay,
		SizeBytes: document.SizeBytes, ContentType: document.ContentType, UploadedAt: document.CreatedAt,
	}
}

func toAttachments(documents []domain.Document) []filestorage.Attachment {
	result := make([]filestorage.Attachment, 0, len(documents))
	for _, document := range documents {
		result = append(result, toAttachment(document))
	}
	return result
}

func toContent(document domain.Document, object ports.Object) filestorage.Content {
	return filestorage.Content{Object: &storedObject{Object: object}, ModTime: object.ModTime(), Filename: document.FilenameDisplay, ContentType: document.ContentType}
}

// storedObject adapts the backend's seekable object to the public random
// access shape. The local backend hands out *os.File, which reads at offsets
// natively; any other object is served through a guarded seek-and-read.
type storedObject struct {
	ports.Object
	mu   sync.Mutex
	size int64
	seen bool
}

func (o *storedObject) ReadAt(p []byte, off int64) (int, error) {
	if reader, ok := o.Object.(io.ReaderAt); ok {
		return reader.ReadAt(p, off)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, err := o.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	// io.ReaderAt reports a short final read as io.EOF, never as
	// io.ErrUnexpectedEOF; a section reader would otherwise fail the last
	// chunk of a download instead of ending it.
	n, err := io.ReadFull(o.Object, p)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	return n, err
}

func (o *storedObject) Size() int64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.seen {
		return o.size
	}
	current, err := o.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0
	}
	end, err := o.Seek(0, io.SeekEnd)
	if err != nil {
		return 0
	}
	if _, err := o.Seek(current, io.SeekStart); err != nil {
		return 0
	}
	o.size, o.seen = end, true
	return end
}
