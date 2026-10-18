package httpintegration_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"

	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedRequestSession struct {
	claims      ports.RequestClaims
	permissions []string
}

func (s fixedRequestSession) Claims(context.Context) ports.RequestClaims { return s.claims }

func (s fixedRequestSession) Permissions(context.Context) []string { return s.permissions }

func TestNewOperationsRequiresTheRequestSession(t *testing.T) {
	t.Parallel()
	operations, err := timetableCompose.NewOperations(ports.OperationDependencies{}, api.TimetableOperationCaller{})
	require.EqualError(t, err, "timetable operations: the request session is required")
	assert.Nil(t, operations)
}

func TestOperationCallerReadsTheRequestSession(t *testing.T) {
	t.Parallel()
	ctx := tenant.WithTenantID(context.Background(), 7)

	school := api.TimetableOperationCaller{RequestSession: fixedRequestSession{claims: ports.RequestClaims{Scope: "school", SchoolScope: true}}}
	assert.True(t, school.AssignmentBoundPortal(ctx))
	assert.False(t, api.TimetableOperationCaller{RequestSession: fixedRequestSession{}}.AssignmentBoundPortal(ctx))

	assert.True(t, api.TimetableOperationCaller{RequestSession: fixedRequestSession{claims: ports.RequestClaims{Admin: true}}}.AdministrativeAccess(ctx))
	assert.True(t, api.TimetableOperationCaller{RequestSession: fixedRequestSession{permissions: []string{"admin:*"}}}.AdministrativeAccess(ctx))
	assert.False(t, api.TimetableOperationCaller{RequestSession: fixedRequestSession{permissions: []string{"schedules:read"}}}.AdministrativeAccess(ctx))

	actor := fixedRequestSession{claims: ports.RequestClaims{AccountID: 42, TenantID: 7, Scope: "tenant"}, permissions: []string{"schedules:read"}}
	assert.True(t, api.TimetableOperationCaller{RequestSession: actor}.OGSActorToken(ctx, 42))
	assert.False(t, api.TimetableOperationCaller{RequestSession: actor}.OGSActorToken(ctx, 43), "another account")
	assert.False(t, api.TimetableOperationCaller{RequestSession: actor}.OGSActorToken(tenant.WithTenantID(context.Background(), 8), 42), "another tenant")
	school.RequestSession = fixedRequestSession{claims: ports.RequestClaims{AccountID: 42, TenantID: 7, Scope: "school", SchoolScope: true}, permissions: []string{"schedules:read"}}
	assert.False(t, school.OGSActorToken(ctx, 42), "a school session is no OGS actor token")
	actor.permissions = nil
	assert.False(t, api.TimetableOperationCaller{RequestSession: actor}.OGSActorToken(ctx, 42), "without schedules:read")
}
