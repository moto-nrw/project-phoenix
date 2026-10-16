//go:build testhooks

package api

import (
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
)

// The owner builders that test graphs call through the typed root.

var (
	// NewRoomSnapshotPeople reads the room snapshot's students through the
	// People Directory.
	NewRoomSnapshotPeople = peopleCompose.NewRoomSnapshotPeople
	// AccountExists is the People Directory's account-reference check over
	// Identity & Access, as the root binds it.
	AccountExists = identityaccessCompose.AccountExists
)

// AccountRoleGrantPolicy returns Security Runtime's role grant decision the
// root binds to the account routes.
func AccountRoleGrantPolicy() RoleGrantPolicy {
	return RoleGrantPolicy{}
}
