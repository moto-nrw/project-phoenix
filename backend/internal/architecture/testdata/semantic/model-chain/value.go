package chain

import "github.com/uptrace/bun"

type foreignRecord struct {
	bun.BaseModel "bun:\"table:beta.records\""
}

func TableBefore(db *bun.DB, model any) {
	db.NewInsert().TableExpr("alpha.records").Model(model)
}

func TableAfter(db *bun.DB, model any) {
	db.NewInsert().Model(model).TableExpr("alpha.records")
}
func TableAfterIntermediateCall(db *bun.DB, model any) {
	db.NewInsert().Model(model).On("CONFLICT DO NOTHING").TableExpr("alpha.records")
}
func NestedFunction(db *bun.DB, model any) {
	func() { db.NewInsert().Model(model).TableExpr("alpha.records") }()
}
func NoTable(db *bun.DB, model any) {
	db.NewInsert().Model(model)
}
func SiblingArgument(db *bun.DB, model any) {
	keep(db.NewInsert().Model(model), db.NewInsert().TableExpr("alpha.records"))
}
func ForeignAfter(db *bun.DB, model any) {
	db.NewInsert().Model(model).TableExpr("beta.records")
}
func ForeignModel(db *bun.DB) {
	db.NewInsert().Model(&foreignRecord{}).TableExpr("alpha.records")
}
func keep(...*bun.InsertQuery) {}
