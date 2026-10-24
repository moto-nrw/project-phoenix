package enrollmenthttp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"

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

// attachAdminSignature adds the request's signature to the staff detail and
// returns it for the proof.
func (rs *Resource) attachAdminSignature(ctx context.Context, detail *AdminRequestDetail, requestID int64) (*SignatureProof, error) {
	if rs.RequestService == nil {
		return nil, nil
	}
	proof, err := rs.RequestService.SignatureProof(ctx, requestID, "")
	if err != nil {
		return nil, err
	}
	detail.Signature = newRequestSignatureResponse(proof.Signature, true)
	return proof, nil
}

// loadAdminSignature enriches the JSON detail without hiding the enrollment
// when the optional signature read is unavailable. The PDF needs the complete
// proof, so it deliberately retains that read's failure.
func (rs *Resource) loadAdminSignature(ctx context.Context, detail *AdminRequestDetail, requestID int64, proofRequested bool) (*SignatureProof, error) {
	if proofRequested {
		return rs.attachAdminSignature(ctx, detail, requestID)
	}
	if err := tenant.WithSavepoint(ctx, func(savepointCtx context.Context) error {
		_, err := rs.attachAdminSignature(savepointCtx, detail, requestID)
		return err
	}); err != nil {
		rs.logger().Warn("enrollment detail: load signature failed",
			slog.String("error", err.Error()))
		if errors.Is(err, tenant.ErrSavepointControl) {
			return nil, err
		}
	}
	return nil, nil
}

// signatureProof is what a proof PDF prints.
type signatureProof struct {
	Request            *Request
	Children           []*RequestChild
	Guardians          []*capability.RequestGuardian
	Schema             *capability.FormSchema
	OfferingSelections []*capability.RequestChildOfferingRecord
	OfferingNames      map[int64]string
	PhaseName          string
	Signature          *capability.RequestSignature
}

var errSignatureProofMissing = errors.New("enrollment request has no signature")

// wantsProofPDF reports whether a detail read asks for the signature proof
// (?format=pdf) instead of the JSON detail.
func wantsProofPDF(r *http.Request) bool {
	return r.URL.Query().Get("format") == "pdf"
}

// writeAdminSignatureProof answers the staff proof of a signed request.
func (rs *Resource) writeAdminSignatureProof(w http.ResponseWriter, r *http.Request, summary *RequestSummary, signature *SignatureProof) {
	proof := signatureProof{
		Request: summary.Request, Children: summary.Children, Guardians: summary.Guardians,
	}
	if signature != nil {
		proof.Schema = signature.Schema
		proof.OfferingSelections = signature.OfferingSelections
		proof.OfferingNames = signature.OfferingNames
		proof.Signature = signature.Signature
	}
	if summary.Phase != nil {
		proof.PhaseName = summary.Phase.Name
	}
	rs.writeSignatureProof(w, r, proof)
}

// writeStatusSignatureProof answers the parent's proof behind the status
// link; the token already resolved the request.
func (rs *Resource) writeStatusSignatureProof(w http.ResponseWriter, r *http.Request, view statusView) {
	rs.writeSignatureProof(w, r, signatureProof{
		Request: view.req, Children: view.children, Guardians: view.proof.Guardians,
		Schema: view.proof.Schema, OfferingSelections: view.proof.OfferingSelections, OfferingNames: view.proof.OfferingNames, Signature: view.proof.Signature,
	})
}

func (rs *Resource) writeSignatureProof(w http.ResponseWriter, r *http.Request, proof signatureProof) {
	if proof.Signature == nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(errSignatureProofMissing, common.CodeEnrollmentSignatureNotFound))
		return
	}
	if rs.ListExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment signature proof not configured")))
		return
	}
	doc, err := signatureProofDocument(proof, time.Now())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("signature proof: %w", err)))
		return
	}
	file, err := rs.ListExportService.RenderRecords(doc, "anmeldenachweis")
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("render signature proof: %w", err)))
		return
	}
	writeExportFile(w, file)
}

func proofText(text string) string { return lists.SanitizeUserText(text) }

