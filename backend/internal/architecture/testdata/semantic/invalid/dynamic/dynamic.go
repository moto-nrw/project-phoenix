package dynamic

import "github.com/uptrace/bun"

func Read(db *bun.DB, table string) {
	db.NewSelect().TableExpr(table)
}

func Fragments(db *bun.DB, fragment string) {
	db.NewSelect().TableExpr("alpha.records").Where(fragment)
	db.NewSelect().TableExpr("alpha.records").ColumnExpr(fragment)
	db.NewSelect().TableExpr("alpha.records").OrderExpr(fragment)
}
