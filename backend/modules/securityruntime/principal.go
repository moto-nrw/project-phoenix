package securityruntime

// PrincipalFacts are authenticated session values, independent of JWT and HTTP.
type PrincipalFacts struct {
	AccountID      int64
	TenantID       int64
	OrganizationID int64
	Scope          string
	Roles          []string
	Permissions    []string
	Admin          bool
	FamilyID       string
}
