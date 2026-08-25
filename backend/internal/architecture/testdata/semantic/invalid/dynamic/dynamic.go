package dynamic

import "github.com/uptrace/bun"

func Read(db *bun.DB, table string) {
	db.NewSelect().TableExpr(table)
}
