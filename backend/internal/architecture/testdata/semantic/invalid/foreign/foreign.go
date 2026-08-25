package foreign

import (
	"github.com/uptrace/bun"

	persistencemodel "example.test/architecture-semantic/models/persistence"
)

func ReadAndWrite(db *bun.DB) {
	db.NewSelect().TableExpr("beta.records")
	db.NewSelect().TableExpr("alpha.records").Join("JOIN beta.joined_records ON true")
	db.NewSelect().TableExpr("alpha.records").ColumnExpr("(SELECT count(*) FROM beta.fragment_records JOIN ghost.records ON true)")
	db.NewUpdate().TableExpr("beta.records")
	db.Exec("TRUNCATE TABLE alpha.records, beta.truncated_records")
	db.Exec("MERGE INTO beta.merged_records USING alpha.records ON false WHEN MATCHED THEN DELETE")
	db.Exec("MERGE INTO alpha.records USING beta.merge_source ON false WHEN MATCHED THEN DELETE")
	db.Exec("DELETE FROM alpha.records USING alpha.records, beta.delete_source WHERE false")
	db.Exec("SELECT * FROM alpha.records, beta.comma_source WHERE false")
	db.Exec("SELECT * FROM unqualified_records")
}

func DynamicJoin(db *bun.DB, join string) {
	db.NewSelect().TableExpr("alpha.records").Join(join)
}

func WriteModel(db *bun.DB, record *persistencemodel.Record) {
	db.NewInsert().Model(record)
}
