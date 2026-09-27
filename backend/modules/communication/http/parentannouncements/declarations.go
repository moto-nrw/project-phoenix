package announcement

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	announcementService "github.com/moto-nrw/project-phoenix/modules/communication"
)

// Erklärungen (#3430): the staff status and the proof export.

type declarationSettingsRequest struct {
	Kind             string `json:"declaration_kind,omitempty"`
	Signers          string `json:"declaration_signers,omitempty"`
	Revocable        bool   `json:"declaration_revocable,omitempty"`
	RequiresPassword bool   `json:"declaration_requires_password,omitempty"`
}

func (d declarationSettingsRequest) settings() announcementService.ParentDeclarationSettings {
	return announcementService.ParentDeclarationSettings(d)
}

type declarationSettingsResponse struct {
	Kind              string `json:"declaration_kind,omitempty"`
	Signers           string `json:"declaration_signers,omitempty"`
	Revocable         *bool  `json:"declaration_revocable,omitempty"`
	RequiresPassword  *bool  `json:"declaration_requires_password,omitempty"`
	LockedAttachments *bool  `json:"declaration_locked_attachments,omitempty"`
}

func toDeclarationSettingsResponse(a *announcementService.ParentAnnouncement) declarationSettingsResponse {
	if a.Declaration.Kind == "" {
		return declarationSettingsResponse{}
	}
	revocable, password, locked := a.Declaration.Revocable, a.Declaration.RequiresPassword, a.DeclarationFrozen
	return declarationSettingsResponse{
		Kind: a.Declaration.Kind, Signers: a.Declaration.Signers,
		Revocable: &revocable, RequiresPassword: &password, LockedAttachments: &locked,
	}
}

type declarationAttachmentResponse struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
}

type declarationVersionResponse struct {
	ID          string                          `json:"id"`
	VersionNo   int                             `json:"version_no"`
	Title       string                          `json:"title"`
	Body        string                          `json:"body"`
	ContentHash string                          `json:"content_hash"`
	PublishedAt time.Time                       `json:"published_at"`
	Attachments []declarationAttachmentResponse `json:"attachments"`
	IntegrityOK bool                            `json:"integrity_ok"`
}

