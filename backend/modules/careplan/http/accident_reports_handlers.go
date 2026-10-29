package careplanhttp

import (
	context "context"
	errors "errors"
	fmt "fmt"
	http "net/http"
	strconv "strconv"
	strings "strings"
	time "time"

	chi "github.com/go-chi/chi/v5"
	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
)

// Accident reports (Unfallanzeige, #4771). The routes mirror the document
// routes: every command runs in its own tenant transaction inside Care Plan,
// so none of them takes the request transaction, and the PDF is written only
// after its data-access log row committed.

// SchoolIdentity is the school's name and address as the accident report
// prints them in its first field. The root binds Organization Tenancy.
type SchoolIdentity interface {
	SchoolIdentity(ctx context.Context, schoolID int64) (name, address string, err error)
}

func (rs *Resource) registerAccidentReportRoutes(r chi.Router) {
	manage := common.RequiresPermission(permissions.StudentAccidentsManage)
	r.With(manage).Post("/{id}/accident-reports", rs.createAccidentReport)
	r.With(manage).Put("/{id}/accident-reports/{documentId}", rs.updateAccidentReport)
	r.With(manage).Post("/{id}/accident-reports/{documentId}/signature", rs.signAccidentReport)
	r.With(manage).Post("/{id}/accident-reports/{documentId}/paper-copy", rs.uploadAccidentPaperCopy)
	r.With(manage).Post("/{id}/accident-reports/{documentId}/corrections", rs.correctAccidentReport)
	r.With(common.RequiresPermission(permissions.StudentAccidentsRead)).
		Get("/{id}/accident-reports/{documentId}/pdf", rs.downloadAccidentReportPDF)
}

// AccidentReportRequest is the recorded accident on the wire.
type AccidentReportRequest struct {
	OccurredAt        time.Time  `json:"occurred_at"`
	Location          string     `json:"location"`
	Description       string     `json:"description"`
	InjuredBodyParts  string     `json:"injured_body_parts"`
	InjuryKind        string     `json:"injury_kind"`
	Witnesses         string     `json:"witnesses"`
	FirstAid          string     `json:"first_aid"`
	DoctorVisited     bool       `json:"doctor_visited"`
	Doctor            string     `json:"doctor"`
	ParentsNotified   string     `json:"parents_notified"`
	ParentsNotifiedAt *time.Time `json:"parents_notified_at"`
	Supervisor        string     `json:"supervisor"`
}

// Bind leaves the field rules to Care Plan; only a missing time is refused
// here, because the zero time would read as a date.
func (req *AccidentReportRequest) Bind(_ *http.Request) error {
	if occurredAt := req.OccurredAt; occurredAt.IsZero() {
		return fmt.Errorf("%w: occurred_at is required", careplan.ErrAccidentReportInvalid)
	}
	return nil
}

func (req *AccidentReportRequest) report() careplan.AccidentReport {
	return careplan.AccidentReport{
		OccurredAt: req.OccurredAt, Location: req.Location, Description: req.Description,
		InjuredBodyParts: req.InjuredBodyParts, InjuryKind: req.InjuryKind, Witnesses: req.Witnesses,
		FirstAid: req.FirstAid, DoctorVisited: req.DoctorVisited, Doctor: req.Doctor,
		ParentsNotified: req.ParentsNotified, ParentsNotifiedAt: req.ParentsNotifiedAt, Supervisor: req.Supervisor,
	}
}

// AccidentSignatureRequest is the leadership's signature: a drawn image, or
// the typed name the client rendered into the same image.
type AccidentSignatureRequest struct {
	Method    string `json:"method"`
	ImagePNG  string `json:"image_png"`
	TypedName string `json:"typed_name"`
}

func (req *AccidentSignatureRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(req.ImagePNG) == "" {
		return fmt.Errorf("%w: image_png is required", careplan.ErrAccidentSignatureInvalid)
	}
	return nil
}

