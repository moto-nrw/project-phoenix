package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/uptrace/bun"
)

// Signatures (#4785) persist audit.signatures. The table is insert-only for
// the request roles; deletion runs only through the school and child foreign
// keys with the table owner's rights.

const signatureRecordFormat = 1

type signatureRow struct {
	bun.BaseModel       `bun:"table:audit.signatures,alias:sig"`
	ID                  int64     `bun:"id,pk,autoincrement"`
	TenantID            int64     `bun:"tenant_id,notnull"`
	Purpose             string    `bun:"purpose,notnull"`
	StudentID           *int64    `bun:"student_id"`
	EnrollmentRequestID *int64    `bun:"enrollment_request_id"`
	SignerAccountID     *int64    `bun:"signer_account_id"`
	SignerName          string    `bun:"signer_name,notnull"`
	Method              string    `bun:"method,notnull"`
	TypedName           *string   `bun:"typed_name"`
	ImagePNG            []byte    `bun:"image_png,notnull"`
	ImageSHA256         string    `bun:"image_sha256,notnull"`
	DocumentSHA256      string    `bun:"document_sha256,notnull"`
	RecordHash          string    `bun:"record_hash,notnull"`
	SignedAt            time.Time `bun:"signed_at,notnull"`
}

// canonicalSignature is what the record hash covers. The signer account is
// excluded: deleting the account clears that nullable foreign key, while the
// signer's name stays frozen in the row.
type canonicalSignature struct {
	Format    int    `json:"format"`
	TenantID  int64  `json:"tenant_id"`
	Purpose   string `json:"purpose"`
	StudentID *int64 `json:"student_id"`
	// EnrollmentRequestID is left out when empty, so the record of a
	// signature about a child has one fixed form.
	EnrollmentRequestID *int64 `json:"enrollment_request_id,omitempty"`
	SignerName          string `json:"signer_name"`
	Method              string `json:"method"`
	TypedName           string `json:"typed_name"`
	ImageSHA256         string `json:"image_sha256"`
	DocumentSHA256      string `json:"document_sha256"`
	SignedAt            string `json:"signed_at"`
}

// Signatures is the signature ledger. fingerprint is the Security Runtime's
// SHA-256 content identity, bound by the root.
type Signatures struct {
	database    Database
	fingerprint func([]byte) string
}

func NewSignatures(database Database, fingerprint func([]byte) string) *Signatures {
	return &Signatures{database: database, fingerprint: fingerprint}
}

var _ auditlog.SignatureLedger = (*Signatures)(nil)

