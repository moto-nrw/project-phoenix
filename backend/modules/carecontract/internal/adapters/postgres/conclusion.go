package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// MarkConcluded moves a sent contract to aktiv and its version from
// beantragt to wirksam (#4680). Both updates are conditional: a contract
// that is no longer versendet, or a version that is no longer beantragt, is
// ErrContractNotConcludable, so a second conclusion changes nothing. The
// caller holds the row lock of the contract; a refused version update rolls
// the whole Träger transaction back with the caller's error.
func (Store) MarkConcluded(ctx context.Context, organizationID, contractID, versionID int64, conclusion carecontract.Concluded) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `UPDATE care_contract.contracts
		SET status = ?, concluded_at = ?, conclusion_method = ?
		WHERE organization_id = ? AND id = ? AND status = ? AND concluded_at IS NULL`,
		carecontract.StatusActive, conclusion.At, conclusion.Method, organizationID, contractID, carecontract.StatusSent)
	if err != nil {
		return fmt.Errorf("care contract postgres: conclude contract: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return carecontract.ErrContractNotConcludable
	}
	result, err = db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = ?
		WHERE organization_id = ? AND contract_id = ? AND id = ? AND status = ?`,
		carecontract.VersionEffective, organizationID, contractID, versionID, carecontract.VersionRequested)
	if err != nil {
		return fmt.Errorf("care contract postgres: make version effective: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return carecontract.ErrContractNotConcludable
	}
	return nil
}
