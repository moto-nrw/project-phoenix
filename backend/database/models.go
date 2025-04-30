// Package database implements postgres connection and queries.
package database

import (
	"github.com/moto-nrw/project-phoenix/models"
	"github.com/uptrace/bun"
)

// RegisterModels registers all models with the Bun ORM to ensure proper
// relationship handling, especially for junction tables in many-to-many relationships
func RegisterModels(db *bun.DB) {
	// Register junction tables for many-to-many relationships
	db.RegisterModel((*models.GroupSupervisor)(nil))
	db.RegisterModel((*models.CombinedGroupGroup)(nil))
	db.RegisterModel((*models.CombinedGroupSpecialist)(nil))
}