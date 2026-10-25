package parent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Erklärungen (#3430): the declaration part of a feed item, the submission
// and the guardian's own proof.

type declarationVersionResponse struct {
	ID          string `json:"id"`
	VersionNo   int    `json:"version_no"`
	ContentHash string `json:"content_hash"`
}

type declarationSignerStateResponse struct {
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Action      *string    `json:"action"`
	SubmittedAt *time.Time `json:"submitted_at"`
}

type declarationChildResponse struct {
	StudentID      string                           `json:"student_id"`
	FirstName      string                           `json:"first_name"`
	LastName       string                           `json:"last_name"`
	CanSubmit      bool                             `json:"can_submit"`
	State          string                           `json:"state"`
	MyAction       *string                          `json:"my_action"`
	MySubmittedAt  *time.Time                       `json:"my_submitted_at"`
	AllowedActions []string                         `json:"allowed_actions"`
	OtherSigners   []declarationSignerStateResponse `json:"other_signers"`
}

// AnnouncementDeclarationResponse is the declaration block of a feed item.
type AnnouncementDeclarationResponse struct {
	Kind              string                      `json:"kind"`
	Signers           string                      `json:"signers"`
	Revocable         bool                        `json:"revocable"`
	RequiresPassword  bool                        `json:"requires_password"`
	RequiresSignature bool                        `json:"requires_signature"`
	Deadline          *time.Time                  `json:"deadline"`
	Closed            bool                        `json:"closed"`
	Version           *declarationVersionResponse `json:"version"`
	Children          []declarationChildResponse  `json:"children"`
}

func toDeclarationResponse(d *parentService.AnnouncementFeedDeclaration) *AnnouncementDeclarationResponse {
	if d == nil {
		return nil
	}
	out := &AnnouncementDeclarationResponse{
		Kind: d.Kind, Signers: d.Signers, Revocable: d.Revocable, RequiresPassword: d.RequiresPassword,
		RequiresSignature: d.RequiresSignature, Deadline: d.Deadline, Closed: d.Closed, Children: make([]declarationChildResponse, 0, len(d.Children)),
	}
	if d.Version != nil {
		out.Version = &declarationVersionResponse{
			ID: strconv.FormatInt(d.Version.ID, 10), VersionNo: d.Version.VersionNo, ContentHash: d.Version.ContentHash,
		}
	}
	for _, c := range d.Children {
		child := declarationChildResponse{
			StudentID: strconv.FormatInt(c.StudentID, 10), FirstName: c.FirstName, LastName: c.LastName,
			CanSubmit: c.CanSubmit, State: c.State, MyAction: c.MyAction, MySubmittedAt: c.MySubmittedAt,
			AllowedActions: c.AllowedActions, OtherSigners: make([]declarationSignerStateResponse, 0, len(c.OtherSigners)),
		}
		if child.AllowedActions == nil {
			child.AllowedActions = []string{}
		}
		for _, other := range c.OtherSigners {
			child.OtherSigners = append(child.OtherSigners, declarationSignerStateResponse{
				FirstName: other.FirstName, LastName: other.LastName, Action: other.Action, SubmittedAt: other.SubmittedAt,
			})
		}
		out.Children = append(out.Children, child)
	}
	return out
}

type declarationRequest struct {
	StudentID string `json:"student_id"`
	Action    string `json:"action"`
	VersionID string `json:"version_id"`
	Password  string `json:"password,omitempty"`
	// Signature is the guardian's signature (#4785): the method, the PNG as
	// base64 without a data: prefix and, when typed, the typed name.
	Signature *declarationSignatureRequest `json:"signature,omitempty"`
}

type declarationSignatureRequest struct {
	Method    string `json:"method"`
	ImagePNG  string `json:"image_png"`
	TypedName string `json:"typed_name,omitempty"`
	// Statement is the exact server-generated declaration rendered to the
	// signer. Pickup-signature flows require it to match their fresh wording.
	Statement string `json:"statement,omitempty"`
}

func (s *declarationSignatureRequest) input() *parentService.DeclarationSignatureInput {
	if s == nil {
		return nil
	}
	return &parentService.DeclarationSignatureInput{Method: s.Method, ImagePNG: s.ImagePNG, TypedName: s.TypedName}
}

type declarationSubmissionResponse struct {
	ID                string    `json:"id"`
	Action            string    `json:"action"`
	SubmittedAt       time.Time `json:"submitted_at"`
	VersionNo         int       `json:"version_no"`
	ContentHash       string    `json:"content_hash"`
	RecordHash        string    `json:"record_hash"`
	PasswordConfirmed bool      `json:"password_confirmed"`
}

// declarationBodyLimit bounds a declaration request including its signature.
const declarationBodyLimit = 320 << 10

func parsePositiveID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0
}

