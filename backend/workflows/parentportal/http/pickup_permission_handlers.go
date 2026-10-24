package parent

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	common "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Pickup permissions (#4780): a signed request that lets a contact person
// collect the child on every day or on some weekdays, the list of those
// requests, and the guardian's proof of any signed request.

// pickupProcedureLabel names the procedure on the proof.
const pickupProcedureLabel = "Einfache elektronische Signatur mit gezeichneter Unterschrift"

// pickupPermissionRequestBody is the wire shape for POST
// .../guardians/{guardianProfileId}/pickup-permission-requests.
type pickupPermissionRequestBody struct {
	// Weekdays are the plan weekdays (1 = Monday … 5 = Friday); empty or all
	// five means every day.
	Weekdays  []int                        `json:"weekdays"`
	Reason    string                       `json:"reason"`
	Signature *declarationSignatureRequest `json:"signature,omitempty"`
}

// pickupPermissionRequestResponse is one pickup permission request.
type pickupPermissionRequestResponse struct {
	ID                string     `json:"id"`
	GuardianProfileID string     `json:"guardian_profile_id"`
	PersonName        string     `json:"person_name"`
	Weekdays          []int      `json:"weekdays"`
	Status            string     `json:"status"`
	DecisionReason    *string    `json:"decision_reason,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
	IsSelf            bool       `json:"is_self"`
	Signed            bool       `json:"signed"`
}

func (s *declarationSignatureRequest) pickupInput() *parentService.PickupSignatureInput {
	if s == nil {
		return nil
	}
	return &parentService.PickupSignatureInput{Method: s.Method, ImagePNG: s.ImagePNG, TypedName: s.TypedName}
}

func toPickupPermissionRequestResponse(req *carerequests.Request, accountID int64) (pickupPermissionRequestResponse, bool) {
	permission, err := carerequests.ParsePickupPermission(req.Payload)
	if err != nil {
		return pickupPermissionRequestResponse{}, false
	}
	weekdays := permission.Weekdays
	if weekdays == nil {
		weekdays = []int{}
	}
	return pickupPermissionRequestResponse{
		ID: strconv.FormatInt(req.ID, 10), GuardianProfileID: strconv.FormatInt(permission.GuardianProfileID, 10),
		PersonName: permission.PersonName, Weekdays: weekdays, Status: req.Status, DecisionReason: req.DecisionReason,
		CreatedAt: req.CreatedAt, ReviewedAt: req.ReviewedAt, IsSelf: req.SubmittedBy == accountID,
		Signed: permission.Signature != nil,
	}, true
}

func (rs *Resource) submitPickupPermissionRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	guardianProfileID, ok := parsePositiveID(chi.URLParam(r, "guardianProfileId"))
	if !ok {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid guardian profile id")))
		return
	}
	var body pickupPermissionRequestBody
	if !decodeEditBody(w, r, &body) {
		return
	}
	created, err := rs.ParentService.SubmitPickupPermissionRequest(r.Context(), accountID, studentID, guardianProfileID, parentService.PickupPermissionInput{
		Weekdays: body.Weekdays, Reason: body.Reason, Signature: body.Signature.pickupInput(),
	})
	if err != nil {
		renderParentRequestError(w, r, err)
		return
	}
	response, ok := toPickupPermissionRequestResponse(created, accountID)
	if !ok {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("invalid pickup permission request payload")))
		return
	}
	common.Respond(w, r, http.StatusCreated, response, "Pickup permission request submitted")
}

func (rs *Resource) listPickupPermissionRequests(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	rows, err := rs.ParentService.ListPickupPermissionRequests(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	out := make([]pickupPermissionRequestResponse, 0, len(rows))
	for i := range rows {
		if response, ok := toPickupPermissionRequestResponse(&rows[i], accountID); ok {
			out = append(out, response)
		}
	}
	common.Respond(w, r, http.StatusOK, out, "Pickup permission requests retrieved")
}

// pickupProofResponse is the guardian's proof of one signed request.
type pickupProofResponse struct {
	RequestID   string                `json:"request_id"`
	Kind        string                `json:"kind"`
	ChildName   string                `json:"child_name"`
	SchoolName  string                `json:"school_name"`
	Procedure   string                `json:"procedure"`
	Statement   string                `json:"statement"`
	Summary     []pickupProofLine     `json:"summary"`
	Status      string                `json:"status"`
	CreatedAt   time.Time             `json:"created_at"`
	ReviewedAt  *time.Time            `json:"reviewed_at,omitempty"`
	Signature   *pickupProofSignature `json:"signature,omitempty"`
	IntegrityOK bool                  `json:"integrity_ok"`
	GeneratedAt time.Time             `json:"generated_at"`
}

type pickupProofLine struct {
	Label string `json:"label"`
	Old   string `json:"old,omitempty"`
	New   string `json:"new"`
}

type pickupProofSignature struct {
	Method       string    `json:"method"`
	TypedName    string    `json:"typed_name,omitempty"`
	SignerName   string    `json:"signer_name"`
	ImageDataURL string    `json:"image_data_url"`
	SignedAt     time.Time `json:"signed_at"`
}

// pickupRequestProof returns the guardian's proof of one signed request:
// JSON for the portal page, or with format=pdf the moto PDF.
func (rs *Resource) pickupRequestProof(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	requestID, ok := parsePositiveID(chi.URLParam(r, "requestId"))
	if !ok {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request id")))
		return
	}
	proof, err := rs.ParentService.PickupRequestProof(r.Context(), accountID, studentID, requestID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("format") == "pdf" {
		rs.writePickupProofPDF(w, r, proof)
		return
	}
	common.Respond(w, r, http.StatusOK, toPickupProofResponse(proof, time.Now()), "Pickup permission proof retrieved")
}

func toPickupProofResponse(proof *parentService.PickupProof, now time.Time) pickupProofResponse {
	out := pickupProofResponse{
		RequestID: strconv.FormatInt(proof.Request.ID, 10), Kind: proof.Request.RequestKind,
		ChildName: proof.ChildName, SchoolName: proof.SchoolName, Procedure: pickupProcedureLabel,
		Statement: proof.Statement, Summary: make([]pickupProofLine, 0, len(proof.Summary)),
		Status: proof.Request.Status, CreatedAt: proof.Request.CreatedAt, ReviewedAt: proof.Request.ReviewedAt,
		IntegrityOK: proof.Intact, GeneratedAt: now,
	}
	for _, line := range proof.Summary {
		out.Summary = append(out.Summary, pickupProofLine{Label: line.Label, Old: line.Old, New: line.New})
	}
	if s := proof.Signature; s != nil {
		out.Signature = &pickupProofSignature{
			Method: s.Method, TypedName: s.TypedName, SignerName: s.SignerName, SignedAt: s.SignedAt,
			ImageDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(s.PNG),
		}
	}
	return out
}

var pickupProofStatusLabels = map[string]string{
	carerequests.StatusPending:   "Wartet auf die OGS",
	carerequests.StatusApproved:  "Von der OGS bestätigt",
	carerequests.StatusRejected:  "Von der OGS abgelehnt",
	carerequests.StatusWithdrawn: "Zurückgezogen",
	carerequests.StatusDone:      "Erledigt",
	carerequests.StatusCareEnded: "Betreuung beendet",
}

// pickupProofReport describes the guardian's proof as a PDF in the moto
// design. German, like the school's own record.
func pickupProofReport(proof *parentService.PickupProof, now time.Time) ReportDocument {
	integrity := "Unterschrift und Text sind unverändert."
	if !proof.Intact {
		integrity = "Achtung: Ein gespeicherter Eintrag wurde nachträglich verändert. Bitte wenden Sie sich an die OGS."
	}
	overview := ReportCard{Title: "Erlaubnis", Fields: []ReportField{
		{Label: "Kind", Value: proof.ChildName},
		{Label: "Schule", Value: proof.SchoolName},
		{Label: "Erklärung", Value: proof.Statement},
		{Label: "Stand", Value: labelOr(pickupProofStatusLabels, proof.Request.Status)},
		{Label: "Verfahren", Value: pickupProcedureLabel},
		{Label: "Prüfung", Value: integrity},
	}}
	for _, line := range proof.Summary {
		value := line.New
		if line.Old != "" {
			value = line.Old + " → " + line.New
		}
		overview.Fields = append(overview.Fields, ReportField{Label: line.Label, Value: value})
	}
	cards := []ReportCard{overview}
	if s := proof.Signature; s != nil {
		typed := ""
		if s.TypedName != "" {
			typed = "Name getippt: " + s.TypedName
		}
		cards = append(cards, ReportCard{Title: "Unterschrift am " + proofStamp(s.SignedAt), Fields: []ReportField{
			{Label: "Von", Value: s.SignerName},
			{Label: "Unterschrift", Value: typed, PNG: s.PNG},
		}})
	}
	return ReportDocument{
		Title: "Nachweis Ihrer Erlaubnis", Subtitle: proof.ChildName, GeneratedAt: now,
		Filters:  []string{proof.ChildName, proof.SchoolName},
		Footer:   "Vertraulich: enthält personenbezogene Daten",
		Sections: []ReportSection{{Title: "Überblick", Cards: cards}},
	}
}

func (rs *Resource) writePickupProofPDF(w http.ResponseWriter, r *http.Request, proof *parentService.PickupProof) {
	if rs.Reports == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errProofRendererUnbound))
		return
	}
	file, err := rs.Reports.RenderReport(pickupProofReport(proof, time.Now()), "nachweis-abholerlaubnis")
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("render pickup permission proof", err))
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

// renderParentPickupPermissionError maps the pickup permission sentinels
// (#4780) onto their codes. It reports whether it rendered.
func renderParentPickupPermissionError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, parentService.ErrPickupSignatureRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCarePickupSignatureRequired))
	case errors.Is(err, parentService.ErrPickupSignatureInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCarePickupSignatureInvalid))
	case errors.Is(err, parentService.ErrPickupPersonNotLinked):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCarePickupPersonNotLinked))
	case errors.Is(err, parentService.ErrPickupPermissionRequestRequired):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCarePickupPermissionRequestRequired))
	case errors.Is(err, parentService.ErrPickupPermissionUnchanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCarePickupPermissionUnchanged))
	case errors.Is(err, parentService.ErrPickupProofNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeCarePickupProofNotFound))
	default:
		return false
	}
	return true
}

// parsePickupPersonID reads the optional collector of a one-day request.
func parsePickupPersonID(raw *string) (int64, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return 0, nil
	}
	id, ok := parsePositiveID(strings.TrimSpace(*raw))
	if !ok {
		return 0, errors.New("invalid pickup_person_guardian_profile_id")
	}
	return id, nil
}

// mountPickupPermissionRoutes registers the pickup permission routes.
func (rs *Resource) mountPickupPermissionRoutes(r chi.Router) {
	r.Post("/me/children/{studentId}/guardians/{guardianProfileId}/pickup-permission-requests", rs.submitPickupPermissionRequest)
	r.Get("/me/children/{studentId}/pickup-permission-requests", rs.listPickupPermissionRequests)
	r.Get("/me/children/{studentId}/pickup-requests/{requestId}/proof", rs.pickupRequestProof)
}
