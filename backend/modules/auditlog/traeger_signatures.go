package auditlog

import (
	"context"
	"time"
)

// Signatures at the Träger (#4679): the same building block as Signatures
// (#4785), for a record that lives at a Träger rather than at a school. A
// care contract belongs to the Träger (ADR 0088, ADR 0105), so its signature
// is stored in audit.traeger_signatures under the Träger policy, inside the
// caller's Träger transaction. The checks of the image and the record hash
// are the same; the ledger still never decides authorization.

// SignaturePurposeCareContract is the parents' signature of a care contract
// version (#4679).
const SignaturePurposeCareContract = "care_contract"

// TraegerSignatureEntry is one signature to store at a Träger.
// ContractVersionID is the signed care contract version; deleting it deletes
// the signature. DocumentSHA256 is the checksum of the PDF that was signed.
// SignerAccountID is nil when the signer came through a one-time mail link.
type TraegerSignatureEntry struct {
	Purpose           string
	ContractVersionID int64
	SignerAccountID   *int64
	SignerName        string
	DocumentSHA256    string
	SignedAt          time.Time
	Image             SignatureImage
}

// TraegerSignature is one stored signature of a Träger. IntegrityOK is false
// when the image or the row no longer matches its stored checksum.
type TraegerSignature struct {
	ID                int64
	OrganizationID    int64
	Purpose           string
	ContractVersionID int64
	SignerAccountID   *int64
	SignerName        string
	Method            string
	TypedName         string
	PNG               []byte
	ImageSHA256       string
	DocumentSHA256    string
	RecordHash        string
	SignedAt          time.Time
	IntegrityOK       bool
}

// TraegerSignatureLedger appends and reads signatures inside the caller's
// Träger transaction, so a signature commits or rolls back with the record
// that references it. Reads return only rows of that Träger.
type TraegerSignatureLedger interface {
	RecordTraegerSignature(context.Context, TraegerSignatureEntry) (*TraegerSignature, error)
	TraegerSignatures(ctx context.Context, ids []int64) (map[int64]*TraegerSignature, error)
}
