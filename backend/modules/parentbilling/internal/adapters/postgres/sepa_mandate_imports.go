package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The trail of imported SEPA mandates (#6198): the creation of each with
// the name of the file and the line it came from. The file itself is
// never stored. Every statement runs in the Träger transaction and names
// the Träger in its own filter.

// sepaMandateImportEventRow is the creation entry of an imported mandate.
type sepaMandateImportEventRow struct {
	bun.BaseModel `bun:"table:parent_billing.sepa_mandate_events"`

	ID             int64     `bun:"id,pk,autoincrement"`
	OrganizationID int64     `bun:"organization_id"`
	MandateID      int64     `bun:"mandate_id"`
	CareContractID int64     `bun:"care_contract_id"`
	Action         string    `bun:"action"`
	Source         string    `bun:"source"`
	Fields         []string  `bun:"fields,array"`
	ActorAccountID *int64    `bun:"actor_account_id"`
	ImportFile     string    `bun:"import_file"`
	ImportLine     int       `bun:"import_line"`
	OccurredAt     time.Time `bun:"occurred_at,nullzero,default:current_timestamp"`
}

// InsertSEPAMandateImportEvents records the creation of each imported
// mandate with the name of the file and its line in one statement.
func (s *Store) InsertSEPAMandateImportEvents(ctx context.Context, organizationID, accountID int64, file string,
	imported []parentbilling.ImportedSEPAMandate,
) error {
	if len(imported) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	rows := make([]sepaMandateImportEventRow, len(imported))
	for i, entry := range imported {
		rows[i] = sepaMandateImportEventRow{OrganizationID: organizationID, MandateID: entry.Mandate.ID,
			CareContractID: entry.Mandate.ContractID, Action: "create", Source: parentbilling.MandateFromImport,
			Fields: []string{}, ActorAccountID: accountRef(accountID), ImportFile: file, ImportLine: entry.Line}
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("record imported SEPA mandates: %w", err)
	}
	return nil
}

// SEPAMandateImportEvents reads the creation entries of the imported
// mandates of the Träger, newest first, one per mandate.
func (s *Store) SEPAMandateImportEvents(ctx context.Context, organizationID int64) ([]parentbilling.SEPAMandateImportEntry, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []sepaMandateImportEventRow
	if err := db.NewSelect().Model(&rows).Column("import_file", "actor_account_id", "occurred_at").
		Where("organization_id = ?", organizationID).Where("import_file IS NOT NULL").
		OrderExpr("occurred_at DESC, import_file, id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read SEPA mandate imports: %w", err)
	}
	entries := make([]parentbilling.SEPAMandateImportEntry, len(rows))
	for i, row := range rows {
		entries[i] = parentbilling.SEPAMandateImportEntry{File: row.ImportFile, AccountID: row.ActorAccountID,
			ImportedAt: row.OccurredAt, Mandates: 1}
	}
	return entries, nil
}
