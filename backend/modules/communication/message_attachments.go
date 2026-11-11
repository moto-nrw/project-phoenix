package communication

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Message attachments (#6685): photos and files of a chat message. A file is
// uploaded first and stays unclaimed with its uploader; sending the message
// claims it. Its lifetime is the message's.
const (
	// MaxMessageAttachments is how many files one message carries.
	MaxMessageAttachments = 5
	// MaxMessageAttachmentBytes is the size limit of one file.
	MaxMessageAttachmentBytes int64 = 10 << 20
	// MaxUnclaimedMessageAttachments bounds the uploads one account may
	// hold without sending them.
	MaxUnclaimedMessageAttachments = 20
	// UnclaimedMessageAttachmentLifetime is how long an upload waits for
	// its message before the retention removes it.
	UnclaimedMessageAttachmentLifetime = 24 * time.Hour
	// FinishedMessageAttachmentCleanupLifetime is how long a settled
	// cleanup intent stays as evidence of the removal.
	FinishedMessageAttachmentCleanupLifetime = 30 * 24 * time.Hour
	// MessageAttachmentObjectKind names the private object namespace.
	MessageAttachmentObjectKind = "message-attachments"

	// MessageAttachmentScopeParent covers the conversations with guardians,
	// 1:1 and group threads; MessageAttachmentScopeStaff the team chat.
	MessageAttachmentScopeParent = "parent"
	MessageAttachmentScopeStaff  = "staff"
)

var (
	// ErrMessageAttachmentNotFound hides an attachment the caller may not
	// open behind the answer for one that does not exist.
	ErrMessageAttachmentNotFound = errors.New("message attachment not found")
	// ErrMessageAttachmentUnavailable refuses a send that names an upload
	// the sender does not hold or that another message already carries.
	ErrMessageAttachmentUnavailable = errors.New("message attachment is not available for this message")
	// ErrMessageAttachmentLimit refuses more files than one message or one
	// account's unsent uploads may hold.
	ErrMessageAttachmentLimit = errors.New("message attachment limit reached")
)

// MessageAttachment is one file of a message as every reader of the
// conversation sees it. The three message tables store this value as their
// attachments snapshot, so its JSON is the wire format too.
type MessageAttachment struct {
	ID          int64  `json:"id,string"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
	// Width and Height are set for images, so a conversation reserves the
	// picture's place before its bytes arrive.
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

// MessageAttachmentUpload is one validated file on its way into the store.
type MessageAttachmentUpload struct {
	Filename    string
	ContentType string
	Extension   string
	SizeBytes   int64
	Width       int
	Height      int
}

// PreparedMessageAttachment is the committed row of an upload whose bytes
// the caller writes next under StoredName.
type PreparedMessageAttachment struct {
	Attachment MessageAttachment
	TenantID   int64
	StoredName string
}

// MessageAttachmentFile locates the bytes of an attachment the caller may open.
type MessageAttachmentFile struct {
	Attachment MessageAttachment
	TenantID   int64
	StoredName string
}

// MessageAttachmentRecord is an attachment with the conversation its
// message belongs to. ThreadID is zero while the upload is unclaimed.
type MessageAttachmentRecord struct {
	Attachment          MessageAttachment
	TenantID            int64
	Scope               string
	UploadedByAccountID int64
	StoredName          string
	Claimed             bool
	// GroupThread tells a group thread with guardians from a 1:1
	// conversation; both live in scope parent.
	GroupThread      bool
	ThreadID         int64
	MessageCreatedAt time.Time
}

// MessageAttachmentCapability is the upload and open side of one chat
// family. Every method runs in the caller's tenant transaction.
type MessageAttachmentCapability interface {
	PrepareMessageAttachment(ctx context.Context, upload MessageAttachmentUpload) (*PreparedMessageAttachment, error)
	DiscardMessageAttachment(ctx context.Context, attachmentID int64) error
	OpenMessageAttachment(ctx context.Context, attachmentID int64) (*MessageAttachmentFile, error)
}

// PortalMessageAttachments is the guardian's side of the same store. The
// parents portal has authorized the child before it calls; the store checks
// that the attachment belongs to a conversation of this guardian and child.
type PortalMessageAttachments interface {
	PrepareGuardianAttachment(ctx context.Context, accountID int64, upload MessageAttachmentUpload) (*PreparedMessageAttachment, error)
	DiscardGuardianAttachment(ctx context.Context, accountID, attachmentID int64) error
	OpenGuardianAttachment(ctx context.Context, accountID, studentID, attachmentID int64) (*MessageAttachmentFile, error)
}

// MessageAttachmentCleanup removes the bytes of deleted attachments.
type MessageAttachmentCleanup interface {
	// SweepMessageAttachmentObjects removes the objects of the school's
	// pending cleanup intents in the caller's tenant transaction and settles
	// them. A failed removal leaves its intent for the next pass.
	SweepMessageAttachmentObjects(ctx context.Context, remove func(ctx context.Context, tenantID int64, storedName string) error) (int, error)
}

// MessageAttachmentCleanupIntent is one queued object removal.
type MessageAttachmentCleanupIntent struct {
	ID         int64
	TenantID   int64
	StoredName string
}

type messageAttachmentIDsKey struct{}

// WithMessageAttachmentIDs carries the uploads a send names into the
// command. It is not authority: the store claims only unclaimed uploads of
// the sender in the message's chat family.
func WithMessageAttachmentIDs(ctx context.Context, ids []int64) context.Context {
	if len(ids) == 0 {
		return ctx
	}
	return context.WithValue(ctx, messageAttachmentIDsKey{}, append([]int64(nil), ids...))
}

// WithMessageSend carries one send attempt into the command: its client key
// and the uploads it names.
func WithMessageSend(ctx context.Context, clientMessageID string, attachmentIDs []int64) context.Context {
	return WithMessageAttachmentIDs(WithMessageSendID(ctx, clientMessageID), attachmentIDs)
}

// MessageAttachmentIDs returns the uploads the send in ctx names.
func MessageAttachmentIDs(ctx context.Context) []int64 {
	ids, _ := ctx.Value(messageAttachmentIDsKey{}).([]int64)
	return ids
}

// HasMessageAttachments reports whether the send in ctx names an upload, so
// a message without text is still a message.
func HasMessageAttachments(ctx context.Context) bool {
	return len(MessageAttachmentIDs(ctx)) > 0
}

// ReservedMessageAttachments builds the snapshot a message stores from the
// uploads its sender named and the rows the store could lock for it, in the
// order the sender chose. Every named upload must be there exactly once: a
// missing one belongs to somebody else or to another chat family.
func ReservedMessageAttachments(ids []int64, locked []MessageAttachment) (json.RawMessage, error) {
	byID := make(map[int64]MessageAttachment, len(locked))
	for _, attachment := range locked {
		byID[attachment.ID] = attachment
	}
	snapshot := make([]MessageAttachment, 0, len(ids))
	for _, id := range ids {
		attachment, ok := byID[id]
		if !ok {
			return nil, ErrMessageAttachmentUnavailable
		}
		delete(byID, id)
		snapshot = append(snapshot, attachment)
	}
	return json.Marshal(snapshot)
}

// MessageAttachmentSnapshot returns the stored snapshot for the wire: nil
// for a message without attachments, so its JSON omits the member.
func MessageAttachmentSnapshot(raw json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "[]" || trimmed == "null" {
		return nil
	}
	return raw
}

// MessagePreview is the text a conversation list shows for a message: its
// body, or the name of its first file for a message without text.
func MessagePreview(body string, attachments json.RawMessage) string {
	if strings.TrimSpace(body) != "" {
		return body
	}
	var files []MessageAttachment
	if err := json.Unmarshal(attachments, &files); err != nil || len(files) == 0 {
		return body
	}
	return "📎 " + files[0].Filename
}
