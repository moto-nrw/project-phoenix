// Package database implements postgres connection and queries.
package database

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// DBConn returns a postgres connection pool.
func DBConn() (*bun.DB, error) {
	// Get DSN from environment with smart defaults based on APP_ENV
	return openPool(GetDatabaseDSN(), "database pool configured")
}

// openPool opens a connection pool for dsn with the shared viper-driven pool
// sizing and verifies connectivity. logMsg distinguishes the superuser pool
// from the phoenix_auth serve pool in startup logs.
func openPool(dsn, logMsg string) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	// Connection pool configuration
	maxOpen := viper.GetInt("db_max_open_conns")
	if maxOpen == 0 {
		maxOpen = 25
	}
	maxIdle := viper.GetInt("db_max_idle_conns")
	if maxIdle == 0 {
		maxIdle = 10
	}
	lifetime := viper.GetDuration("db_conn_max_lifetime")
	if lifetime == 0 {
		lifetime = 5 * time.Minute
	}
	idleTime := viper.GetDuration("db_conn_max_idle_time")
	if idleTime == 0 {
		idleTime = 1 * time.Minute
	}

	sqldb.SetMaxOpenConns(maxOpen)
	sqldb.SetMaxIdleConns(maxIdle)
	sqldb.SetConnMaxLifetime(lifetime)
	sqldb.SetConnMaxIdleTime(idleTime)

	slog.Info(logMsg,
		"max_open_conns", maxOpen,
		"max_idle_conns", maxIdle,
		"conn_max_lifetime", lifetime.String(),
		"conn_max_idle_time", idleTime.String(),
	)

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

// DBConnForServe returns a postgres connection pool using the phoenix_auth role.
// Used by the HTTP server to enforce least-privilege at the connection level.
func DBConnForServe() (*bun.DB, error) {
	return openPool(GetServeDSN(), "database pool configured (phoenix_auth)")
}

// InitDB initializes a database connection for CLI commands
func InitDB() (*bun.DB, error) {
	return DBConn()
}
