package application

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The PAIN.008 file (#4697, ADR 0181): a Customer Direct Debit Initiation
// in version pain.008.001.08 as the Deutsche Kreditwirtschaft prescribes it
// (Anlage 3 of the DFÜ-Abkommen, version 3.9 and 26.11, technical
// validation subset GBIC_5), SEPA Core. One batch per sequence type and
// collection day. Identifiers keep to the SEPA characters; names and texts
// to the character set of the Deutsche Kreditwirtschaft, with umlauts.

// pain008Namespace is the namespace of pain.008.001.08.
const pain008Namespace = "urn:iso:std:iso:20022:tech:xsd:pain.008.001.08"

// Limits of the message.
const (
	maxSEPAIdentifier = 35
	maxSEPAName       = 70
	maxSEPARemittance = 140
	// sepaNotProvided names an agent without a BIC (IBAN-only within the EEA).
	sepaNotProvided = "NOTPROVIDED"
)

// debitFileEntry is one transaction of the file with its open account.
type debitFileEntry struct {
	EndToEndID       string
	AmountCents      int64
	MandateReference string
	MandateSignedOn  calendar.Date
	DebtorName       string
	DebtorIBAN       string
	DebtorBIC        string
	Remittance       string
}

// debitFileBatch is one payment information block: one sequence type on
// one collection day.
type debitFileBatch struct {
	ID           string
	SequenceType string
	CollectOn    calendar.Date
	Entries      []debitFileEntry
}

// debitFileHead is what the whole file names: the message, the Träger as
// initiating party and creditor, its account and creditor identifier.
type debitFileHead struct {
	MessageID    string
	CreatedAt    time.Time
	CreditorName string
	CreditorIBAN string
	CreditorBIC  string
	CreditorID   string
}

type painDocument struct {
	XMLName xml.Name `xml:"Document"`
	XMLNS   string   `xml:"xmlns,attr"`
	XSI     string   `xml:"xmlns:xsi,attr"`
	Body    painBody `xml:"CstmrDrctDbtInitn"`
}

type painBody struct {
	GroupHeader painGroupHeader   `xml:"GrpHdr"`
	Payments    []painPaymentInfo `xml:"PmtInf"`
}

type painParty struct {
	Name string `xml:"Nm"`
}

type painGroupHeader struct {
	MessageID    string    `xml:"MsgId"`
	CreatedAt    string    `xml:"CreDtTm"`
	Transactions int       `xml:"NbOfTxs"`
	ControlSum   string    `xml:"CtrlSum"`
	Initiator    painParty `xml:"InitgPty"`
}

type painAccount struct {
	IBAN string `xml:"Id>IBAN"`
}

type painAgentOther struct {
	ID string `xml:"Id"`
}

type painAgent struct {
	BIC   string          `xml:"FinInstnId>BICFI,omitempty"`
	Other *painAgentOther `xml:"FinInstnId>Othr,omitempty"`
}

func agentOf(bic string) painAgent {
	if bic == "" {
		return painAgent{Other: &painAgentOther{ID: sepaNotProvided}}
	}
	return painAgent{BIC: bic}
}

type painSchemeID struct {
	ID     string `xml:"Id>PrvtId>Othr>Id"`
	Scheme string `xml:"Id>PrvtId>Othr>SchmeNm>Prtry"`
}

type painPaymentType struct {
	ServiceLevel string `xml:"SvcLvl>Cd"`
	Instrument   string `xml:"LclInstrm>Cd"`
	SequenceType string `xml:"SeqTp"`
}

type painPaymentInfo struct {
	ID              string            `xml:"PmtInfId"`
	Method          string            `xml:"PmtMtd"`
	Count           int               `xml:"NbOfTxs"`
	ControlSum      string            `xml:"CtrlSum"`
	PaymentType     painPaymentType   `xml:"PmtTpInf"`
	CollectOn       string            `xml:"ReqdColltnDt"`
	Creditor        painParty         `xml:"Cdtr"`
	CreditorAccount painAccount       `xml:"CdtrAcct"`
	CreditorAgent   painAgent         `xml:"CdtrAgt"`
	ChargeBearer    string            `xml:"ChrgBr"`
	CreditorScheme  painSchemeID      `xml:"CdtrSchmeId"`
	Debits          []painTransaction `xml:"DrctDbtTxInf"`
}

type painAmount struct {
	Currency string `xml:"Ccy,attr"`
	Value    string `xml:",chardata"`
}

type painMandate struct {
	ID       string `xml:"MndtId"`
	SignedOn string `xml:"DtOfSgntr"`
}

type painTransaction struct {
	EndToEndID    string      `xml:"PmtId>EndToEndId"`
	Amount        painAmount  `xml:"InstdAmt"`
	Mandate       painMandate `xml:"DrctDbtTx>MndtRltdInf"`
	DebtorAgent   painAgent   `xml:"DbtrAgt"`
	Debtor        painParty   `xml:"Dbtr"`
	DebtorAccount painAccount `xml:"DbtrAcct"`
	Remittance    string      `xml:"RmtInf>Ustrd"`
}

// euros writes cents as the decimal amount of the message.
func euros(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

// renderPain008 writes the file.
func renderPain008(head debitFileHead, batches []debitFileBatch) ([]byte, error) {
	if err := validateSEPANames(head, batches); err != nil {
		return nil, err
	}
	document := painDocument{XMLNS: pain008Namespace, XSI: "http://www.w3.org/2001/XMLSchema-instance"}
	body := &document.Body
	var count int
	var total int64
	for _, batch := range batches {
		info := painPaymentInfo{
			ID: batch.ID, Method: "DD",
			PaymentType: painPaymentType{ServiceLevel: "SEPA", Instrument: "CORE", SequenceType: batch.SequenceType},
			CollectOn:   batch.CollectOn.String(), Creditor: painParty{Name: sepaText(head.CreditorName, maxSEPAName)},
			CreditorAccount: painAccount{IBAN: head.CreditorIBAN}, CreditorAgent: agentOf(head.CreditorBIC),
			ChargeBearer: "SLEV", CreditorScheme: painSchemeID{ID: head.CreditorID, Scheme: "SEPA"},
		}
		var sum int64
		for _, entry := range batch.Entries {
			info.Debits = append(info.Debits, painTransaction{
				EndToEndID: entry.EndToEndID, Amount: painAmount{Currency: "EUR", Value: euros(entry.AmountCents)},
				Mandate:     painMandate{ID: entry.MandateReference, SignedOn: entry.MandateSignedOn.String()},
				DebtorAgent: agentOf(entry.DebtorBIC), Debtor: painParty{Name: sepaText(entry.DebtorName, maxSEPAName)},
				DebtorAccount: painAccount{IBAN: entry.DebtorIBAN},
				Remittance:    sepaText(entry.Remittance, maxSEPARemittance),
			})
			sum += entry.AmountCents
		}
		info.Count, info.ControlSum = len(batch.Entries), euros(sum)
		count += len(batch.Entries)
		total += sum
		body.Payments = append(body.Payments, info)
	}
	body.GroupHeader = painGroupHeader{MessageID: head.MessageID, CreatedAt: head.CreatedAt.Format("2006-01-02T15:04:05"),
		Transactions: count, ControlSum: euros(total), Initiator: painParty{Name: sepaText(head.CreditorName, maxSEPAName)}}
	var out bytes.Buffer
	out.WriteString(xml.Header)
	encoder := xml.NewEncoder(&out)
	encoder.Indent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("parent billing: write the PAIN.008 file: %w", err)
	}
	out.WriteString("\n")
	return out.Bytes(), nil
}

// sepaIdentifier keeps an identifier to the SEPA characters of the
// Deutsche Kreditwirtschaft (letters, digits, + ? / - : ( ) . , ') and at
// most 35 of them; a character outside becomes a hyphen. The file never
// leads with a slash or carries a double one.
func sepaIdentifier(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)), strings.ContainsRune("+?-:().,'", r):
			b.WriteRune(r)
		case r == '/':
			if s := b.String(); s != "" && !strings.HasSuffix(s, "/") {
				b.WriteRune(r)
			}
		default:
			b.WriteRune('-')
		}
	}
	id := strings.Trim(b.String(), " ")
	if len(id) > maxSEPAIdentifier {
		id = id[:maxSEPAIdentifier]
	}
	if id == "" {
		return sepaNotProvided
	}
	return id
}

