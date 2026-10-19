package guardianlinkview

import "github.com/uptrace/bun"

// Reader is the projection the roots bind to People Directory's guardian
// link ports (ADR 0058). Its signatures carry only the transaction, so
// neither side names the other's types. Every row source runs in the
// caller's statement and transaction.
type Reader struct{}

// GuardianLinkRows is Query.
func (Reader) GuardianLinkRows(db bun.IDB, tenantID int64) *bun.SelectQuery {
	return Query(db, tenantID)
}

// GuardianLinkModelRows is ModelQuery.
func (Reader) GuardianLinkModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	return ModelQuery(db, tenantID, model)
}

func (Reader) AllSchoolsGuardianLinkRows(db bun.IDB) *bun.SelectQuery { return AllSchoolsQuery(db) }
func (Reader) AllSchoolsGuardianLinkModelRows(db bun.IDB, model any) *bun.SelectQuery {
	return AllSchoolsModelQuery(db, model)
}