// submitDeclaration records one declaration for one child. The account comes
// from the token; the password, when the school asks for it, is checked by
// Identity & Access and never logged.
func (rs *Resource) submitDeclaration(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	announcementID, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	// A signature PNG is at most 200 KB, about 273 KB as base64 (#4785).
	r.Body = http.MaxBytesReader(w, r.Body, declarationBodyLimit)
	var req declarationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	studentID, okStudent := parsePositiveID(req.StudentID)
	versionID, okVersion := parsePositiveID(req.VersionID)
	if !okStudent || !okVersion {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("student_id and version_id are required")))
		return
	}
	submission, created, err := rs.ParentService.SubmitDeclaration(r.Context(), accountID, announcementID,
		parentService.DeclarationInput{StudentID: studentID, VersionID: versionID, Action: req.Action, Password: req.Password, Signature: req.Signature.input()},
		rs.passwordConfirmer(accountID))
	if err != nil {
		renderDeclarationError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"created": created,
		"submission": declarationSubmissionResponse{
			ID: strconv.FormatInt(submission.ID, 10), Action: submission.Action, SubmittedAt: submission.SubmittedAt,
			VersionNo: submission.VersionNo, ContentHash: submission.ContentHash, RecordHash: submission.RecordHash,
			PasswordConfirmed: submission.PasswordConfirmed,
		},
	}, "Declaration recorded")
}

// passwordConfirmer binds the login runtime's password check to the caller.
func (rs *Resource) passwordConfirmer(accountID int64) parentService.PasswordConfirmer {
	if rs.Auth.ConfirmPassword == nil {
		return nil
	}
	return func(ctx context.Context, password string) error {
		err := rs.Auth.ConfirmPassword(ctx, accountID, password)
		if err != nil && rs.Auth.InvalidCredentials != nil && rs.Auth.InvalidCredentials(err) {
			return parentService.ErrDeclarationPasswordIncorrect
		}
		return err
	}
}

func renderDeclarationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, parentService.ErrDeclarationNotPermitted):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareDeclarationNotPermitted))
	case errors.Is(err, parentService.ErrDeclarationVersionChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareDeclarationVersionChanged))
	case errors.Is(err, parentService.ErrDeclarationClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareDeclarationClosed))
	case errors.Is(err, parentService.ErrDeclarationActionNotAllowed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareDeclarationActionNotAllowed))
	case errors.Is(err, parentService.ErrDeclarationPasswordRequired):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareDeclarationPasswordRequired))
	case errors.Is(err, parentService.ErrDeclarationPasswordIncorrect):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCareDeclarationPasswordIncorrect))
	case errors.Is(err, parentService.ErrDeclarationSignatureRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareDeclarationSignatureRequired))
	case errors.Is(err, parentService.ErrDeclarationSignatureInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareDeclarationSignatureInvalid))
	case errors.Is(err, parentService.ErrChildCareEnded):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCareChildCareEnded))
	default:
		renderParentWriteError(w, r, err)
	}
}

type proofAttachmentResponse struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
}

type proofVersionResponse struct {
	ID          string                    `json:"id"`
	VersionNo   int                       `json:"version_no"`
	Title       string                    `json:"title"`
	Body        string                    `json:"body"`
	ContentHash string                    `json:"content_hash"`
	PublishedAt time.Time                 `json:"published_at"`
	Attachments []proofAttachmentResponse `json:"attachments"`
}

type proofSubmissionResponse struct {
	ID                string    `json:"id"`
	Action            string    `json:"action"`
	SubmittedAt       time.Time `json:"submitted_at"`
	SignerName        string    `json:"signer_name"`
	GuardianRole      string    `json:"guardian_role"`
	VersionNo         int       `json:"version_no"`
	PasswordConfirmed bool      `json:"password_confirmed"`
	Method            string    `json:"method"`
	ContentHash       string    `json:"content_hash"`
	RecordHash        string    `json:"record_hash"`
	// Signature is the guardian's own signature of a signed answer (#4785).
	Signature *proofSignatureResponse `json:"signature"`
}

type proofSignatureResponse struct {
	Method       string    `json:"method"`
	TypedName    *string   `json:"typed_name"`
	ImageDataURL string    `json:"image_data_url"`
	ImageSHA256  string    `json:"image_sha256"`
	SignedAt     time.Time `json:"signed_at"`
}

func toProofSignatureResponse(proof *parentService.DeclarationProof, sub *parentService.DeclarationSubmission) *proofSignatureResponse {
	s := proofSignatureForRendering(proof, sub)
	if s == nil {
		return nil
	}
	out := &proofSignatureResponse{
		Method: s.Method, ImageDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(s.PNG),
		ImageSHA256: s.ImageSHA256, SignedAt: s.SignedAt,
	}
	if s.TypedName != "" {
		typed := s.TypedName
		out.TypedName = &typed
	}
	return out
}

// proofSignatureForRendering returns only a signature from a proof whose
// checks, including its declaration and child binding, have passed. A proof
// with an altered entry must keep its warning without rendering stored data
// as the guardian's signature.
func proofSignatureForRendering(proof *parentService.DeclarationProof, sub *parentService.DeclarationSubmission) *parentService.DeclarationSignature {
	if !proof.IntegrityOK || sub.SignatureID == nil {
		return nil
	}
	s, ok := proof.Signatures[*sub.SignatureID]
	if !ok || s == nil || !s.IntegrityOK {
		return nil
	}
	return s
}

