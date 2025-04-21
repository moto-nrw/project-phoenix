//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/spf13/viper"
)

func init() {
	// Load configuration
	viper.SetConfigName("dev.env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Using config file: %s\n", viper.ConfigFileUsed())
}

func main() {
	fmt.Printf("Trying to connect to DB: %s\n", viper.GetString("db_dsn"))

	// Connect to the database
	db, err := database.DBConn()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run a simple query
	var tables []string
	rows, err := db.QueryContext(nil, "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename")
	if err != nil {
		log.Fatalf("Failed to query tables: %v", err)
	}
	defer rows.Close()

	// Process the results
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Fatalf("Error scanning row: %v", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error reading rows: %v", err)
	}

	fmt.Printf("Connected successfully and found %d tables:\n", len(tables))
	for _, table := range tables {
		fmt.Printf("- %s\n", table)
	}
}
