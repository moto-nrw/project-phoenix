package careplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Accident reports (Unfallanzeige, #4771). The team records a child's
// accident in the fields of the official form for children in day care,
// pupils and students; the report is a child document of the category
// unfallanzeige and its PDF is rendered from what was recorded. The
// leadership signs it digitally with the shared signature store or uploads
// the signed paper copy. A signed report never changes again: a correction
// is a new report naming the one it corrects, and the old one stays.
// moto never sends a report to the accident insurer; the school downloads
// the PDF and passes it on.

// AccidentReport is what the team recorded about one accident.
type AccidentReport struct {
	// OccurredAt is the time of the accident (form field 12).
	OccurredAt time.Time `json:"occurred_at"`
	// Location is the exact place (13).
	Location string `json:"location"`
	// Description is the detailed course of events (14).
	Description string `json:"description"`
	// InjuredBodyParts names the injured parts of the body (15).
	InjuredBodyParts string `json:"injured_body_parts"`
	// InjuryKind is the kind of injury (16).
	InjuryKind string `json:"injury_kind"`
	// Witnesses names who noticed the accident first and the witnesses (19).
	Witnesses string `json:"witnesses"`
	// FirstAid records the first aid given.
	FirstAid string `json:"first_aid"`
	// DoctorVisited records whether a doctor treated the child; Doctor names
	// the doctor or hospital (20).
	DoctorVisited bool   `json:"doctor_visited"`
	Doctor        string `json:"doctor"`
	// ParentsNotified names the parents who were told and ParentsNotifiedAt
	// when.
	ParentsNotified   string     `json:"parents_notified"`
	ParentsNotifiedAt *time.Time `json:"parents_notified_at,omitempty"`
	// Supervisor is the staff member who supervised the child.
	Supervisor string `json:"supervisor"`
}

// Text limits of a report. A course of events may be long; every other field
// is a line or a short paragraph.
const (
	AccidentReportShortTextMax   = 500
	AccidentReportDescriptionMax = 5000
	// AccidentReportClockSkew is how far a device clock may run ahead: an
	// accident later than that has not happened yet.
	AccidentReportClockSkew = 10 * time.Minute
)

// AccidentReport statuses as the Dokumente tab shows them.
const (
	AccidentReportStatusOpen   = "open"
	AccidentReportStatusSigned = "signed"
)

var (
	// ErrAccidentReportInvalid marks a report whose fields are missing or too
	// long.
	ErrAccidentReportInvalid = errors.New("invalid accident report")
	// ErrAccidentReportSigned marks a change of a report that is already
	// signed; only a correction may follow it.
	ErrAccidentReportSigned = errors.New("accident report is signed")
	// ErrAccidentReportNotSigned marks a correction of a report that is still
	// open and can be changed directly.
	ErrAccidentReportNotSigned = errors.New("accident report is not signed yet")
	// ErrAccidentReportSuperseded marks a correction of a report that already
	// has a newer version.
	ErrAccidentReportSuperseded = errors.New("accident report already has a newer version")
	// ErrAccidentReportNotFound marks a document that is no accident report.
	ErrAccidentReportNotFound = errors.New("accident report not found")
	// ErrAccidentSignatureInvalid marks a signature image the signature store
	// refused.
	ErrAccidentSignatureInvalid = errors.New("invalid accident report signature")
)

// NormalizeAccidentReport trims the report and checks every field the
// official form needs.
func NormalizeAccidentReport(report AccidentReport) (AccidentReport, error) {
	report.Location = strings.TrimSpace(report.Location)
	report.Description = strings.TrimSpace(report.Description)
	report.InjuredBodyParts = strings.TrimSpace(report.InjuredBodyParts)
	report.InjuryKind = strings.TrimSpace(report.InjuryKind)
	report.Witnesses = strings.TrimSpace(report.Witnesses)
	report.FirstAid = strings.TrimSpace(report.FirstAid)
	report.Doctor = strings.TrimSpace(report.Doctor)
	report.ParentsNotified = strings.TrimSpace(report.ParentsNotified)
	report.Supervisor = strings.TrimSpace(report.Supervisor)
	if !report.DoctorVisited {
		report.Doctor = ""
	}
	if report.ParentsNotified == "" {
		report.ParentsNotifiedAt = nil
	}
	if report.OccurredAt.IsZero() {
		return AccidentReport{}, fmt.Errorf("%w: occurred_at is required", ErrAccidentReportInvalid)
	}
	// Instants are kept in UTC so the signed statement reads the same after
	// the report went through the database.
	report.OccurredAt = report.OccurredAt.UTC()
	if report.ParentsNotifiedAt != nil {
		at := report.ParentsNotifiedAt.UTC()
		report.ParentsNotifiedAt = &at
	}
	if report.OccurredAt.After(time.Now().Add(AccidentReportClockSkew)) {
		return AccidentReport{}, fmt.Errorf("%w: occurred_at lies in the future", ErrAccidentReportInvalid)
	}
	for _, field := range []struct{ name, value string }{
		{"location", report.Location}, {"description", report.Description},
		{"injury_kind", report.InjuryKind}, {"supervisor", report.Supervisor},
	} {
		if field.value == "" {
			return AccidentReport{}, fmt.Errorf("%w: %s is required", ErrAccidentReportInvalid, field.name)
		}
	}
	if report.DoctorVisited && report.Doctor == "" {
		return AccidentReport{}, fmt.Errorf("%w: doctor is required when a doctor treated the child", ErrAccidentReportInvalid)
	}
	for _, field := range []struct{ name, value string }{
		{"location", report.Location}, {"injured_body_parts", report.InjuredBodyParts}, {"injury_kind", report.InjuryKind},
		{"witnesses", report.Witnesses}, {"first_aid", report.FirstAid}, {"doctor", report.Doctor},
		{"parents_notified", report.ParentsNotified}, {"supervisor", report.Supervisor},
	} {
		if utf8.RuneCountInString(field.value) > AccidentReportShortTextMax {
			return AccidentReport{}, fmt.Errorf("%w: %s exceeds %d characters", ErrAccidentReportInvalid, field.name, AccidentReportShortTextMax)
		}
	}
	if utf8.RuneCountInString(report.Description) > AccidentReportDescriptionMax {
		return AccidentReport{}, fmt.Errorf("%w: description exceeds %d characters", ErrAccidentReportInvalid, AccidentReportDescriptionMax)
	}
	return report, nil
}

// AccidentReportStatement is the exact content a signature is given on: the
// document, the child and every recorded field. Its SHA-256 is the
// signature's document checksum, so a report that changed after signing no
// longer matches.
func AccidentReportStatement(document CareDocument) (string, error) {
	if document.AccidentReport == nil {
		return "", ErrAccidentReportNotFound
	}
	encoded, err := json.Marshal(struct {
		Format     string         `json:"format"`
		DocumentID int64          `json:"document_id"`
		StudentID  int64          `json:"student_id"`
		Supersedes *int64         `json:"supersedes,omitempty"`
		Report     AccidentReport `json:"report"`
	}{
		Format: "moto.accident_report.v1", DocumentID: document.ID, StudentID: document.StudentID,
		Supersedes: document.AccidentSupersedesID, Report: *document.AccidentReport,
	})
	if err != nil {
		return "", fmt.Errorf("encode accident report statement: %w", err)
	}
	return string(encoded), nil
}

// AccidentReportStatus is open until the report is signed digitally or by
// its paper copy.
func AccidentReportStatus(document CareDocument) string {
	if document.AccidentSignedAt != nil {
		return AccidentReportStatusSigned
	}
	return AccidentReportStatusOpen
}

// AccidentSignatureInput is the leadership's signature as the client sent
// it: drawn, or the typed name rendered into the same image.
type AccidentSignatureInput struct {
	Method    string
	ImagePNG  string
	TypedName string
}

// AccidentSignatureEntry is one signature to store for an accident report.
type AccidentSignatureEntry struct {
	StudentID       int64
	SignerAccountID int64
	SignerName      string
	Statement       string
	SignedAt        time.Time
	Input           AccidentSignatureInput
}

// AccidentSignature is a stored signature of an accident report. Intact is
// false when the image, the row or the report it was given on no longer
// matches its checksum, or it belongs to another child or purpose.
type AccidentSignature struct {
	Method     string
	TypedName  string
	SignerName string
	PNG        []byte
	SignedAt   time.Time
	Intact     bool
}

// AccidentPaperCopyInput carries the stored signed paper copy of a report.
type AccidentPaperCopyInput struct {
	StudentID       int64
	DocumentID      int64
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
}

// AccidentReportFile is one report as its PDF shows it: the document and, for
// a digital signature, the signature with its check.
type AccidentReportFile struct {
	Document  CareDocument
	Signature *AccidentSignature
}

// StudentAccidentReports is the accident report capability. Reading needs
// student_accidents:read or :manage, every write student_accidents:manage;
// each command commits its own tenant transaction with its change-history row.
type StudentAccidentReports interface {
	// CreateStudentAccidentReport records a new open report.
	CreateStudentAccidentReport(ctx context.Context, studentID int64, report AccidentReport, actor StudentDocumentActor) (CareDocument, error)
	// UpdateStudentAccidentReport changes an open report.
	UpdateStudentAccidentReport(ctx context.Context, studentID, documentID int64, report AccidentReport, actor StudentDocumentActor) (CareDocument, error)
	// SignStudentAccidentReport signs an open report digitally.
	SignStudentAccidentReport(ctx context.Context, studentID, documentID int64, signature AccidentSignatureInput, actor StudentDocumentActor) (CareDocument, error)
	// CorrectStudentAccidentReport records a new open version of a signed
	// report; the signed one stays as it is.
	CorrectStudentAccidentReport(ctx context.Context, studentID, documentID int64, report AccidentReport, actor StudentDocumentActor) (CareDocument, error)
	// AuthorizeAccidentPaperCopy answers whether the caller may attach a paper
	// copy to the report WITHOUT writing anything.
	AuthorizeAccidentPaperCopy(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) error
	// AttachAccidentPaperCopy signs an open report with its uploaded paper
	// copy.
	AttachAccidentPaperCopy(ctx context.Context, input AccidentPaperCopyInput, actor StudentDocumentActor) (CareDocument, error)
	// ResolveStudentAccidentReportFile authorizes the PDF download, writes its
	// data-access log row and reads the signature.
	ResolveStudentAccidentReportFile(ctx context.Context, studentID, documentID int64, actor StudentDocumentActor) (AccidentReportFile, error)
}
