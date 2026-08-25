package service

import "github.com/uptrace/bun"

func Read(db *bun.DB) {
	db.NewSelect().TableExpr("alpha.records")
}
