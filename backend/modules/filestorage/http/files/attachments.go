package files

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// Anhänge an Elternmitteilungen (#2890).
//
// Warum diese Routen hier liegen und nicht bei der Mitteilung: die Bytes einer
// hochgeladenen Datei sind Sache der Dateiablage. Die Mitteilung steuert nur
// bei, wer die Datei sehen darf, und tut das über einen Port des Owners.
//
// Der Anhang geht bewusst NICHT mit der E-Mail raus. Elternmitteilungen kennen
// zwei E-Mail-Empfängerkreise, und "alle Bezugspersonen" erreicht auch Adressen
// ohne Portalzugang. Ein Anhang an eine solche Adresse würde genau die
// Zugangskontrolle umgehen, die "nur mit Portalzugang" herstellt.

// Beide Meldungen erreichen eine Person in der Schule und sagen, was als
// Nächstes zu tun ist, nicht nur, dass etwas fehlgeschlagen ist.
var (
	//nolint:staticcheck // ST1005: user-facing German message
	errAnnouncementPublished = errors.New("Die Mitteilung ist schon veröffentlicht. Ziehen Sie sie zurück, um Anhänge zu ändern.")
	//nolint:staticcheck // ST1005: user-facing German message
	errAttachmentLimit = errors.New("Diese Mitteilung hat schon die höchstmögliche Zahl an Anhängen.")
)

// AnnouncementAttachmentResponse is one attachment on the wire.
type AnnouncementAttachmentResponse struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// AnnouncementAttachmentListResponse is the attachment list of one
// announcement, together with the limits the UI must state BEFORE somebody
// picks a file. Editable is false once the announcement is published; es
// bleibt true, wenn nur die Höchstzahl erreicht ist.
type AnnouncementAttachmentListResponse struct {
	Attachments []AnnouncementAttachmentResponse `json:"attachments"`
	MaxCount    int                              `json:"max_count"`
	MaxBytes    int64                            `json:"max_bytes"`
	Editable    bool                             `json:"editable"`
}

func newAttachmentResponse(attachment filestorage.Attachment) AnnouncementAttachmentResponse {
	return AnnouncementAttachmentResponse{
		ID:          idString(attachment.ID),
		Filename:    attachment.Filename,
		SizeBytes:   attachment.SizeBytes,
		ContentType: attachment.ContentType,
		UploadedAt:  attachment.UploadedAt,
	}
}

func newAttachmentListResponse(attachments []filestorage.Attachment, editable bool) *AnnouncementAttachmentListResponse {
	resp := &AnnouncementAttachmentListResponse{
		Attachments: make([]AnnouncementAttachmentResponse, 0, len(attachments)),
		MaxCount:    filestorage.MaxAnnouncementAttachments,
		MaxBytes:    maxFile,
		Editable:    editable,
	}
	for _, attachment := range attachments {
		resp.Attachments = append(resp.Attachments, newAttachmentResponse(attachment))
	}
	return resp
}

// AnnouncementAttachmentRouter serves the staff side under
// /api/announcement-attachments. It is gated on the same permission as the
// announcements themselves (#1669): who may write an Elternmitteilung may
// attach a file to it.
func (rs *Resource) AnnouncementAttachmentRouter() chi.Router {
	r := chi.NewRouter()
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		admin := common.RequiresPermission(permissions.AdminWildcard)

		r.With(admin, withTx).Get("/{announcementId}", rs.listAnnouncementAttachments)
		r.With(admin).Post("/{announcementId}", rs.uploadAnnouncementAttachment)
		r.With(admin).Get("/{announcementId}/{attachmentId}/download", rs.downloadAnnouncementAttachment)
		r.With(admin, withTx).Delete("/{announcementId}/{attachmentId}", rs.deleteAnnouncementAttachment)
	})
	return r
}

