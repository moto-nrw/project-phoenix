package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// PartiesOf reads the parties of the given contracts of the Träger in one
// statement, in the order they were added (#4684).
func (Store) PartiesOf(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64][]carecontract.Party, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ContractID int64  `bun:"contract_id"`
		FirstName  string `bun:"first_name"`
		LastName   string `bun:"last_name"`
		Email      string `bun:"email"`
		HasAccount bool   `bun:"has_account"`
	}
	if err := db.NewRaw(`SELECT contract_id, first_name, last_name, email, account_id IS NOT NULL AS has_account
		FROM care_contract.contract_parties WHERE organization_id = ? AND contract_id = ANY(?)
		ORDER BY contract_id, id`, organizationID, pgdialect.Array(contractIDs)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read contract parties: %w", err)
	}
	parties := make(map[int64][]carecontract.Party, len(contractIDs))
	for _, row := range rows {
		parties[row.ContractID] = append(parties[row.ContractID], carecontract.Party{
			FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, HasAccount: row.HasAccount,
		})
	}
	return parties, nil
}
