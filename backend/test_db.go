//go:build ignore
// +build ignore

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/viper"
	_ "github.com/uptrace/bun/driver/pgdriver"
)

func main() {
	// Load configuration
	viper.SetConfigName("dev.env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Using config file: %s\n", viper.ConfigFileUsed())

	// Get DSN from config
	dsn := viper.GetString("db_dsn")
	fmt.Printf("Connecting to: %s\n", dsn)

	// Connect directly with sql.DB
	db, err := sql.Open("pg", dsn)
	if err != nil {
		fmt.Printf("Failed to open connection: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Test connection
	err = db.PingContext(context.Background())
	if err != nil {
		fmt.Printf("Failed to ping database: %v\n", err)
		os.Exit(1)
	}

	// Query tables
	rows, err := db.QueryContext(context.Background(),
		"SELECT tablename, schemaname FROM pg_tables")
	if err != nil {
		fmt.Printf("Failed to query tables: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	// Print results
	fmt.Println("Tables in database:")
	for rows.Next() {
		var tableName, schemaName string
		if err := rows.Scan(&tableName, &schemaName); err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			continue
		}
		fmt.Printf("- %s.%s\n", schemaName, tableName)
	}

	if err := rows.Err(); err != nil {
		fmt.Printf("Error iterating rows: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Connection test successful!")
}
