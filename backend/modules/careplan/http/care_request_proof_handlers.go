package careplanhttp

import (
	context "context"
	errors "errors"
	fmt "fmt"
	http "net/http"
	strconv "strconv"
	strings "strings"
	time "time"

	common "github.com/moto-nrw/project-phoenix/api/common"
	carerequests "github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// The OGS proof of a signed parent request (#4780): a pickup permission, a
// one-day collector or the way home alone, with the wording the guardian
// signed and the drawn signature, as a PDF in the moto design. The document
// is described here in the same unnamed shapes as the Erklärung report, so
// the root binds one Document Rendering record renderer for all of them.
type (
	// ReportField is one label/value line; PNG is printed below the label.
	ReportField = struct {
		Label, Value string
		PNG          []byte
	}
	// ReportBlock is an indented block inside a card.
	ReportBlock = struct {
		Title  string
		Fields []ReportField
	}
	// ReportCard is one card: a heading, its fields and its blocks.
	ReportCard = struct {
		Title  string
		Fields []ReportField
		Blocks []ReportBlock
	}
	// ReportSection is a titled group of cards.
	ReportSection = struct {
		Title string
		Cards []ReportCard
	}
	// ReportDocument is the whole document.
	ReportDocument = struct {
		Title       string
		Subtitle    string
		GeneratedAt time.Time
		Filters     []string
		Footer      string
		Sections    []ReportSection
	}
	// ReportFile is the rendered file.
	ReportFile = struct {
		Data        []byte
		ContentType string
		Filename    string
	}
)

// ReportRenderer renders a report document as a PDF.
type ReportRenderer interface {
	RenderReport(doc ReportDocument, filenameBase string) (ReportFile, error)
}

// PickupSignatureProof is a stored signature of a parent request. Intact is
// false when the image, the row or the wording it was given on no longer
// matches its checksum, or the signature belongs to another child or flow.
type PickupSignatureProof struct {
	Method     string
	TypedName  string
	SignerName string
	PNG        []byte
	SignedAt   time.Time
	Intact     bool
}

// PickupSignatureProofs reads the signature of a request in the caller's
// tenant transaction; nil when the school holds no such signature.
type PickupSignatureProofs interface {
	PickupSignatureProof(ctx context.Context, studentID, signatureID int64, statement string) (*PickupSignatureProof, error)
}

var errPickupProofUnbound = errors.New("pickup permission proof is not bound")

var careProofKindLabels = map[string]string{
	"pickup_change":                   "Einzelner Tag",
	carerequests.KindPickupPermission: "Abholberechtigung",
	"weekly_schedule":                 "Betreuungszeiten",
}

var careProofStatusLabels = map[string]string{
	carerequests.StatusPending:   "Offen",
	carerequests.StatusApproved:  "Bestätigt",
	carerequests.StatusRejected:  "Abgelehnt",
	carerequests.StatusWithdrawn: "Zurückgezogen",
	carerequests.StatusDone:      "Erledigt",
	carerequests.StatusCareEnded: "Betreuung beendet",
}

// getCareScheduleChangeRequestProof serves GET
// .../care-schedule-change-requests/{requestId}/proof: the PDF proof of a
// signed request for a reader the review policy allows for the child.
func (rs *Resource) getCareScheduleChangeRequestProof(w http.ResponseWriter, r *http.Request) {
	if rs.CareRequestReviews == nil || rs.PickupSignatureProofs == nil || rs.Reports == nil {
		renderError(w, r, common.ErrorInternalServer(errPickupProofUnbound))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	item, err := rs.CareRequestReviews.GetForReview(r.Context(), requestID)
	if err != nil {
		renderError(w, r, careRequestDetailErrorRenderer(err))
		return
	}
	signed := carerequests.StoredSignature(item.Request.Payload)
	if signed == nil {
		renderError(w, r, common.ErrorNotFoundWithCode(errors.New("request carries no signature"), common.CodeStudentsPickupProofNotFound))
		return
	}
	signature, err := rs.PickupSignatureProofs.PickupSignatureProof(r.Context(), item.Request.StudentID, signed.SignatureID, signed.Statement)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("read pickup permission signature", err))
		return
	}
	file, err := rs.Reports.RenderReport(careRequestProofReport(item, signed.Statement, signature, rs.Now()), "nachweis-elternanfrage")
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("render pickup permission proof", err))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

// careRequestProofReport describes the OGS proof of one signed request.
func careRequestProofReport(item *carerequests.HistoryItem, statement string, signature *PickupSignatureProof, now time.Time) ReportDocument {
	child := strings.TrimSpace(item.FirstName + " " + item.LastName)
	integrity := "Unterschrift und Text sind unverändert."
	if signature == nil || !signature.Intact {
		integrity = "Achtung: Die Unterschrift fehlt oder ein gespeicherter Eintrag wurde nachträglich verändert."
	}
	overview := ReportCard{Title: careProofLabel(careProofKindLabels, item.Request.RequestKind), Fields: []ReportField{
		{Label: "Kind", Value: child},
		{Label: "Erklärung der Eltern", Value: statement},
		{Label: "Eingegangen", Value: careProofStamp(item.Request.CreatedAt)},
		{Label: "Stand", Value: careProofLabel(careProofStatusLabels, item.Request.Status)},
		{Label: "Verfahren", Value: "Einfache elektronische Signatur mit gezeichneter Unterschrift"},
		{Label: "Prüfung", Value: integrity},
	}}
	for _, line := range item.Requested {
		value := line.New
		if line.Old != "" {
			value = line.Old + " → " + line.New
		}
		overview.Fields = append(overview.Fields, ReportField{Label: line.Label, Value: value})
	}
	cards := []ReportCard{overview}
	if signature != nil {
		typed := ""
		if signature.TypedName != "" {
			typed = "Name getippt: " + signature.TypedName
		}
		cards = append(cards, ReportCard{Title: "Unterschrift am " + careProofStamp(signature.SignedAt), Fields: []ReportField{
			{Label: "Von", Value: signature.SignerName},
			{Label: "Unterschrift", Value: typed, PNG: signature.PNG},
		}})
	}
	return ReportDocument{
		Title: "Nachweis einer Elternanfrage", Subtitle: child, GeneratedAt: now,
		Filters: []string{child}, Footer: "Vertraulich: enthält personenbezogene Daten",
		Sections: []ReportSection{{Title: "Überblick", Cards: cards}},
	}
}

func careProofLabel(labels map[string]string, key string) string {
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}

func careProofStamp(t time.Time) string {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		location = time.FixedZone("CET", 60*60)
	}
	return t.In(location).Format("02.01.2006, 15:04") + " Uhr"
}