// AccidentReportResponse is the accident report part of a document on the
// wire. Version counts the report and the reports it corrects.
type AccidentReportResponse struct {
	Status            string     `json:"status"`
	Version           int        `json:"version,omitempty"`
	SupersedesID      *string    `json:"supersedes_id,omitempty"`
	SupersededByID    *string    `json:"superseded_by_id,omitempty"`
	SignedAt          *time.Time `json:"signed_at,omitempty"`
	SignedDigitally   bool       `json:"signed_digitally"`
	OccurredAt        time.Time  `json:"occurred_at"`
	Location          string     `json:"location"`
	Description       string     `json:"description"`
	InjuredBodyParts  string     `json:"injured_body_parts"`
	InjuryKind        string     `json:"injury_kind"`
	Witnesses         string     `json:"witnesses"`
	FirstAid          string     `json:"first_aid"`
	DoctorVisited     bool       `json:"doctor_visited"`
	Doctor            string     `json:"doctor"`
	ParentsNotified   string     `json:"parents_notified"`
	ParentsNotifiedAt *time.Time `json:"parents_notified_at,omitempty"`
	Supervisor        string     `json:"supervisor"`
}

func newAccidentReportResponse(doc careplan.CareDocument, version int, supersededBy *int64) *AccidentReportResponse {
	report := doc.AccidentReport
	if report == nil {
		return nil
	}
	return &AccidentReportResponse{
		Status: careplan.AccidentReportStatus(doc), Version: version,
		SupersedesID: idString(doc.AccidentSupersedesID), SupersededByID: idString(supersededBy),
		SignedAt: doc.AccidentSignedAt, SignedDigitally: doc.AccidentSignatureID != nil,
		OccurredAt: report.OccurredAt, Location: report.Location, Description: report.Description,
		InjuredBodyParts: report.InjuredBodyParts, InjuryKind: report.InjuryKind, Witnesses: report.Witnesses,
		FirstAid: report.FirstAid, DoctorVisited: report.DoctorVisited, Doctor: report.Doctor,
		ParentsNotified: report.ParentsNotified, ParentsNotifiedAt: report.ParentsNotifiedAt, Supervisor: report.Supervisor,
	}
}

func idString(id *int64) *string {
	if id == nil {
		return nil
	}
	value := strconv.FormatInt(*id, 10)
	return &value
}

// accidentReportVersions numbers each report of the list by its chain of
// corrections and names the report that corrects it.
func accidentReportVersions(docs []careplan.CareDocument) (map[int64]int, map[int64]int64) {
	predecessors := make(map[int64]int64, len(docs))
	successors := make(map[int64]int64, len(docs))
	for _, doc := range docs {
		if doc.AccidentSupersedesID != nil {
			predecessors[doc.ID] = *doc.AccidentSupersedesID
			successors[*doc.AccidentSupersedesID] = doc.ID
		}
	}
	versions := make(map[int64]int, len(docs))
	for _, doc := range docs {
		if doc.AccidentReport == nil {
			continue
		}
		version, id := 1, doc.ID
		for steps := 0; steps < len(docs); steps++ {
			previous, ok := predecessors[id]
			if !ok {
				break
			}
			version, id = version+1, previous
		}
		versions[doc.ID] = version
	}
	return versions, successors
}

// accidentReportErrorRenderer answers each accident report failure with its
// own code, so the frontend words it from the catalog (ADR 0006).
var accidentReportErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: careplan.ErrStudentDocumentForbidden, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeStudentsAccidentReportForbidden)
	}},
	{Target: careplan.ErrStudentDocumentNoAccess, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeStudentsAccidentReportForbidden)
	}},
	{Target: careplan.ErrAccidentReportInvalid, Render: invalidWithCode(common.CodeStudentsAccidentReportInvalid)},
	{Target: careplan.ErrStudentDocumentInvalid, Render: invalidWithCode(common.CodeStudentsAccidentReportInvalid)},
	{Target: careplan.ErrAccidentSignatureInvalid, Render: invalidWithCode(common.CodeStudentsAccidentSignatureInvalid)},
	{Target: careplan.ErrAccidentReportSigned, Render: conflictWithCode(common.CodeStudentsAccidentReportSigned)},
	{Target: careplan.ErrAccidentReportNotSigned, Render: conflictWithCode(common.CodeStudentsAccidentReportNotSigned)},
	{Target: careplan.ErrAccidentReportSuperseded, Render: conflictWithCode(common.CodeStudentsAccidentReportSuperseded)},
	{Target: careplan.ErrAccidentReportNotFound, Render: notFoundWithCode(common.CodeStudentsAccidentReportNotFound)},
	{Match: isStudentDocumentNotFound, Render: notFoundWithCode(common.CodeStudentsAccidentReportNotFound)},
}, common.ErrorInternalServer)

