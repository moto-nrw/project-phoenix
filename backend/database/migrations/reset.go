package migrations

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database"
)

// ResetDatabase drops all tables in the database to start fresh
func ResetDatabase() error {
	db, err := database.DBConn()
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("Resetting database: Dropping all tables...")

	// Use PostgreSQL's information schema to get all tables
	rows, err := db.QueryContext(context.Background(), `
		SELECT tablename FROM pg_tables 
		WHERE schemaname = 'public' 
		ORDER BY tablename
	`)
	if err != nil {
		return fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	// Collect all table names
	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating tables: %w", err)
	}

	if len(tables) == 0 {
		fmt.Println("No tables found to drop")
		return nil
	}

	// First disable all triggers
	_, err = db.ExecContext(context.Background(), "SET session_replication_role = 'replica'")
	if err != nil {
		return fmt.Errorf("failed to disable triggers: %w", err)
	}

	// Generate and execute DROP TABLE statements
	fmt.Printf("Found %d tables to drop\n", len(tables))

	// Drop all tables in a single statement
	query := "DROP TABLE IF EXISTS "
	for i, table := range tables {
		if i > 0 {
			query += ", "
		}
		query += table
	}
	query += " CASCADE"

	_, err = db.ExecContext(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to drop tables: %w", err)
	}

	// Re-enable triggers
	_, err = db.ExecContext(context.Background(), "SET session_replication_role = 'origin'")
	if err != nil {
		return fmt.Errorf("failed to re-enable triggers: %w", err)
	}

	fmt.Println("Database reset complete")
	return nil
}
