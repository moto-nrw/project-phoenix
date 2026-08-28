package unknown

import "github.com/uptrace/bun"

func Read(db *bun.DB) {
	db.NewSelect().TableExpr("ghost.records")
}