// accidentReportTarget parses the child and the report of the URL and the
// caller, rendering the failure itself.
func accidentReportTarget(w http.ResponseWriter, r *http.Request, withDocument bool) (int64, int64, careplan.StudentDocumentActor, bool) {
	studentID, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStudentID)
	if !ok {
		return 0, 0, careplan.StudentDocumentActor{}, false
	}
	var documentID int64
	if withDocument {
		if documentID, ok = common.ParseInt64IDWithError(w, r, "documentId", "invalid document ID"); !ok {
			return 0, 0, careplan.StudentDocumentActor{}, false
		}
	}
	actor, err := studentDocumentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return 0, 0, careplan.StudentDocumentActor{}, false
	}
	return studentID, documentID, actor, true
}

func (rs *Resource) respondAccidentReport(w http.ResponseWriter, r *http.Request, status int, doc careplan.CareDocument, err error, message string) {
	if err != nil {
		common.RenderError(w, r, accidentReportErrorRenderer(err))
		return
	}
	resp := newStudentDocumentResponse(doc)
	// A write answers without the version: only the list reads the chain of
	// corrections that numbers it.
	resp.AccidentReport = newAccidentReportResponse(doc, 0, nil)
	common.Respond(w, r, status, &resp, message)
}

// createAccidentReport serves POST /{id}/accident-reports.
func (rs *Resource) createAccidentReport(w http.ResponseWriter, r *http.Request) {
	studentID, _, actor, ok := accidentReportTarget(w, r, false)
	if !ok {
		return
	}
	req := &AccidentReportRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeStudentsAccidentReportInvalid))
		return
	}
	doc, err := rs.StudentAccidentReports.CreateStudentAccidentReport(r.Context(), studentID, req.report(), actor)
	rs.respondAccidentReport(w, r, http.StatusCreated, doc, err, "Accident report recorded")
}

// updateAccidentReport serves PUT /{id}/accident-reports/{documentId}.
func (rs *Resource) updateAccidentReport(w http.ResponseWriter, r *http.Request) {
	studentID, documentID, actor, ok := accidentReportTarget(w, r, true)
	if !ok {
		return
	}
	req := &AccidentReportRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeStudentsAccidentReportInvalid))
		return
	}
	doc, err := rs.StudentAccidentReports.UpdateStudentAccidentReport(r.Context(), studentID, documentID, req.report(), actor)
	rs.respondAccidentReport(w, r, http.StatusOK, doc, err, "Accident report updated")
}

// correctAccidentReport serves POST /{id}/accident-reports/{documentId}/corrections.
func (rs *Resource) correctAccidentReport(w http.ResponseWriter, r *http.Request) {
	studentID, documentID, actor, ok := accidentReportTarget(w, r, true)
	if !ok {
		return
	}
	req := &AccidentReportRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeStudentsAccidentReportInvalid))
		return
	}
	doc, err := rs.StudentAccidentReports.CorrectStudentAccidentReport(r.Context(), studentID, documentID, req.report(), actor)
	rs.respondAccidentReport(w, r, http.StatusCreated, doc, err, "Accident report corrected")
}

// signAccidentReport serves POST /{id}/accident-reports/{documentId}/signature.
func (rs *Resource) signAccidentReport(w http.ResponseWriter, r *http.Request) {
	studentID, documentID, actor, ok := accidentReportTarget(w, r, true)
	if !ok {
		return
	}
	req := &AccidentSignatureRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeStudentsAccidentSignatureInvalid))
		return
	}
	doc, err := rs.StudentAccidentReports.SignStudentAccidentReport(r.Context(), studentID, documentID, careplan.AccidentSignatureInput{
		Method: req.Method, ImagePNG: req.ImagePNG, TypedName: req.TypedName,
	}, actor)
	rs.respondAccidentReport(w, r, http.StatusOK, doc, err, "Accident report signed")
}

