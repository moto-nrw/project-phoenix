package models

// Compile-time interface compliance checks for models with the conventional
// ID/created_at/updated_at shape.
//
// If a conventional model fails to implement base.Entity
// (GetID, GetCreatedAt, GetUpdatedAt), this file will fail to compile.

import (
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Compile-time assertions for base.Entity interface compliance
var (
	// activities package

	// schedule package

	// users package
	_ base.Entity = (*users.Guest)(nil)
	_ base.Entity = (*users.GuardianProfile)(nil)
	_ base.Entity = (*users.Person)(nil)
	_ base.Entity = (*users.PrivacyConsent)(nil)
	_ base.Entity = (*users.Staff)(nil)
	_ base.Entity = (*users.Student)(nil)
	_ base.Entity = (*users.StudentGuardian)(nil)
	_ base.Entity = (*users.Teacher)(nil)
)
