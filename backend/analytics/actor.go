package analytics

import "strings"

// Deployment is the deployment property of every analytics event: "demo"
// for the public demo, otherwise the tenant domain of this instance. The
// frontend sends the same value (frontend/src/lib/analytics-deployment.ts).
func Deployment(demo bool, tenantDomain string) string {
	if demo {
		return "demo"
	}
	return strings.TrimSpace(tenantDomain)
}

// SessionFacts are the session facts the analytics surface and role follow
// from.
type SessionFacts struct {
	Platform        bool
	Parent          bool
	School          bool
	TenantID        int64
	ReadOnlyPreview bool
	Admin           bool
	AccountID       int64
}

// ActorOf maps a session to the analytics surface and role, the same values
// the frontend registers. Role names are school data, so the OGS portal
// reports only the admin flag. Operator sessions are no portal. Only an OGS
// session names its account, for the pseudonymous ID under Analyse-Freigabe;
// the read-only staff preview (#2893) is an admin looking at someone else's
// view, so it names none.
func ActorOf(session SessionFacts) (Actor, bool) {
	switch {
	case session.Platform:
		return Actor{}, false
	case session.Parent:
		return Actor{Surface: SurfaceParents, Role: RoleGuardian}, true
	case session.School:
		return Actor{Surface: SurfaceSchool, Role: RoleLehrkraft, SchoolID: session.TenantID}, true
	case session.TenantID == 0:
		return Actor{}, false
	case session.ReadOnlyPreview:
		return Actor{Surface: SurfaceOGS, Role: RoleAdmin, SchoolID: session.TenantID}, true
	case session.Admin:
		return Actor{Surface: SurfaceOGS, Role: RoleAdmin, SchoolID: session.TenantID, AccountID: session.AccountID}, true
	default:
		return Actor{Surface: SurfaceOGS, Role: RoleStaff, SchoolID: session.TenantID, AccountID: session.AccountID}, true
	}
}
