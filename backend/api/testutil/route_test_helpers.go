//go:build testhooks

package testutil

import "github.com/moto-ogs/project-phoenix/api/testutil/routetest"

var (
	ExecuteRequestForTest      = routetest.ExecuteRequestForTest
	MintTestJWT                = routetest.MintTestJWT
	ExecuteWithAuth            = routetest.ExecuteWithAuth
	ExecuteWithAuthPermissions = routetest.ExecuteWithAuthPermissions
)
