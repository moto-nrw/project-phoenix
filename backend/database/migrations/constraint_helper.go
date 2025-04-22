package migrations

import (
	"context"
	"database/sql"
	"log"

	"github.com/moto-nrw/project-phoenix/database"
)

// FixConstraints applies all necessary constraints to match the ER diagram
// This can be called after migrations to ensure constraints are correct
func FixConstraints() {
	db, err := database.DBConn()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	// Apply constraints in a transaction to ensure atomicity
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		log.Fatalf("Error starting transaction: %v", err)
	}

	// Collection of all the fixes we need to apply
	fixes := []string{
		// 1. Fix groups.name to be UNIQUE and INDEXED
		`ALTER TABLE groups ADD CONSTRAINT IF NOT EXISTS groups_name_key UNIQUE (name);`,
		`CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);`,

		// 2. Fix room_occupancy.device_id to be NOT NULL, UNIQUE and INDEXED
		`UPDATE room_occupancy SET device_id = 'device_' || id WHERE device_id IS NULL;`,
		`ALTER TABLE room_occupancy ALTER COLUMN device_id SET NOT NULL;`,
		`ALTER TABLE room_occupancy ADD CONSTRAINT IF NOT EXISTS room_occupancy_device_id_key UNIQUE (device_id);`,
		`CREATE INDEX IF NOT EXISTS idx_room_occupancy_device_id ON room_occupancy(device_id);`,

		// 3. Fix students.group_id foreign key to use ON DELETE CASCADE
		`ALTER TABLE students DROP CONSTRAINT IF EXISTS students_group_id_fkey;`,
		`ALTER TABLE students ADD CONSTRAINT students_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE;`,
	}

	// Apply each fix
	for _, fix := range fixes {
		_, err = tx.ExecContext(ctx, fix)
		if err != nil {
			tx.Rollback()
			log.Printf("Error applying fix: %s, Error: %v", fix, err)
			// Continue with other fixes - not fatal
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		log.Fatalf("Error committing transaction: %v", err)
	}

	log.Println("All constraints successfully applied to match ER diagram!")
}
