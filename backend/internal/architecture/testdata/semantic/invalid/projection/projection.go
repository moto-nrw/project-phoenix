package projection

import "github.com/uptrace/bun"

func ReadAndWrite(db *bun.DB) {
	db.NewSelect().TableExpr("beta.records")
	db.NewDelete().TableExpr("beta.records")
}
