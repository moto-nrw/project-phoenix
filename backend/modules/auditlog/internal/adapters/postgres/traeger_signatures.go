package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/auditlog"
	"github.com/uptrace/bun"
)

// TraegerSignatures (#4679) persist audit.traeger_signatures: the signature
// ledger of #4785 at a Träger. The table is insert-only for the request
// role; deletion runs only through the contract version and the Träger.

const traegerSignatureRecordFormat = 1

type traegerSignatureRow struct {
	bun.BaseModel         `bun:"table:audit.traeger_signatures,alias:tsig"`
	ID                    int64     `bun:"id,pk,autoincrement"`
	OrganizationID        int64     `bun:"organization_id,notnull"`
	Purpose               string    `bun:"purpose,notnull"`
	CareContractVersionID int64     `bun:"care_contract_version_id,notnull"`
	SignerAccountID       *int64    `bun:"signer_account_id"`
	SignerName            string    `bun:"signer_name,notnull"`
	Method                string    `bun:"method,notnull"`
	TypedName             *string   `bun:"typed_name"`
	ImagePNG              []byte    `bun:"image_png,notnull"`
	ImageSHA256           string    `bun:"image_sha256,notnull"`
	DocumentSHA256        string    `bun:"document_sha256,notnull"`
	RecordHash            string    `bun:"record_hash,notnull"`
	SignedAt              time.Time `bun:"signed_at,notnull"`
}

// canonicalTraegerSignature is what the record hash covers. As at the
// school, the signer account is excluded: deleting the account clears it.
type canonicalTraegerSignature struct {
	Format            int    `json:"format"`
	OrganizationID    int64  `json:"organization_id"`
	Purpose           string `json:"purpose"`
	ContractVersionID int64  `json:"care_contract_version_id"`
	SignerName        string `json:"signer_name"`
	Method            string `json:"method"`
	TypedName         string `json:"typed_name"`
	ImageSHA256       string `json:"image_sha256"`
	DocumentSHA256    string `json:"document_sha256"`
	SignedAt          string `json:"signed_at"`
}

// TraegerSignatures is the signature ledger at a Träger. database answers
// the caller's Träger transaction with its organization.
type TraegerSignatures struct {
	database    Database
	fingerprint func([]byte) string
}

func NewTraegerSignatures(database Database, fingerprint func([]byte) string) *TraegerSignatures {
	return &TraegerSignatures{database: database, fingerprint: fingerprint}
}

var _ auditlog.TraegerSignatureLedger = (*TraegerSignatures)(nil)

func (s *TraegerSignatures) RecordTraegerSignature(ctx context.Context, entry auditlog.TraegerSignatureEntry) (*auditlog.TraegerSignature, error) {
	if err := validTraegerSignatureEntry(entry); err != nil {
		return nil, err
	}
	db, organizationID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := &traegerSignatureRow{
		OrganizationID: organizationID, Purpose: entry.Purpose, CareContractVersionID: entry.ContractVersionID,
		SignerAccountID: entry.SignerAccountID, SignerName: strings.TrimSpace(entry.SignerName),
		Method: entry.Image.Method, ImagePNG: entry.Image.PNG, ImageSHA256: s.fingerprint(entry.Image.PNG),
		DocumentSHA256: entry.DocumentSHA256, SignedAt: time.UnixMicro(entry.SignedAt.UnixMicro()).UTC(),
	}
	if entry.Image.Method == auditlog.SignatureMethodTyped {
		typed := entry.Image.TypedName
		row.TypedName = &typed
	}
	row.RecordHash = s.recordHash(row)
	if _, err := db.NewInsert().Model(row).Returning("id").Exec(ctx); err != nil {
		return nil, fmt.Errorf("audit database error during append audit.traeger_signatures: %w", err)
	}
	return s.value(row), nil
}

func (s *TraegerSignatures) TraegerSignatures(ctx context.Context, ids []int64) (map[int64]*auditlog.TraegerSignature, error) {
	out := make(map[int64]*auditlog.TraegerSignature, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	db, organizationID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []traegerSignatureRow
	if err := db.NewSelect().Model(&rows).
		Where(`"tsig".organization_id = ?`, organizationID).
		Where(`"tsig".id IN (?)`, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("audit database error during read audit.traeger_signatures: %w", err)
	}
	for i := range rows {
		out[rows[i].ID] = s.value(&rows[i])
	}
	return out, nil
}

func (s *TraegerSignatures) value(row *traegerSignatureRow) *auditlog.TraegerSignature {
	signature := &auditlog.TraegerSignature{
		ID: row.ID, OrganizationID: row.OrganizationID, Purpose: row.Purpose, ContractVersionID: row.CareContractVersionID,
		SignerAccountID: row.SignerAccountID, SignerName: row.SignerName, Method: row.Method, PNG: row.ImagePNG,
		ImageSHA256: row.ImageSHA256, DocumentSHA256: row.DocumentSHA256, RecordHash: row.RecordHash, SignedAt: row.SignedAt,
	}
	if row.TypedName != nil {
		signature.TypedName = *row.TypedName
	}
	signature.IntegrityOK = s.fingerprint(row.ImagePNG) == row.ImageSHA256 && s.recordHash(row) == row.RecordHash
	return signature
}

func (s *TraegerSignatures) recordHash(row *traegerSignatureRow) string {
	typed := ""
	if row.TypedName != nil {
		typed = *row.TypedName
	}
	encoded, err := json.Marshal(canonicalTraegerSignature{
		Format: traegerSignatureRecordFormat, OrganizationID: row.OrganizationID, Purpose: row.Purpose,
		ContractVersionID: row.CareContractVersionID, SignerName: row.SignerName, Method: row.Method, TypedName: typed,
		ImageSHA256: row.ImageSHA256, DocumentSHA256: row.DocumentSHA256, SignedAt: row.SignedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		// The canonical struct holds only strings and numbers.
		panic(fmt.Sprintf("audit: encode Träger signature canonical form: %v", err))
	}
	return s.fingerprint(encoded)
}

func validTraegerSignatureEntry(entry auditlog.TraegerSignatureEntry) error {
	if entry.Purpose != auditlog.SignaturePurposeCareContract {
		return fmt.Errorf("%w: unknown purpose %q", auditlog.ErrSignatureInvalid, entry.Purpose)
	}
	if entry.ContractVersionID <= 0 {
		return fmt.Errorf("%w: the contract version is required", auditlog.ErrSignatureInvalid)
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
	if _, err := auditlog.ParseSignatureImageBytes(entry.Image.Method, entry.Image.PNG, entry.Image.TypedName); err != nil {
		return err
	}
	return nil
}
