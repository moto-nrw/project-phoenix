package users

// Compile-time interface compliance checks for models with the conventional
// ID/created_at/updated_at shape.
//
// If a conventional model fails to implement entity
// (GetID, GetCreatedAt, GetUpdatedAt), this file will fail to compile.

import (
	"time"
)

// entity is the exact conventional-row method set formerly asserted through base.Entity.
type entity interface {
	GetID() interface{}
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
}

// Compile-time assertions for entity interface compliance
var (
	// activities package

	// schedule package

	// users package
	_ entity = (*Guest)(nil)
	_ entity = (*GuardianProfile)(nil)
	_ entity = (*Person)(nil)
	_ entity = (*PrivacyConsent)(nil)
	_ entity = (*Staff)(nil)
	_ entity = (*Student)(nil)
	_ entity = (*StudentGuardian)(nil)
	_ entity = (*Teacher)(nil)
)
