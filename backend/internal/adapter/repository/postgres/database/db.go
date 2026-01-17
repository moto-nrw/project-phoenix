// Package database implements postgres connection and queries.
package database

import (
	"context"
	"database/sql"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

// DBConn returns a postgres connection pool.
func DBConn() (*bun.DB, error) {
	// Get DSN from environment (fail fast if missing).
	dsn := GetDatabaseDSN()

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	db := bun.NewDB(sqldb, pgdialect.New())

	if err := checkConn(db); err != nil {
		return nil, err
	}

	if viper.GetBool("db_debug") {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	return db, nil
}

func checkConn(db *bun.DB) error {
	var n int
	return db.NewSelect().ColumnExpr("1").Scan(context.Background(), &n)
}

// InitDB initializes a database connection for CLI commands
func InitDB() (*bun.DB, error) {
	return DBConn()
}
