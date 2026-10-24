package enrollmenthttp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
)

// The signature of an enrollment (#4786) on the wire: the parent sends it
// with the submission or the edit, the staff detail shows it, and both the
// staff and the parent behind the status link download a proof as a PDF.
// The procedure is a simple electronic signature with a drawn signature.

// SubmitSignatureRequest is the parent's signature in a submit or edit body.
type SubmitSignatureRequest struct {
	Method    string `json:"method"`
	ImagePNG  string `json:"image_png"`
	TypedName string `json:"typed_name,omitempty"`
}

func (s *SubmitSignatureRequest) value() *capability.SubmitSignature {
	if s == nil {
		return nil
	}
	return &capability.SubmitSignature{Method: s.Method, ImagePNG: s.ImagePNG, TypedName: s.TypedName}
}

// RequestSignatureResponse describes a stored signature. ImagePNG is only
// sent to the staff detail; the status page shows that and how the request
// was signed and offers the proof.
type RequestSignatureResponse struct {
	Method         string    `json:"method"`
	TypedName      string    `json:"typed_name,omitempty"`
	SignerName     string    `json:"signer_name"`
	SignedAt       time.Time `json:"signed_at"`
	DocumentSHA256 string    `json:"document_sha256"`
	IntegrityOK    bool      `json:"integrity_ok"`
	ImagePNG       string    `json:"image_png,omitempty"`
}

func newRequestSignatureResponse(s *capability.RequestSignature, withImage bool) *RequestSignatureResponse {
	if s == nil {
		return nil
	}
	out := &RequestSignatureResponse{
		Method: s.Method, TypedName: s.TypedName, SignerName: s.SignerName, SignedAt: s.SignedAt,
		DocumentSHA256: s.DocumentSHA256, IntegrityOK: s.IntegrityOK,
	}
	if withImage {
		out.ImagePNG = base64.StdEncoding.EncodeToString(s.PNG)
	}
	return out
}

// attachAdminSignature adds the request's signature to the staff detail.
func (rs *Resource) attachAdminSignature(ctx context.Context, detail *AdminRequestDetail, requestID int64) error {
	if rs.RequestService == nil {
		return nil
	}
	signature, err := rs.RequestService.RequestSignature(ctx, requestID)
	if err != nil {
		return err
	}
	detail.Signature = newRequestSignatureResponse(signature, true)
	return nil
}

// signatureProof is what a proof PDF prints.
type signatureProof struct {
	Request   *Request
	Children  []*RequestChild
	Guardians []*capability.RequestGuardian
	PhaseName string
	Signature *capability.RequestSignature
}

var errSignatureProofMissing = errors.New("enrollment request has no signature")

// getAdminRequestSignatureProof downloads the staff proof of a signed request.
func (rs *Resource) getAdminRequestSignatureProof(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil || rs.RequestService == nil || rs.ListExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment signature proof not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	var proof signatureProof
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		summary, err := rs.DecisionService.Get(ctx, id)
		if err != nil {
			return err
		}
		signature, err := rs.RequestService.RequestSignature(ctx, id)
		if err != nil {
			return err
		}
		if signature == nil {
			return errSignatureProofMissing
		}
		proof = signatureProof{Request: summary.Request, Children: summary.Children, Guardians: summary.Guardians, Signature: signature}
		if summary.Phase != nil {
			proof.PhaseName = summary.Phase.Name
		}
		return nil
	})
	switch {
	case errors.Is(err, capability.ErrDecisionRequestNotFound):
		common.RenderError(w, r, renderRequestNotFound(err))
		return
	case errors.Is(err, errSignatureProofMissing):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentSignatureNotFound))
		return
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	rs.writeSignatureProof(w, r, proof)
}

// getStatusSignatureProof downloads the parent's proof behind the status
// link. The token is the only authentication: an unknown, expired or
// altered token answers like a missing request.
func (rs *Resource) getStatusSignatureProof(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil || rs.ListExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment signature proof not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("status token is required"), common.CodeEnrollmentStatusLinkInvalid))
		return
	}
	var proof *SignatureProof
	err := withinAdmin(r.Context(), func(adminCtx context.Context) error {
		var err error
		proof, err = rs.RequestService.SignatureProofByStatusToken(adminCtx, token)
		return err
	})
	if errors.Is(err, capability.ErrRequestNotFound) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentStatusLinkInvalid))
		return
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if proof.Signature == nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(errSignatureProofMissing, common.CodeEnrollmentSignatureNotFound))
		return
	}
	rs.writeSignatureProof(w, r, signatureProof{
		Request: proof.Request, Children: proof.Children, Guardians: proof.Guardians, Signature: proof.Signature,
	})
}

func (rs *Resource) writeSignatureProof(w http.ResponseWriter, r *http.Request, proof signatureProof) {
	file, err := rs.ListExportService.RenderRecords(signatureProofDocument(proof, time.Now()), "anmeldenachweis")
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("render signature proof: %w", err)))
		return
	}
	writeExportFile(w, file)
}

func proofText(text string) string { return lists.SanitizeUserText(text) }

func proofField(label, value string) lists.Field {
	return lists.Field{Label: label, Value: proofText(value)}
}

