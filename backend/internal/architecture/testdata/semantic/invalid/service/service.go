package service

import (
	"context"
	"database/sql"
	"fmt"

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

type SQLDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Query(string, ...any) (*sql.Rows, error)
	Begin() (*sql.Tx, error)
}

func SQLAccessInterface(db SQLDB) {
	db.ExecContext(context.Background(), "SELECT * FROM beta.fragment_records")
}

type PingDB interface {
	Ping() error
	PingContext(context.Context) error
}

func PingDatabase(db PingDB) {
	_ = db.Ping()
	_ = db.PingContext(context.Background())
}

type MixedDB interface {
	NewSelect() *bun.SelectQuery
	Close() error
}

func CloseMixedDB(db MixedDB) {
	_ = db.Close()
}

type Event struct{}

type EventExecutor interface {
	Exec(Event) error
}

func ExecuteEvent(executor EventExecutor, event Event) {
	_ = executor.Exec(event)
}

func PackageCall() error {
	return fmt.Errorf("package-qualified calls have no receiver type")
}