func proofField(label, value string) lists.Field {
	return lists.Field{Label: proofText(label), Value: proofText(value)}
}

func proofStamp(t time.Time) string {
	return t.In(calendar.Berlin).Format("02.01.2006, 15:04") + " Uhr"
}

// signatureProofDocument describes the proof: the signature with its
// checksums, the guardians, the children and the consents of the signed
// version. The PDF is German like the school's own records.
func signatureProofDocument(p signatureProof, now time.Time) (lists.RecordDocument, error) {
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
	consents, err := signatureProofConsents(req)
	if err != nil {
		return doc, err
	}
	if len(consents.Subs) > 0 {
		doc.Groups = append(doc.Groups, lists.RecordGroup{Title: "Zustimmungen", Records: []lists.Record{consents}})
	}
	return doc, nil
}

func signatureProofRecord(s *capability.RequestSignature) lists.Record {
	integrity := "Das Unterschriftsbild und der gespeicherte Eintrag der Unterschrift sind unverändert."
	if !s.IntegrityOK {
		integrity = "Achtung: Der gespeicherte Eintrag der Unterschrift wurde nachträglich verändert."
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
		proofField("Hinweis", "Die Prüfsumme belegt die Fassung, die unterschrieben wurde. Die Angaben unten zeigen den aktuellen Stand der Anmeldung. Hat die Schule sie später geändert, weichen sie von der unterschriebenen Fassung ab."),
	}}
}

func signatureProofRequestRecords(p signatureProof) []lists.Record {
	records := []lists.Record{signatureProofPrimaryGuardianRecord(p.Request, p.Schema)}
	records = append(records, signatureProofAdditionalGuardianRecords(p.Guardians)...)
	choicesByChild := signatureProofChoicesByChild(p.OfferingSelections, len(p.Children))
	return append(records, signatureProofChildRecords(p.Children, p.Schema, choicesByChild, p.OfferingNames)...)
}

func signatureProofPrimaryGuardianRecord(req *Request, schema *capability.FormSchema) lists.Record {
	guardian := []lists.Field{
		proofField("Name", strings.TrimSpace(req.GuardianFirstName+" "+req.GuardianLastName)),
		proofField("E-Mail", req.GuardianEmail),
	}
	if req.GuardianPhone != nil && strings.TrimSpace(*req.GuardianPhone) != "" {
		guardian = append(guardian, proofField("Telefon", *req.GuardianPhone))
	}
	guardian = append(guardian, proofField("Eingegangen am", proofStamp(req.SubmittedAt)))
	guardian = append(guardian, signatureProofCustomFields(req.CustomData, schema, false)...)
	return lists.Record{Title: "Erziehungsberechtigte Person", Fields: guardian}
}

func signatureProofAdditionalGuardianRecords(guardians []*capability.RequestGuardian) []lists.Record {
	records := make([]lists.Record, 0, len(guardians))
	for _, g := range guardians {
		fields := []lists.Field{proofField("Name", strings.TrimSpace(g.FirstName+" "+g.LastName))}
		if g.Email != nil && *g.Email != "" {
			fields = append(fields, proofField("E-Mail", *g.Email))
		}
		if g.Phone != nil && *g.Phone != "" {
			fields = append(fields, proofField("Telefon", *g.Phone))
		}
		records = append(records, lists.Record{Title: "Weitere erziehungsberechtigte Person", Fields: fields})
	}
	return records
}

func signatureProofChoicesByChild(selections []*capability.RequestChildOfferingRecord, childCount int) map[int64][]*capability.RequestChildOfferingRecord {
	choicesByChild := make(map[int64][]*capability.RequestChildOfferingRecord, childCount)
	for _, choice := range selections {
		if choice == nil {
			continue
		}
		choicesByChild[choice.RequestChildID] = append(choicesByChild[choice.RequestChildID], choice)
	}
	return choicesByChild
}

