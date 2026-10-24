package communication

import "context"

// StaffColleagueReads supplies the same school-colleague relation for picker
// and authorization. Active membership alone must never replace this relation.
type StaffColleagueReads interface {
	ListAddressableStaff(context.Context, int64) ([]*AddressableStaffRecord, error)
	IsAddressableStaff(context.Context, int64) (bool, error)
	StaffRoleKindsForAccounts(context.Context, []int64) (map[int64]string, error)
}
type staffColleagueReads struct {
	list     func(context.Context, int64) ([]*AddressableStaffRecord, error)
	includes func(context.Context, int64) (bool, error)
	roles    func(context.Context, []int64) (map[int64]string, error)
}

// NewStaffColleagueReads binds all three operations to the owner's public port.
func NewStaffColleagueReads(list func(context.Context, int64) ([]*AddressableStaffRecord, error), includes func(context.Context, int64) (bool, error), roles func(context.Context, []int64) (map[int64]string, error)) StaffColleagueReads {
	return staffColleagueReads{list: list, includes: includes, roles: roles}
}
func (r staffColleagueReads) ListAddressableStaff(ctx context.Context, id int64) ([]*AddressableStaffRecord, error) {
	return r.list(ctx, id)
}
func (r staffColleagueReads) IsAddressableStaff(ctx context.Context, id int64) (bool, error) {
	return r.includes(ctx, id)
}
func (r staffColleagueReads) StaffRoleKindsForAccounts(ctx context.Context, ids []int64) (map[int64]string, error) {
	return r.roles(ctx, ids)
}