// dkTextCharacters are the characters besides letters and digits the
// Deutsche Kreditwirtschaft admits in names and texts.
const dkTextCharacters = " '&*$%:?,-(+.)/"

// sepaText keeps a name or text to the character set of the Deutsche
// Kreditwirtschaft: Latin letters with the German umlauts and ß, digits and
// a few signs. Other accents fall away (é becomes e), anything else becomes
// a space; the text is at most max characters.
func sepaText(value string, max int) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case strings.ContainsRune("ÄÖÜäöüß", r):
			b.WriteRune(r)
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(dkTextCharacters, r)):
			b.WriteRune(r)
		default:
			base := []rune(norm.NFD.String(string(r)))
			if len(base) > 0 && base[0] < unicode.MaxASCII && unicode.IsLetter(base[0]) {
				b.WriteRune(base[0])
			} else {
				b.WriteRune(' ')
			}
		}
	}
	text := strings.Join(strings.Fields(b.String()), " ")
	if runes := []rune(text); len(runes) > max {
		text = strings.TrimSpace(string(runes[:max]))
	}
	return text
}

// Never emit an empty mandatory Nm after converting to the DK character
// set. Report only the object ID, never the decrypted name or bank data.
func validateSEPANames(head debitFileHead, batches []debitFileBatch) error {
	if sepaText(head.CreditorName, maxSEPAName) == "" {
		return parentbilling.ErrSEPAAccountHolderUnsupported
	}
	for _, batch := range batches {
		for _, entry := range batch.Entries {
			if sepaText(entry.DebtorName, maxSEPAName) == "" {
				return parentbilling.ErrSEPAAccountHolderUnsupported
			}
		}
	}
	return nil
}
