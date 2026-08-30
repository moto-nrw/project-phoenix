package foreign

import "github.com/uptrace/bun"

func ReadAgain(db *bun.DB) {
	db.NewSelect().TableExpr("beta.records")
}