func signatureProofChildRecords(children []*RequestChild, schema *capability.FormSchema, choicesByChild map[int64][]*capability.RequestChildOfferingRecord, offeringNames map[int64]string) []lists.Record {
	records := make([]lists.Record, 0, len(children))
	for _, c := range children {
		fields := []lists.Field{proofField("Geburtsdatum", c.DateOfBirth.Format("02.01.2006"))}
		if c.TargetGradeLevel != nil {
			fields = append(fields, proofField("Klassenstufe", strconv.Itoa(int(*c.TargetGradeLevel))))
		}
		if c.TargetSchoolClass != nil && *c.TargetSchoolClass != "" {
			fields = append(fields, proofField("Klasse", *c.TargetSchoolClass))
		}
		fields = append(fields,
			proofField("Betreuungsangebote", signatureProofOfferings(choicesByChild[c.ID], offeringNames)),
		)
		fields = append(fields, signatureProofCustomFields(c.CustomData, schema, true)...)
		records = append(records, lists.Record{Title: proofText(strings.TrimSpace(c.FirstName + " " + c.LastName)), Fields: fields})
	}
	return records
}

func signatureProofOfferings(choices []*capability.RequestChildOfferingRecord, names map[int64]string) string {
	if len(choices) == 0 {
		return "Keine"
	}
	parts := make([]string, 0, len(choices))
	for _, choice := range choices {
		if choice == nil {
			continue
		}
		value := strings.TrimSpace(names[choice.CareOfferingID])
		if value == "" {
			value = "Angebot #" + strconv.FormatInt(choice.CareOfferingID, 10)
		} else {
			value += " (#" + strconv.FormatInt(choice.CareOfferingID, 10) + ")"
		}
		if days := formatDayCodes(choice.SelectedDays); days != "" {
			value += " (" + days + ")"
		}
		parts = append(parts, value)
	}
	if len(parts) == 0 {
		return "Keine"
	}
	return strings.Join(parts, "; ")
}

func signatureProofCustomFields(values map[string]any, schema *capability.FormSchema, appliesToChild bool) []lists.Field {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	fields := make([]lists.Field, 0, len(values))
	if schema != nil {
		for _, field := range schema.Fields {
			if field.AppliesToCh != appliesToChild {
				continue
			}
			if value, ok := values[field.Key]; ok {
				fields = append(fields, proofField(field.Label, formatCustomValue(field, value)))
				seen[field.Key] = true
			}
		}
	}
	keys := make([]string, 0, len(values)-len(seen))
	for key := range values {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		fields = append(fields, proofField("Weitere Angabe: "+key, stringifyValue(values[key])))
	}
	return fields
}

// signatureProofConsents reproduces each legal block of the newest legal
// snapshot with the parent's answer, text and document references.
func signatureProofConsents(req *Request) (lists.Record, error) {
	record := lists.Record{Title: "Rechtstexte zum Zeitpunkt der letzten Abgabe"}
	if len(req.LegalBlocksSnapshot) == 0 {
		return record, nil
	}
	entry := req.LegalBlocksSnapshot[len(req.LegalBlocksSnapshot)-1]
	flags := map[string]any{}
	if err := decodeJSONObject(entry.ConsentFlags, &flags); err != nil {
		return record, fmt.Errorf("decode consent flags of the legal snapshot: %w", err)
	}
	for _, b := range entry.Blocks {
		record.Subs = append(record.Subs, signatureProofLegalBlock(b, flags))
	}
	return record, nil
}

func signatureProofLegalBlock(block capability.LegalBlockSnapshot, flags map[string]any) lists.SubRecord {
	answer := "Nicht zugestimmt"
	if block.Kind == capability.LegalBlockKindNotice || block.Kind == capability.LegalBlockKindPrivacyNotice {
		answer = "Zur Kenntnis genommen"
	} else if accepted, ok := flags[block.Key].(bool); ok && accepted {
		answer = "Zugestimmt"
	}
	fields := []lists.Field{
		proofField("Bestätigung", block.Label),
		proofField("Antwort", answer),
		proofField("Text", block.Text),
	}
	for _, attachment := range block.Attachments {
		label := "Dokument"
		if name := strings.TrimSpace(attachment.Label); name != "" {
			label += ": " + name
		}
		fields = append(fields, proofField(label, attachment.DocumentURL))
	}
	return lists.SubRecord{Title: proofText(block.Title), Fields: fields}
}
