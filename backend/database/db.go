// Package database implements postgres connection and queries.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

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
	pool, err := poolConfigFromEnv()
	if err != nil {
		return nil, err
	}
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	sqldb.SetMaxOpenConns(pool.maxOpen)
	sqldb.SetMaxIdleConns(pool.maxIdle)
	sqldb.SetConnMaxLifetime(pool.maxLifetime)
	sqldb.SetConnMaxIdleTime(pool.maxIdleTime)

	slog.Info(logMsg,
		"max_open_conns", pool.maxOpen,
		"max_idle_conns", pool.maxIdle,
		"conn_max_lifetime", pool.maxLifetime.String(),
		"conn_max_idle_time", pool.maxIdleTime.String(),
	)

	db := bun.NewDB(sqldb, pgdialect.New())

	if err := checkConn(db); err != nil {
		return nil, errors.Join(err, db.Close())
	}

	return db, nil
}

type poolConfig struct {
	maxOpen     int
	maxIdle     int
	maxLifetime time.Duration
	maxIdleTime time.Duration
}

func poolConfigFromEnv() (poolConfig, error) {
	maxOpen, err := requiredPositiveInt("DB_MAX_OPEN_CONNS")
	if err != nil {
		return poolConfig{}, err
	}
	maxIdle, err := requiredPositiveInt("DB_MAX_IDLE_CONNS")
	if err != nil {
		return poolConfig{}, err
	}
	maxLifetime, err := requiredPositiveDuration("DB_CONN_MAX_LIFETIME")
	if err != nil {
		return poolConfig{}, err
	}
	maxIdleTime, err := requiredPositiveDuration("DB_CONN_MAX_IDLE_TIME")
	if err != nil {
		return poolConfig{}, err
	}
	return poolConfig{maxOpen: maxOpen, maxIdle: maxIdle, maxLifetime: maxLifetime, maxIdleTime: maxIdleTime}, nil
}

func requiredPositiveInt(name string) (int, error) {
	raw := os.Getenv(name)
	value, err := strconv.Atoi(raw)
	if raw == "" || err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func requiredPositiveDuration(name string) (time.Duration, error) {
	raw := os.Getenv(name)
	value, err := time.ParseDuration(raw)
	if raw == "" || err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
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

// ClosePool releases a pool owned by a process composition root.
func ClosePool(db *bun.DB) error {
	return db.Close()
}

// InitDB initializes a database connection for CLI commands
func InitDB() (*bun.DB, error) {
	return DBConn()
}