func (s *Signatures) RecordSignature(ctx context.Context, entry auditlog.SignatureEntry) (*auditlog.Signature, error) {
	if err := validSignatureEntry(entry); err != nil {
		return nil, err
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := &signatureRow{
		TenantID: tenantID, Purpose: entry.Purpose, StudentID: entry.StudentID,
		EnrollmentRequestID: entry.EnrollmentRequestID, SignerAccountID: entry.SignerAccountID,
		SignerName: strings.TrimSpace(entry.SignerName), Method: entry.Image.Method, ImagePNG: entry.Image.PNG,
		ImageSHA256: s.fingerprint(entry.Image.PNG), DocumentSHA256: entry.DocumentSHA256,
		SignedAt: time.UnixMicro(entry.SignedAt.UnixMicro()).UTC(),
	}
	if entry.Image.Method == auditlog.SignatureMethodTyped {
		typed := entry.Image.TypedName
		row.TypedName = &typed
	}
	row.RecordHash = s.recordHash(row)
	if _, err := db.NewInsert().Model(row).Returning("id").Exec(ctx); err != nil {
		return nil, fmt.Errorf("audit database error during append audit.signatures: %w", err)
	}
	return s.value(row), nil
}

func (s *Signatures) Signatures(ctx context.Context, ids []int64) (map[int64]*auditlog.Signature, error) {
	out := make(map[int64]*auditlog.Signature, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []signatureRow
	if err := db.NewSelect().Model(&rows).
		Where(`"sig".tenant_id = ?`, tenantID).
		Where(`"sig".id IN (?)`, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("audit database error during read audit.signatures: %w", err)
	}
	for i := range rows {
		out[rows[i].ID] = s.value(&rows[i])
	}
	return out, nil
}

func (s *Signatures) LatestEnrollmentSignatures(ctx context.Context, requestIDs []int64) (map[int64]*auditlog.Signature, error) {
	out := make(map[int64]*auditlog.Signature, len(requestIDs))
	if len(requestIDs) == 0 {
		return out, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []signatureRow
	if err := db.NewSelect().Model(&rows).
		DistinctOn(`"sig".enrollment_request_id`).
		Where(`"sig".tenant_id = ?`, tenantID).
		Where(`"sig".purpose = ?`, auditlog.SignaturePurposeEnrollment).
		Where(`"sig".enrollment_request_id IN (?)`, bun.List(requestIDs)).
		OrderExpr(`"sig".enrollment_request_id, "sig".signed_at DESC, "sig".id DESC`).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("audit database error during read audit.signatures: %w", err)
	}
	for i := range rows {
		out[*rows[i].EnrollmentRequestID] = s.value(&rows[i])
	}
	return out, nil
}

func (s *Signatures) value(row *signatureRow) *auditlog.Signature {
	signature := &auditlog.Signature{
		ID: row.ID, TenantID: row.TenantID, Purpose: row.Purpose, StudentID: row.StudentID,
		EnrollmentRequestID: row.EnrollmentRequestID, SignerAccountID: row.SignerAccountID, SignerName: row.SignerName, Method: row.Method,
		PNG: row.ImagePNG, ImageSHA256: row.ImageSHA256, DocumentSHA256: row.DocumentSHA256,
		RecordHash: row.RecordHash, SignedAt: row.SignedAt,
	}
	if row.TypedName != nil {
		signature.TypedName = *row.TypedName
	}
	signature.IntegrityOK = s.fingerprint(row.ImagePNG) == row.ImageSHA256 && s.recordHash(row) == row.RecordHash
	return signature
}

func (s *Signatures) recordHash(row *signatureRow) string {
	typed := ""
	if row.TypedName != nil {
		typed = *row.TypedName
	}
	encoded, err := json.Marshal(canonicalSignature{
		Format: signatureRecordFormat, TenantID: row.TenantID, Purpose: row.Purpose, StudentID: row.StudentID,
		EnrollmentRequestID: row.EnrollmentRequestID,
		SignerName:          row.SignerName, Method: row.Method, TypedName: typed, ImageSHA256: row.ImageSHA256,
		DocumentSHA256: row.DocumentSHA256, SignedAt: row.SignedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		// The canonical struct holds only strings, numbers and a nullable number.
		panic(fmt.Sprintf("audit: encode signature canonical form: %v", err))
	}
	return s.fingerprint(encoded)
}

func validSignatureEntry(entry auditlog.SignatureEntry) error {
	switch entry.Purpose {
	case auditlog.SignaturePurposeDeclaration, auditlog.SignaturePurposeEnrollment, auditlog.SignaturePurposePickupPermission,
		auditlog.SignaturePurposeAccidentReport:
	default:
		return fmt.Errorf("%w: unknown purpose %q", auditlog.ErrSignatureInvalid, entry.Purpose)
	}
	if (entry.StudentID == nil) == (entry.EnrollmentRequestID == nil) {
		return fmt.Errorf("%w: exactly one of student and enrollment request is required", auditlog.ErrSignatureInvalid)
	}
	if strings.TrimSpace(entry.SignerName) == "" {
		return fmt.Errorf("%w: signer name is required", auditlog.ErrSignatureInvalid)
	}
	if !isSHA256Hex(entry.DocumentSHA256) {
		return fmt.Errorf("%w: document checksum must be a SHA-256 hex digest", auditlog.ErrSignatureInvalid)
	}
	if entry.SignedAt.IsZero() {
		return errors.New("signature: signed_at is required")
	}
	// Re-check the image: an entry may be built without ParseSignatureImage.
	if _, err := auditlog.ParseSignatureImageBytes(entry.Image.Method, entry.Image.PNG, entry.Image.TypedName); err != nil {
		return err
	}
	return nil
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
