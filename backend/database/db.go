// Package database implements postgres connection and queries.
package database

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// DBConn returns a postgres connection pool.
func DBConn() (*bun.DB, error) {
	// Get DSN from environment with smart defaults based on APP_ENV
	dsn := GetDatabaseDSN()

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	db := bun.NewDB(sqldb, pgdialect.New())

	if err := checkConn(db); err != nil {
		return nil, err
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
