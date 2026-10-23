package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeploymentNamesTheDemoOrTheTenantDomain(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "demo", Deployment(true, "schule.test"))
	assert.Equal(t, "schule.test", Deployment(false, " schule.test "))
}

func TestActorOfNamesOnlyOGSAccounts(t *testing.T) {
	t.Parallel()
	_, ok := ActorOf(SessionFacts{Platform: true, TenantID: 21})
	assert.False(t, ok, "operator sessions are no portal")
	_, ok = ActorOf(SessionFacts{AccountID: 31})
	assert.False(t, ok, "an OGS session without a school is none")

	actor, ok := ActorOf(SessionFacts{Parent: true, TenantID: 21, AccountID: 31})
	assert.True(t, ok)
	assert.Equal(t, Actor{Surface: SurfaceParents, Role: RoleGuardian}, actor)
	actor, _ = ActorOf(SessionFacts{Caterer: true, AccountID: 31})
	assert.Equal(t, Actor{Surface: SurfaceCaterer, Role: RoleCaterer}, actor, "a caterer names neither school nor account")
	actor, _ = ActorOf(SessionFacts{School: true, TenantID: 21, AccountID: 31})
	assert.Equal(t, Actor{Surface: SurfaceSchool, Role: RoleLehrkraft, SchoolID: 21}, actor)
	actor, _ = ActorOf(SessionFacts{TenantID: 21, ReadOnlyPreview: true, Admin: true, AccountID: 31})
	assert.Equal(t, Actor{Surface: SurfaceOGS, Role: RoleAdmin, SchoolID: 21}, actor, "the preview names no account")
	actor, _ = ActorOf(SessionFacts{TenantID: 21, Admin: true, AccountID: 31})
	assert.Equal(t, Actor{Surface: SurfaceOGS, Role: RoleAdmin, SchoolID: 21, AccountID: 31}, actor)
	actor, _ = ActorOf(SessionFacts{TenantID: 21, AccountID: 31})
	assert.Equal(t, Actor{Surface: SurfaceOGS, Role: RoleStaff, SchoolID: 21, AccountID: 31}, actor)
}