type declarationSignerResponse struct {
	AccountID   string     `json:"account_id"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Action      *string    `json:"action"`
	SubmittedAt *time.Time `json:"submitted_at"`
}

type declarationChildResponse struct {
	StudentID   string                      `json:"student_id"`
	FirstName   string                      `json:"first_name"`
	LastName    string                      `json:"last_name"`
	SchoolClass string                      `json:"school_class"`
	State       string                      `json:"state"`
	Signers     []declarationSignerResponse `json:"signers"`
}

type declarationSubmissionResponse struct {
	ID                string    `json:"id"`
	StudentID         string    `json:"student_id"`
	StudentFirstName  string    `json:"student_first_name"`
	StudentLastName   string    `json:"student_last_name"`
	SignerName        string    `json:"signer_name"`
	GuardianRole      string    `json:"guardian_role"`
	Action            string    `json:"action"`
	Method            string    `json:"method"`
	PasswordConfirmed bool      `json:"password_confirmed"`
	VersionNo         int       `json:"version_no"`
	ContentHash       string    `json:"content_hash"`
	RecordHash        string    `json:"record_hash"`
	SubmittedAt       time.Time `json:"submitted_at"`
	IntegrityOK       bool      `json:"integrity_ok"`
}

type declarationStatusResponse struct {
	Kind             string                          `json:"kind"`
	Signers          string                          `json:"signers"`
	Revocable        bool                            `json:"revocable"`
	RequiresPassword bool                            `json:"requires_password"`
	Deadline         *time.Time                      `json:"deadline"`
	CurrentVersion   *declarationVersionResponse     `json:"current_version"`
	Versions         []declarationVersionResponse    `json:"versions"`
	Summary          map[string]int                  `json:"summary"`
	Children         []declarationChildResponse      `json:"children"`
	Submissions      []declarationSubmissionResponse `json:"submissions"`
	IntegrityOK      bool                            `json:"integrity_ok"`
}

var declarationSummaryStates = []string{
	announcementService.DeclarationStateAgreed, announcementService.DeclarationStateDeclined,
	announcementService.DeclarationStateAcknowledged, announcementService.DeclarationStateRevoked,
	announcementService.DeclarationStatePartial, announcementService.DeclarationStateOpen,
	announcementService.DeclarationStateNoSigner, announcementService.DeclarationStateExpired,
}

func toDeclarationVersionResponse(v announcementService.DeclarationVersion) declarationVersionResponse {
	out := declarationVersionResponse{
		ID: strconv.FormatInt(v.ID, 10), VersionNo: v.VersionNo, Title: v.Title, Body: v.Body, ContentHash: v.ContentHash,
		PublishedAt: v.PublishedAt, IntegrityOK: v.IntegrityOK,
		Attachments: make([]declarationAttachmentResponse, 0, len(v.Attachments)),
	}
	for _, a := range v.Attachments {
		out.Attachments = append(out.Attachments, declarationAttachmentResponse(a))
	}
	return out
}

func toDeclarationStatusResponse(s *announcementService.ParentDeclarationStatus) declarationStatusResponse {
	out := declarationStatusResponse{
		Kind: s.Settings.Kind, Signers: s.Settings.Signers, Revocable: s.Settings.Revocable,
		RequiresPassword: s.Settings.RequiresPassword, Deadline: s.Deadline, IntegrityOK: s.IntegrityAllGood,
		Versions: []declarationVersionResponse{}, Children: []declarationChildResponse{},
		Submissions: []declarationSubmissionResponse{},
		Summary:     map[string]int{"children_total": s.Summary.ChildrenTotal},
	}
	for _, state := range declarationSummaryStates {
		out.Summary[state] = s.Summary.ByState[state]
	}
	for _, v := range s.Versions {
		out.Versions = append(out.Versions, toDeclarationVersionResponse(v))
	}
	if s.CurrentVersion != nil {
		current := toDeclarationVersionResponse(*s.CurrentVersion)
		out.CurrentVersion = &current
	}
	for _, c := range s.Children {
		child := declarationChildResponse{
			StudentID: strconv.FormatInt(c.StudentID, 10), FirstName: c.FirstName, LastName: c.LastName,
			SchoolClass: c.SchoolClass, State: c.State, Signers: make([]declarationSignerResponse, 0, len(c.Signers)),
		}
		for _, signer := range c.Signers {
			child.Signers = append(child.Signers, declarationSignerResponse{
				AccountID: strconv.FormatInt(signer.AccountID, 10), FirstName: signer.FirstName,
				LastName: signer.LastName, Action: signer.Action, SubmittedAt: signer.SubmittedAt,
			})
		}
		out.Children = append(out.Children, child)
	}
	for _, sub := range s.Submissions {
		out.Submissions = append(out.Submissions, declarationSubmissionResponse{
			ID: strconv.FormatInt(sub.ID, 10), StudentID: strconv.FormatInt(sub.StudentID, 10),
			StudentFirstName: sub.StudentFirstName, StudentLastName: sub.StudentLastName,
			SignerName: sub.SignerName, GuardianRole: sub.GuardianRole, Action: sub.Action, Method: sub.Method,
			PasswordConfirmed: sub.PasswordConfirmed, VersionNo: sub.VersionNo, ContentHash: sub.ContentHash,
			RecordHash: sub.RecordHash, SubmittedAt: sub.SubmittedAt, IntegrityOK: sub.IntegrityOK,
		})
	}
	return out
}

func (rs *Resource) declarationStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	status, err := rs.Service.ParentDeclarationStatus(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toDeclarationStatusResponse(status), "Declaration status retrieved")
}

func (rs *Resource) declarationExport(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAnnouncementID(w, r)
	if !ok {
		return
	}
	// The printable proof is rendered by the portal from the status; the
	// export is the machine-readable history.
	if format := r.URL.Query().Get("format"); format != "" && format != "csv" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("format must be csv")))
		return
	}
	status, err := rs.Service.ParentDeclarationStatus(r.Context(), id)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	file, err := declarationCSV(status)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("render declaration export", err))
		return
	}
	w.Header().Set("Content-Type", file.contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.data)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.data)
}

// berlin is the school time zone the export states times in.
var berlin = func() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.FixedZone("CET", 60*60)
	}
	return location
}()

func berlinStamp(t time.Time) string {
	return t.In(berlin).Format("02.01.2006, 15:04:05") + " Uhr"
}

func yesNo(v bool) string {
	if v {
		return "ja"
	}
	return "nein"
}

// declarationCSV is the full history as a spreadsheet: semicolon separated,
// UTF-8 with BOM so German spreadsheet programs open it correctly.
type csvFile struct {
	data        []byte
	contentType string
	filename    string
}

func declarationCSV(s *announcementService.ParentDeclarationStatus) (csvFile, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	rows := [][]string{{"Eintrag", "Kind", "Aktion", "Zeitpunkt", "Erklärt von", "Berechtigung", "Verfahren",
		"Passwort bestätigt", "Fassung", "Prüfsumme Fassung", "Prüfsumme Eintrag", "Eintrag unverändert"}}
	for _, sub := range s.Submissions {
		rows = append(rows, []string{
			strconv.FormatInt(sub.ID, 10), csvSafe(strings.TrimSpace(sub.StudentFirstName + " " + sub.StudentLastName)),
			announcementService.DeclarationActionLabel(sub.Action), berlinStamp(sub.SubmittedAt), csvSafe(sub.SignerName),
			announcementService.GuardianRoleLabel(sub.GuardianRole), announcementService.DeclarationMethodLabel(sub.Method),
			yesNo(sub.PasswordConfirmed), strconv.Itoa(sub.VersionNo), sub.ContentHash, sub.RecordHash, yesNo(sub.IntegrityOK),
		})
	}
	if err := w.WriteAll(rows); err != nil {
		return csvFile{}, err
	}
	return csvFile{data: buf.Bytes(), contentType: "text/csv; charset=utf-8", filename: "nachweis-erklaerung.csv"}, nil
}

// csvSafe neutralises spreadsheet formula injection in user-entered names.
func csvSafe(value string) string {
	if value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}
	return value
}