// uploadAccidentPaperCopy serves POST
// /{id}/accident-reports/{documentId}/paper-copy (multipart: file). It runs
// the document upload: authorize, queue the cleanup intent, write the object
// within the deadline, then commit the metadata, here as the report's signed
// paper copy.
func (rs *Resource) uploadAccidentPaperCopy(w http.ResponseWriter, r *http.Request) {
	documentID, ok := common.ParseInt64IDWithError(w, r, "documentId", "invalid document ID")
	if !ok {
		return
	}
	upload, ok := rs.openStudentDocumentUpload(w, r)
	if !ok {
		return
	}
	defer common.CloseFile(upload.uploaded.File)
	if err := rs.StudentAccidentReports.AuthorizeAccidentPaperCopy(r.Context(), upload.studentID, documentID, upload.actor); err != nil {
		common.RenderError(w, r, accidentReportErrorRenderer(err))
		return
	}
	if !rs.queueStudentDocumentObject(w, r, upload) {
		return
	}
	rs.storeStudentDocumentObject(w, r, upload, func(ctx context.Context, size int64) (careplan.CareDocument, error) {
		return rs.StudentAccidentReports.AttachAccidentPaperCopy(ctx, careplan.AccidentPaperCopyInput{
			StudentID: upload.studentID, DocumentID: documentID, FilenameDisplay: upload.uploaded.Filename,
			FilenameStored: upload.storedName, SizeBytes: size, ContentType: upload.uploaded.ContentType,
		}, upload.actor)
	}, func(err error) bool {
		return errors.Is(err, careplan.ErrStudentDocumentInvalid) || errors.Is(err, careplan.ErrStudentDocumentForbidden) ||
			errors.Is(err, careplan.ErrAccidentReportSigned)
	}, accidentReportErrorRenderer)
}

// downloadAccidentReportPDF serves GET /{id}/accident-reports/{documentId}/pdf:
// the report in the layout of the official form, with the digital signature
// when there is one.
func (rs *Resource) downloadAccidentReportPDF(w http.ResponseWriter, r *http.Request) {
	if rs.Reports == nil || rs.Schools == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("accident report rendering is not bound")))
		return
	}
	studentID, documentID, actor, ok := accidentReportTarget(w, r, true)
	if !ok {
		return
	}
	file, err := rs.StudentAccidentReports.ResolveStudentAccidentReportFile(r.Context(), studentID, documentID, actor)
	if err != nil {
		common.RenderError(w, r, accidentReportErrorRenderer(err))
		return
	}
	child, err := rs.accidentReportChild(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("read accident report child", err))
		return
	}
	schoolName, schoolAddress, err := rs.Schools.SchoolIdentity(r.Context(), file.Document.TenantID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("read accident report school", err))
		return
	}
	child.SchoolName, child.SchoolAddress = schoolName, schoolAddress
	rendered, err := rs.Reports.RenderReport(accidentReportDocument(file, child, rs.Now()), "unfallanzeige")
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("render accident report", err))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", rendered.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, rendered.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(rendered.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rendered.Data)
}

// accidentReportChild reads the child facts the form's fields 5 to 7 print.
func (rs *Resource) accidentReportChild(ctx context.Context, studentID int64) (accidentReportFacts, error) {
	student, err := rs.findStudent(ctx, studentID)
	if err != nil {
		return accidentReportFacts{}, err
	}
	person, err := rs.findPerson(ctx, student.PersonID)
	if err != nil {
		return accidentReportFacts{}, err
	}
	facts := accidentReportFacts{
		Name: strings.TrimSpace(person.LastName + ", " + person.FirstName), Birthday: person.Birthday,
		SchoolClass: student.SchoolClass,
	}
	facts.Address = joinNonEmpty(", ", derefString(student.AddressStreet),
		joinNonEmpty(" ", derefString(student.AddressPostalCode), derefString(student.AddressCity)))
	return facts, nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func joinNonEmpty(separator string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, separator)
}
