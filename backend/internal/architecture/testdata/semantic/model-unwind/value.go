package chain

import "github.com/uptrace/bun"

func Nested(db *bun.DB, model any) {
	keep(db.NewInsert().Model(model), db.NewInsert().Model(model))
	db.NewInsert().Model(model)
}
func keep(...*bun.InsertQuery) {}
