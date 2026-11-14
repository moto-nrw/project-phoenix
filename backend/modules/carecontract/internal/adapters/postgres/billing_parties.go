package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
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
		PartyID    *int64 `bun:"party_id"`
		FirstName  string `bun:"first_name"`
		LastName   string `bun:"last_name"`
		Email      string `bun:"email"`
		HasAccount bool   `bun:"has_account"`
	}
	if err := db.NewRaw(`SELECT c.id AS contract_id, p.id AS party_id, COALESCE(p.first_name, '') AS first_name,
			COALESCE(p.last_name, '') AS last_name, COALESCE(p.email, '') AS email, p.account_id IS NOT NULL AS has_account
		FROM care_contract.contracts AS c
		LEFT JOIN care_contract.contract_parties AS p ON p.organization_id = c.organization_id AND p.contract_id = c.id
		WHERE c.organization_id = ? AND c.id = ANY(?)
		ORDER BY c.id, p.id`, organizationID, pgdialect.Array(contractIDs)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read contract parties: %w", err)
	}
	parties := make(map[int64][]carecontract.Party, len(contractIDs))
	for _, row := range rows {
		if row.PartyID == nil {
			// A contract without a party (ADR 0232) still is a contract of
			// the Träger: it is named with no party.
			parties[row.ContractID] = []carecontract.Party{}
			continue
		}
		parties[row.ContractID] = append(parties[row.ContractID], carecontract.Party{
			FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, HasAccount: row.HasAccount,
		})
	}
	return parties, nil
}

// ContractFamilyEmails finds all matching parties of existing contracts in
// one statement; another Träger's contracts never establish membership.
func (Store) ContractFamilyEmails(ctx context.Context, organizationID int64, emails []string) ([]string, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	families := []string{}
	if err := db.NewRaw(`SELECT DISTINCT LOWER(BTRIM(party.email)) AS family_email
  FROM care_contract.contract_parties AS party
  JOIN care_contract.contracts AS contract ON contract.organization_id = party.organization_id AND contract.id = party.contract_id
  WHERE party.organization_id = ? AND LOWER(BTRIM(party.email)) = ANY(?::text[])
  ORDER BY family_email`, organizationID, pgdialect.Array(emails)).Scan(ctx, &families); err != nil {
		return nil, fmt.Errorf("care contract postgres: read family membership: %w", err)
	}
	return families, nil
}
