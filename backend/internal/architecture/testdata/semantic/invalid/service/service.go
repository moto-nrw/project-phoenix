package service

import "github.com/uptrace/bun"

func Read(db *bun.DB) {
	db.NewSelect().TableExpr("alpha.records")
}

type WrappedDB struct{ *bun.DB }

func ReadWrapped(db *WrappedDB) {
	db.NewSelect()
}
