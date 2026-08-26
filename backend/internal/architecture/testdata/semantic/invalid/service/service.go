package service

import (
	"database/sql"

	"github.com/uptrace/bun"
)

func Read(db *bun.DB) {
	db.NewSelect().TableExpr("alpha.records")
}

type WrappedDB struct{ *bun.DB }

func ReadWrapped(db *WrappedDB) {
	db.NewSelect()
}

func Control(db *bun.DB) {
	db.Begin()
}

type SelectDB interface {
	NewSelect() *bun.SelectQuery
}

func ReadInterface(db SelectDB) {
	db.NewSelect().TableExpr("beta.records")
}

func SQLAccess(conn *sql.Conn, stmt *sql.Stmt) {
	conn.ExecContext(nil, "SELECT * FROM beta.records")
	stmt.Exec()
}