// proofMethodLabel names the procedure of the proof: the signature procedure
// only when the proof can safely render a signed answer.
func proofMethodLabel(proof *parentService.DeclarationProof) string {
	for _, sub := range proof.Submissions {
		if proofSignatureForRendering(proof, sub) != nil {
			return signatureMethodLabel
		}
	}
	return methodLabel
}

type declarationProofResponse struct {
	Title       string                    `json:"title"`
	SchoolName  string                    `json:"school_name"`
	ChildName   string                    `json:"child_name"`
	Kind        string                    `json:"kind"`
	MethodLabel string                    `json:"method_label"`
	GeneratedAt time.Time                 `json:"generated_at"`
	Versions    []proofVersionResponse    `json:"versions"`
	Submissions []proofSubmissionResponse `json:"submissions"`
	IntegrityOK bool                      `json:"integrity_ok"`
}

// methodLabel and signatureMethodLabel name the two procedures moto offers,
// and say what they are.
const (
	methodLabel          = "Einfache elektronische Erklärung im angemeldeten Eltern-Konto"
	signatureMethodLabel = "Einfache elektronische Signatur im angemeldeten Eltern-Konto"
)

// declarationProof returns the guardian's own proof for one child: JSON for
// the portal page, or with format=pdf the proof in the moto PDF design.
func (rs *Resource) declarationProof(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	announcementID, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePositiveID(r.URL.Query().Get("student_id"))
	if !ok {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("student_id is required")))
		return
	}
	proof, err := rs.ParentService.DeclarationProof(r.Context(), accountID, announcementID, studentID)
	if err != nil {
		renderDeclarationError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("format") == "pdf" {
		rs.writeDeclarationProofPDF(w, r, proof)
		return
	}
	common.Respond(w, r, http.StatusOK, toDeclarationProofResponse(proof, time.Now()), "Declaration proof retrieved")
}

func (rs *Resource) writeDeclarationProofPDF(w http.ResponseWriter, r *http.Request, proof *parentService.DeclarationProof) {
	file, err := rs.renderDeclarationProof(proof, time.Now())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("render declaration proof", err))
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func toDeclarationProofResponse(proof *parentService.DeclarationProof, now time.Time) declarationProofResponse {
	out := declarationProofResponse{
		Title: proof.Title, SchoolName: proof.SchoolName, ChildName: proof.ChildName, Kind: proof.Kind,
		MethodLabel: proofMethodLabel(proof), GeneratedAt: now, IntegrityOK: proof.IntegrityOK,
		Versions: []proofVersionResponse{}, Submissions: []proofSubmissionResponse{},
	}
	declared := map[int64]bool{}
	for _, sub := range proof.Submissions {
		versionNo := 0
		if v, ok := proof.Versions[sub.VersionID]; ok {
			versionNo = v.VersionNo
		}
		declared[sub.VersionID] = true
		out.Submissions = append(out.Submissions, proofSubmissionResponse{
			ID: strconv.FormatInt(sub.ID, 10), Action: sub.Action, SubmittedAt: sub.SubmittedAt,
			SignerName: sub.SignerName, GuardianRole: sub.GuardianRole, VersionNo: versionNo,
			PasswordConfirmed: sub.PasswordConfirmed, Method: sub.Method,
			ContentHash: sub.ContentHash, RecordHash: sub.RecordHash,
			Signature: toProofSignatureResponse(proof, sub),
		})
	}
	versions := make([]*parentService.DeclarationVersion, 0, len(declared))
	for id := range declared {
		if v, ok := proof.Versions[id]; ok {
			versions = append(versions, v)
		}
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].VersionNo > versions[j].VersionNo })
	for _, v := range versions {
		version := proofVersionResponse{
			ID: strconv.FormatInt(v.ID, 10), VersionNo: v.VersionNo, Title: v.Title, Body: v.Body,
			ContentHash: v.ContentHash, PublishedAt: v.PublishedAt,
			Attachments: make([]proofAttachmentResponse, 0, len(v.Attachments)),
		}
		for _, a := range v.Attachments {
			version.Attachments = append(version.Attachments, proofAttachmentResponse{
				ID: strconv.FormatInt(a.AttachmentID, 10), Filename: a.Filename, ContentType: a.ContentType, SizeBytes: a.SizeBytes, SHA256: a.SHA256,
			})
		}
		out.Versions = append(out.Versions, version)
	}
	return out
}

// mountDeclarationRoutes registers the Erklärung routes (#3430): one
// declaration per child and call, and the guardian's own proof. The auth
// limiter also bounds password guesses when the school asks for them.
func (rs *Resource) mountDeclarationRoutes(r chi.Router, authRateLimiter func(http.Handler) http.Handler) {
	submit := r
	if authRateLimiter != nil {
		submit = r.With(authRateLimiter)
	}
	submit.Post("/me/news/{announcementId}/declaration", rs.submitDeclaration)
	r.Get("/me/news/{announcementId}/declaration/proof", rs.declarationProof)
}