func proofStamp(t time.Time) string {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		location = time.FixedZone("CET", 60*60)
	}
	return t.In(location).Format("02.01.2006, 15:04") + " Uhr"
}

// signatureProofDocument describes the proof: the signature with its
// checksums, the guardians, the children and the consents of the signed
// version. The PDF is German like the school's own records.
func signatureProofDocument(p signatureProof, now time.Time) lists.RecordDocument {
	req := p.Request
	guardianName := strings.TrimSpace(req.GuardianFirstName + " " + req.GuardianLastName)
	doc := lists.RecordDocument{
		Title: "Anmeldenachweis", Subtitle: proofText(guardianName), GeneratedAt: now,
		Footer: "Vertraulich: enthält personenbezogene Daten",
	}
	if p.PhaseName != "" {
		doc.Filters = []string{proofText(p.PhaseName)}
	}
	doc.Groups = append(doc.Groups,
		lists.RecordGroup{Title: "Unterschrift", Records: []lists.Record{signatureProofRecord(p.Signature)}},
		lists.RecordGroup{Title: "Anmeldung", Records: signatureProofRequestRecords(p)},
	)
	if consents := signatureProofConsents(req); len(consents.Fields) > 0 {
		doc.Groups = append(doc.Groups, lists.RecordGroup{Title: "Zustimmungen", Records: []lists.Record{consents}})
	}
	return doc
}

func signatureProofRecord(s *capability.RequestSignature) lists.Record {
	integrity := "Unterschrift und gespeicherter Eintrag sind unverändert."
	if !s.IntegrityOK {
		integrity = "Achtung: Der gespeicherte Eintrag wurde nachträglich verändert."
	}
	method := "Mit Finger, Stift oder Maus gezeichnet"
	if s.Method == "typed" {
		method = "Name getippt: " + s.TypedName
	}
	return lists.Record{Title: "Einfache elektronische Signatur mit gezeichneter Unterschrift", Fields: []lists.Field{
		{Label: "Unterschrift", Value: proofText(method), PNG: s.PNG},
		proofField("Unterschrieben von", s.SignerName),
		proofField("Unterschrieben am", proofStamp(s.SignedAt)),
		proofField("Prüfsumme der unterschriebenen Anmeldung", s.DocumentSHA256),
		proofField("Prüfsumme des Unterschriftsbilds", s.ImageSHA256),
		proofField("Prüfung", integrity),
	}}
}

func signatureProofRequestRecords(p signatureProof) []lists.Record {
	req := p.Request
	guardian := []lists.Field{
		proofField("Name", strings.TrimSpace(req.GuardianFirstName+" "+req.GuardianLastName)),
		proofField("E-Mail", req.GuardianEmail),
	}
	if req.GuardianPhone != nil && strings.TrimSpace(*req.GuardianPhone) != "" {
		guardian = append(guardian, proofField("Telefon", *req.GuardianPhone))
	}
	guardian = append(guardian, proofField("Eingegangen am", proofStamp(req.SubmittedAt)))
	records := []lists.Record{{Title: "Erziehungsberechtigte Person", Fields: guardian}}
	for _, g := range p.Guardians {
		fields := []lists.Field{proofField("Name", strings.TrimSpace(g.FirstName+" "+g.LastName))}
		if g.Email != nil && *g.Email != "" {
			fields = append(fields, proofField("E-Mail", *g.Email))
		}
		if g.Phone != nil && *g.Phone != "" {
			fields = append(fields, proofField("Telefon", *g.Phone))
		}
		records = append(records, lists.Record{Title: "Weitere erziehungsberechtigte Person", Fields: fields})
	}
	for _, c := range p.Children {
		fields := []lists.Field{proofField("Geburtsdatum", c.DateOfBirth.Format("02.01.2006"))}
		if c.TargetGradeLevel != nil {
			fields = append(fields, proofField("Klassenstufe", strconv.Itoa(int(*c.TargetGradeLevel))))
		}
		if c.TargetSchoolClass != nil && *c.TargetSchoolClass != "" {
			fields = append(fields, proofField("Klasse", *c.TargetSchoolClass))
		}
		records = append(records, lists.Record{Title: proofText(strings.TrimSpace(c.FirstName + " " + c.LastName)), Fields: fields})
	}
	return records
}

// signatureProofConsents lists the legal blocks of the newest legal
// snapshot with the parent's answer.
func signatureProofConsents(req *Request) lists.Record {
	record := lists.Record{Title: "Rechtstexte zum Zeitpunkt der letzten Abgabe"}
	if len(req.LegalBlocksSnapshot) == 0 {
		return record
	}
	entry := req.LegalBlocksSnapshot[len(req.LegalBlocksSnapshot)-1]
	flags := map[string]any{}
	_ = decodeJSONObject(entry.ConsentFlags, &flags)
	for _, b := range entry.Blocks {
		if b.Kind == capability.LegalBlockKindNotice || b.Kind == capability.LegalBlockKindPrivacyNotice {
			record.Fields = append(record.Fields, proofField(b.Title, "Zur Kenntnis genommen"))
			continue
		}
		answer := "Nicht zugestimmt"
		if v, ok := flags[b.Key].(bool); ok && v {
			answer = "Zugestimmt"
		}
		record.Fields = append(record.Fields, proofField(b.Title, answer))
	}
	return record
}