// ParentAnnouncementAttachmentRouter serves the guardian side. It is mounted
// at the root rather than under /parent, which is a catch-all mount, and is
// authenticated with ParentMiddleware. No tenant transaction: a parent token
// is cross-tenant, so the owner resolves the school from the announcement.
func (rs *Resource) ParentAnnouncementAttachmentRouter() chi.Router {
	r := chi.NewRouter()
	common.ProtectedParentGroup(r, func(r chi.Router) {
		r.Get("/{announcementId}", rs.listParentAnnouncementAttachments)
		r.Get("/{announcementId}/{attachmentId}/download", rs.downloadParentAnnouncementAttachment)
	})
	return r
}

// renderAttachmentError maps the attachment errors onto HTTP.
func renderAttachmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, filestorage.ErrAttachmentNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, filestorage.ErrAttachmentPublished):
		common.RenderError(w, r, common.ErrorConflictWithCode(errAnnouncementPublished, "announcement_published"))
	case errors.Is(err, filestorage.ErrAttachmentLimitReached):
		common.RenderError(w, r, common.ErrorConflictWithCode(errAttachmentLimit, "attachment_limit_reached"))
	case errors.Is(err, filestorage.ErrObjectNotFound):
		common.RenderError(w, r, common.ErrorNotFound(errors.New("attachment not found")))
	default:
		renderError(w, r, err)
	}
}

// --- staff handlers -----------------------------------------------------------

func (rs *Resource) listAnnouncementAttachments(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	list, err := rs.files.ListAttachments(r.Context(), announcementID)
	if err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, newAttachmentListResponse(list.Attachments, list.Editable), "Attachments retrieved successfully")
}

func (rs *Resource) uploadAnnouncementAttachment(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	upload, closeUpload, err := parseUpload(w, r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(common.GermanUploadError(err, maxFile)))
		return
	}
	defer closeUpload()

	attachment, err := rs.files.UploadAttachment(r.Context(), announcementID, upload, actor)
	if err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	resp := newAttachmentResponse(attachment)
	common.Respond(w, r, http.StatusCreated, &resp, "Attachment uploaded successfully")
}

func (rs *Resource) downloadAnnouncementAttachment(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	attachmentID, ok := common.ParseInt64IDWithError(w, r, "attachmentId", "invalid attachment ID")
	if !ok {
		return
	}
	content, err := rs.files.OpenAttachment(r.Context(), announcementID, attachmentID)
	if err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	rs.serveContent(w, r, content)
}

func (rs *Resource) deleteAnnouncementAttachment(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	attachmentID, ok := common.ParseInt64IDWithError(w, r, "attachmentId", "invalid attachment ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	if err := rs.files.DeleteAttachment(r.Context(), announcementID, attachmentID, actor); err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"announcement_id": idString(announcementID)}, "Attachment deleted successfully")
}

// --- parent handlers ----------------------------------------------------------

// parentAccountID reads the guardian account from the parent token.
func parentAccountID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token: no account id")))
		return 0, false
	}
	return int64(claims.ID), true
}

func (rs *Resource) listParentAnnouncementAttachments(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	accountID, ok := parentAccountID(w, r)
	if !ok {
		return
	}
	attachments, err := rs.files.ListGuardianAttachments(r.Context(), accountID, announcementID)
	if err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	// The parent side never edits attachments; the flag exists so both
	// responses share one shape.
	common.Respond(w, r, http.StatusOK, newAttachmentListResponse(attachments, false), "Attachments retrieved successfully")
}

func (rs *Resource) downloadParentAnnouncementAttachment(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	attachmentID, ok := common.ParseInt64IDWithError(w, r, "attachmentId", "invalid attachment ID")
	if !ok {
		return
	}
	accountID, ok := parentAccountID(w, r)
	if !ok {
		return
	}
	content, err := rs.files.OpenGuardianAttachment(r.Context(), accountID, announcementID, attachmentID)
	if err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	rs.serveContent(w, r, content)
}
