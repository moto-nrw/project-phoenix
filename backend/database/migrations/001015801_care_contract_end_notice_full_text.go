package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const careContractEndNoticeFullTextVersion = "1.15.801"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careContractEndNoticeFullTextVersion,
		Description: "Betreuungsvertrag: expandierte Mitteilungen vollständig speichern (#4682)",
		DependsOn:   []string{careContractNoticeEndDecisionVersion},
	})
	Migrations.MustRegister(careContractEndNoticeFullTextUp, careContractEndNoticeFullTextDown)
}

// Template input is bounded, but repeating placeholders can expand beyond
// 8,000 characters. Preserve the letter exactly as sent, including the reason
// and mandatory receipt/end lines; keep rejecting blank text.
func careContractEndNoticeFullTextUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE care_contract.contract_notices
  DROP CONSTRAINT chk_contract_notices_end_notice_text,
  ADD CONSTRAINT chk_contract_notices_end_notice_text CHECK (
   end_notice_text IS NULL OR BTRIM(end_notice_text) <> '');`); err != nil {
		return fmt.Errorf("allow full care contract end notices: %w", err)
	}
	return nil
}

// Rollback refuses letters the old bound cannot hold rather than truncating
// the parents' saved notice.
func careContractEndNoticeFullTextDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE care_contract.contract_notices
  DROP CONSTRAINT chk_contract_notices_end_notice_text,
  ADD CONSTRAINT chk_contract_notices_end_notice_text CHECK (
   end_notice_text IS NULL OR (BTRIM(end_notice_text) <> '' AND char_length(end_notice_text) <= 8000));`); err != nil {
		return fmt.Errorf("restore care contract end notice bound: %w", err)
	}
	return nil
}
