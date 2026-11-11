package common

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg" // decode the dimensions of JPEG attachments
	_ "image/png"  // decode the dimensions of PNG attachments
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common/imagemeta"
)

// Shared inbound helpers of the photos and files of chat messages (#6685).
// The three portals upload and read through the same checks: content by
// magic bytes, extension by content, images without metadata.

// MessageAttachmentUpload is one validated file of a message upload.
type MessageAttachmentUpload struct {
	Filename    string
	ContentType string
	Extension   string
	SizeBytes   int64
	// Width and Height are the pixel size of an image, zero otherwise.
	Width, Height int
	Content       io.Reader
}

// MessageAttachmentFiles moves the bytes of the attachments. The root binds
// File Storage's document coordinator of the message-attachments namespace.
type MessageAttachmentFiles interface {
	Save(ctx context.Context, tenantID int64, storedName string, source io.Reader) (int64, error)
	Serve(w http.ResponseWriter, r *http.Request, tenantID int64, storedName, displayName, contentType string) bool
	ServeInline(w http.ResponseWriter, r *http.Request, tenantID int64, storedName, displayName, contentType string) bool
}

// MessageAttachmentFileStorage opens the object store for one request, so
// building the routes opens no storage.
type MessageAttachmentFileStorage func() (MessageAttachmentFiles, error)

// ParseMessageAttachment reads the multipart field "file" as one attachment:
// PDF, PNG, JPEG, DOCX, XLSX or PPTX by content, the extension naming that
// content. An image comes back without Exif, GPS and other metadata and with
// its pixel size; the size is measured from the bytes that will be stored.
func ParseMessageAttachment(w http.ResponseWriter, r *http.Request, maxFileSize int64) (*MessageAttachmentUpload, error) {
	uploaded, err := ParseOfficeFileWithLimits(w, r, "file", maxFileSize, maxFileSize+4096)
	if err != nil {
		return nil, err
	}
	defer CloseFile(uploaded.File)
	data, err := io.ReadAll(io.LimitReader(uploaded.File, maxFileSize+1))
	if err != nil || int64(len(data)) > maxFileSize {
		return nil, uploadRefusal{code: CodeFilesFileTooLarge, text: "file too large"}
	}
	upload := &MessageAttachmentUpload{
		Filename: uploaded.Filename, ContentType: uploaded.ContentType,
		Extension: DocumentFileExtension(uploaded.ContentType),
	}
	if strings.HasPrefix(uploaded.ContentType, "image/") {
		if data, err = imagemeta.Strip(data, uploaded.ContentType); err != nil {
			return nil, uploadRefusal{code: CodeFilesFileUnreadable, text: "cannot read image"}
		}
		// A picture the decoder cannot measure still uploads; the reader
		// then shows it without a reserved frame.
		if config, _, decodeErr := image.DecodeConfig(bytes.NewReader(data)); decodeErr == nil {
			upload.Width, upload.Height = config.Width, config.Height
		}
	}
	if len(data) == 0 {
		return nil, uploadRefusal{code: CodeFilesFileUnreadable, text: "empty file"}
	}
	upload.SizeBytes = int64(len(data))
	upload.Content = bytes.NewReader(data)
	return upload, nil
}

// ErrTooManyMessageAttachments refuses a send that names more files than one
// message carries.
var ErrTooManyMessageAttachments = errors.New("too many attachments for one message")

// ParseMessageAttachmentIDs reads the attachment_ids of a send request.
func ParseMessageAttachmentIDs(raw []string, limit int) ([]int64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > limit {
		return nil, ErrTooManyMessageAttachments
	}
	ids := make([]int64, 0, len(raw))
	for _, value := range raw {
		id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid attachment ID")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ParseMessageSend checks the client key and the attachment_ids of a send
// request. It answers a malformed request itself and reports false.
func ParseMessageSend(w http.ResponseWriter, r *http.Request, clientMessageID string, attachmentIDs []string, limit int) ([]int64, bool) {
	if len(clientMessageID) > 128 {
		RenderError(w, r, ErrorInvalidRequestMessageWithCodeAndDetails("client_message_id exceeds 128 bytes", CodeGeneralInput, map[string]any{"max_bytes": 128}))
		return nil, false
	}
	ids, err := ParseMessageAttachmentIDs(attachmentIDs, limit)
	if errors.Is(err, ErrTooManyMessageAttachments) {
		RenderError(w, r, ErrorConflictWithCode(err, CodeFilesAttachmentLimitReached))
		return nil, false
	}
	if err != nil {
		RenderError(w, r, ErrorInvalidRequest(err))
		return nil, false
	}
	return ids, true
}

// ServeMessageAttachment streams one attachment. ?inline=1 shows a picture
// or PDF in the browser, which is how a conversation renders its photos;
// every other request downloads. It reports false when the bytes are gone.
func ServeMessageAttachment(w http.ResponseWriter, r *http.Request, files MessageAttachmentFiles, tenantID int64, storedName, filename, contentType string) bool {
	if r.URL.Query().Get("inline") == "1" {
		return files.ServeInline(w, r, tenantID, storedName, filename, contentType)
	}
	return files.Serve(w, r, tenantID, storedName, filename, contentType)
}
