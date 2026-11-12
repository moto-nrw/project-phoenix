package postgres

import "github.com/moto-ogs/project-phoenix/modules/parentbilling"

// positionAccountsColumns are where an issued position books (#4821): the
// release writes them once; the trigger refuse_issued_change keeps them.
type positionAccountsColumns struct {
	RevenueAccount *string `bun:"revenue_account"`
	CostCenter     *string `bun:"cost_center"`
	CostCenter2    *string `bun:"cost_center_2"`
	AccountsFrozen bool    `bun:"accounts_frozen"`
}

// positionAccountsOf is the columns of what the release resolved; an empty
// value is NULL.
func positionAccountsOf(accounts parentbilling.PositionAccounts) positionAccountsColumns {
	if !accounts.Frozen {
		return positionAccountsColumns{}
	}
	return positionAccountsColumns{RevenueAccount: textRef(accounts.RevenueAccount), CostCenter: textRef(accounts.CostCenter),
		CostCenter2: textRef(accounts.CostCenter2), AccountsFrozen: true}
}

func (c positionAccountsColumns) accounts() parentbilling.PositionAccounts {
	return parentbilling.PositionAccounts{RevenueAccount: textOf(c.RevenueAccount), CostCenter: textOf(c.CostCenter),
		CostCenter2: textOf(c.CostCenter2), Frozen: c.AccountsFrozen}
}
