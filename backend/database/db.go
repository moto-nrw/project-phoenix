// Package database implements postgres connection and queries.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
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
	config, err := resolvePoolConfig()
	if err != nil {
		return nil, err
	}

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	sqldb.SetMaxOpenConns(config.maxOpen)
	sqldb.SetMaxIdleConns(config.maxIdle)
	sqldb.SetConnMaxLifetime(config.lifetime)
	sqldb.SetConnMaxIdleTime(config.idleTime)

	slog.Info(logMsg,
		"max_open_conns", config.maxOpen,
		"max_idle_conns", config.maxIdle,
		"conn_max_lifetime", config.lifetime.String(),
		"conn_max_idle_time", config.idleTime.String(),
	)

	db := bun.NewDB(sqldb, pgdialect.New())

	if err := checkConn(db); err != nil {
		return nil, errors.Join(err, db.Close())
	}

	return db, nil
}

type poolConfig struct {
	maxOpen  int
	maxIdle  int
	lifetime time.Duration
	idleTime time.Duration
}

func resolvePoolConfig() (poolConfig, error) {
	maxOpen, err := requiredPositiveInt("db_max_open_conns", "DB_MAX_OPEN_CONNS")
	if err != nil {
		return poolConfig{}, err
	}
	maxIdle, err := requiredPositiveInt("db_max_idle_conns", "DB_MAX_IDLE_CONNS")
	if err != nil {
		return poolConfig{}, err
	}
	if maxIdle > maxOpen {
		return poolConfig{}, fmt.Errorf("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	lifetime, err := requiredPositiveDuration("db_conn_max_lifetime", "DB_CONN_MAX_LIFETIME")
	if err != nil {
		return poolConfig{}, err
	}
	idleTime, err := requiredPositiveDuration("db_conn_max_idle_time", "DB_CONN_MAX_IDLE_TIME")
	if err != nil {
		return poolConfig{}, err
	}
	return poolConfig{maxOpen: maxOpen, maxIdle: maxIdle, lifetime: lifetime, idleTime: idleTime}, nil
}

func requiredPositiveInt(key, envName string) (int, error) {
	raw := viper.GetString(key)
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", envName)
	}
	return value, nil
}

func requiredPositiveDuration(key, envName string) (time.Duration, error) {
	raw := viper.GetString(key)
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", envName)
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
