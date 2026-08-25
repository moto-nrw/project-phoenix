package adapter

import "github.com/uptrace/bun"

func Read(db *bun.DB) {
	db.NewSelect().TableExpr("alpha.records")
	db.NewSelect().TableExpr(`"alpha"."records"`)
	db.NewSelect().TableExpr("alpha.records").Join("JOIN alpha.records AS related ON true")
	db.NewSelect().TableExpr("alpha.records").ColumnExpr("(SELECT count(*) FROM alpha.records)")
	db.NewSelect().TableExpr("alpha.records").For("UPDATE SKIP LOCKED")
}

func Write(db *bun.DB) {
	db.NewInsert().TableExpr("alpha.records")
	db.NewInsert().TableExpr("alpha.records").On("CONFLICT DO UPDATE SET value = EXCLUDED.value")
}

func TransactionControl(db *bun.DB) {
	db.Exec("SAVEPOINT architecture_fixture")
	db.Exec("TRUNCATE alpha.records, alpha.audit RESTART IDENTITY CASCADE")
	db.Exec("MERGE INTO alpha.records USING alpha.records AS source ON false WHEN MATCHED THEN DELETE")
	db.Exec("WITH alpha AS (SELECT * FROM alpha.records) SELECT * FROM alpha")
	db.Exec("SELECT source.id, source.value FROM (SELECT * FROM alpha.records, alpha.audit) AS source")
}
