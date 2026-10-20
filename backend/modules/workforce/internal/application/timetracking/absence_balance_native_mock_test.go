package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func absenceMockValues(rows []*StaffAbsence) []workforce.StaffAbsence {
	result := make([]workforce.StaffAbsence, 0, len(rows))
	for _, r := range rows {
		if r != nil {
			result = append(result, StaffAbsenceToCapability(r))
		}
	}
	return result
}
func adjustmentMockValues(rows []*StaffBalanceAdjustment) []workforce.StaffBalanceAdjustment {
	result := make([]workforce.StaffBalanceAdjustment, 0, len(rows))
	for _, r := range rows {
		if r != nil {
			result = append(result, StaffBalanceAdjustmentToCapability(r))
		}
	}
	return result
}

func (m *absStaffAbsenceRepoMock) CreateStaffAbsence(ctx context.Context, v *workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	r := absenceToRecord(*v)
	err := m.Create(ctx, r)
	return StaffAbsenceToCapability(r), err
}
func (m *absStaffAbsenceRepoMock) UpdateStaffAbsence(ctx context.Context, v workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	r := absenceToRecord(v)
	err := m.Update(ctx, r)
	return StaffAbsenceToCapability(r), err
}
func (m *absStaffAbsenceRepoMock) FindStaffAbsence(ctx context.Context, id int64) (workforce.StaffAbsence, error) {
	r, err := m.FindByID(ctx, id)
	if r == nil {
		return workforce.StaffAbsence{}, err
	}
	return StaffAbsenceToCapability(r), err
}
func (m *absStaffAbsenceRepoMock) DeleteStaffAbsence(ctx context.Context, id int64) error {
	return m.Delete(ctx, id)
}
func (m *absStaffAbsenceRepoMock) CountStaffAbsences(ctx context.Context, f workforce.StaffAbsenceFilter) (int, error) {
	return m.CountWithFilter(ctx, f)
}
func (m *absStaffAbsenceRepoMock) OldestStaffAbsenceDate(ctx context.Context, col, before string) (string, error) {
	var d *Date
	if before != "" {
		v := Date(before)
		d = &v
	}
	r, err := m.OldestBefore(ctx, col, d)
	if r == nil {
		return "", err
	}
	return r.String(), err
}
func (m *absStaffAbsenceRepoMock) DeleteStaffAbsencesOlderThan(ctx context.Context, col, cutoff string) (int64, error) {
	return m.DeleteOlderThan(ctx, col, Date(cutoff))
}
func (m *absStaffAbsenceRepoMock) StaffAbsenceMapForDate(ctx context.Context, date string) (map[int64]string, error) {
	return m.GetAbsenceMapForDate(ctx, Date(date))
}
func (m *absStaffAbsenceRepoMock) StaffAbsenceTypeIDMapForDate(ctx context.Context, date string) (map[int64]int64, error) {
	return m.GetAbsenceTypeIDMapForDate(ctx, Date(date))
}
func (m *absStaffAbsenceRepoMock) ListStaffAbsences(ctx context.Context, f workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error) {
	var rows []*StaffAbsence
	var err error
	switch {
	case f.StaffIDs != nil:
		grouped, e := m.GetByStaffIDsAndDateRange(ctx, f.StaffIDs, Date(f.OverlapFrom), Date(f.OverlapTo))
		err = e
		for _, id := range f.StaffIDs {
			rows = append(rows, grouped[id]...)
		}
	case f.Limit == 1 && f.StaffID != 0:
		row, e := m.GetByStaffAndDate(ctx, f.StaffID, Date(f.OverlapFrom))
		err = e
		if row != nil {
			rows = append(rows, row)
		}
	case f.StaffID != 0 && f.OverlapFrom != "" && f.OverlapTo != "":
		rows, err = m.GetByStaffAndDateRange(ctx, f.StaffID, Date(f.OverlapFrom), Date(f.OverlapTo))
	case len(f.Statuses) > 0 && f.StaffID == 0:
		rows, err = m.ListByStatuses(ctx, f.Statuses)
	default:
		rows, err = m.ListWithFilter(ctx, f)
	}
	return absenceMockValues(rows), err
}

func (m *wsMockAbsenceRecords) CreateStaffAbsence(ctx context.Context, v *workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	r := absenceToRecord(*v)
	err := m.Create(ctx, r)
	return StaffAbsenceToCapability(r), err
}
func (m *wsMockAbsenceRecords) UpdateStaffAbsence(ctx context.Context, v workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	r := absenceToRecord(v)
	err := m.Update(ctx, r)
	return StaffAbsenceToCapability(r), err
}
func (m *wsMockAbsenceRecords) FindStaffAbsence(ctx context.Context, id int64) (workforce.StaffAbsence, error) {
	r, err := m.FindByID(ctx, id)
	if r == nil {
		return workforce.StaffAbsence{}, err
	}
	return StaffAbsenceToCapability(r), err
}
func (m *wsMockAbsenceRecords) DeleteStaffAbsence(ctx context.Context, id int64) error {
	return m.Delete(ctx, id)
}
func (m *wsMockAbsenceRecords) CountStaffAbsences(ctx context.Context, f workforce.StaffAbsenceFilter) (int, error) {
	return m.CountWithFilter(ctx, f)
}
func (m *wsMockAbsenceRecords) OldestStaffAbsenceDate(ctx context.Context, col, before string) (string, error) {
	var d *Date
	if before != "" {
		v := Date(before)
		d = &v
	}
	r, err := m.OldestBefore(ctx, col, d)
	if r == nil {
		return "", err
	}
	return r.String(), err
}
func (m *wsMockAbsenceRecords) DeleteStaffAbsencesOlderThan(ctx context.Context, col, cutoff string) (int64, error) {
	return m.DeleteOlderThan(ctx, col, Date(cutoff))
}
func (m *wsMockAbsenceRecords) StaffAbsenceMapForDate(ctx context.Context, date string) (map[int64]string, error) {
	return m.GetAbsenceMapForDate(ctx, Date(date))
}
func (m *wsMockAbsenceRecords) StaffAbsenceTypeIDMapForDate(ctx context.Context, date string) (map[int64]int64, error) {
	return m.GetAbsenceTypeIDMapForDate(ctx, Date(date))
}
func (m *wsMockAbsenceRecords) ListStaffAbsences(ctx context.Context, f workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error) {
	var rows []*StaffAbsence
	var err error
	switch {
	case f.StaffIDs != nil:
		grouped, e := m.GetByStaffIDsAndDateRange(ctx, f.StaffIDs, Date(f.OverlapFrom), Date(f.OverlapTo))
		err = e
		for _, id := range f.StaffIDs {
			rows = append(rows, grouped[id]...)
		}
	case f.Limit == 1 && f.StaffID != 0:
		row, e := m.GetByStaffAndDate(ctx, f.StaffID, Date(f.OverlapFrom))
		err = e
		if row != nil {
			rows = append(rows, row)
		}
	case f.StaffID != 0 && f.OverlapFrom != "" && f.OverlapTo != "":
		rows, err = m.GetByStaffAndDateRange(ctx, f.StaffID, Date(f.OverlapFrom), Date(f.OverlapTo))
	case len(f.Statuses) > 0 && f.StaffID == 0:
		rows, err = m.ListByStatuses(ctx, f.Statuses)
	default:
		rows, err = m.ListWithFilter(ctx, f)
	}
	return absenceMockValues(rows), err
}

func (m *wtmMockAbsenceReader) ListStaffAbsences(ctx context.Context, f workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error) {
	rows, err := m.GetByStaffAndDateRange(ctx, f.StaffID, Date(f.OverlapFrom), Date(f.OverlapTo))
	return absenceMockValues(rows), err
}
func (m *wtmMockAdjustmentReader) ListStaffBalanceAdjustments(ctx context.Context, f workforce.StaffBalanceAdjustmentFilter) ([]workforce.StaffBalanceAdjustment, error) {
	rows, err := m.GetByStaffAndDateRange(ctx, f.StaffID, Date(f.EffectiveFrom), Date(f.EffectiveTo))
	return adjustmentMockValues(rows), err
}
func (m *absVacationQuotaRepoMock) ListStaffVacationQuotas(ctx context.Context, f workforce.StaffVacationFilter) ([]workforce.StaffVacationQuota, error) {
	rows := []workforce.StaffVacationQuota{}
	if f.StaffIDs != nil {
		grouped, err := m.GetByStaffIDsAndYear(ctx, f.StaffIDs, f.Year)
		for _, id := range f.StaffIDs {
			if r := grouped[id]; r != nil {
				rows = append(rows, quotaToCapability(r))
			}
		}
		return rows, err
	}
	r, err := m.GetByStaffAndYear(ctx, f.StaffID, f.Year)
	if r != nil {
		rows = append(rows, quotaToCapability(r))
	}
	return rows, err
}
func (m *absVacationQuotaRepoMock) UpsertStaffVacationQuota(ctx context.Context, v workforce.StaffVacationQuota) error {
	return m.Upsert(ctx, quotaToRecord(v))
}
func (m *voOpeningRepoMock) ListStaffVacationOpenings(ctx context.Context, f workforce.StaffVacationFilter) ([]workforce.StaffVacationOpening, error) {
	rows := []workforce.StaffVacationOpening{}
	if f.StaffIDs != nil {
		grouped, err := m.GetByStaffIDsAndYear(ctx, f.StaffIDs, f.Year)
		for _, id := range f.StaffIDs {
			if r := grouped[id]; r != nil {
				rows = append(rows, openingToCapability(r))
			}
		}
		return rows, err
	}
	r, err := m.GetByStaffAndYear(ctx, f.StaffID, f.Year)
	if r != nil {
		rows = append(rows, openingToCapability(r))
	}
	return rows, err
}
func (m *voOpeningRepoMock) CreateStaffVacationOpening(ctx context.Context, v workforce.StaffVacationOpening) (workforce.StaffVacationOpening, error) {
	r := openingToRecord(v)
	err := m.Create(ctx, r)
	return openingToCapability(r), err
}
func (m *voOpeningRepoMock) DeleteStaffVacationOpening(ctx context.Context, id int64) error {
	return m.Delete(ctx, id)
}
func (m *absStaffAbsenceAuditRepoMock) RecordStaffAbsenceAudit(ctx context.Context, v workforce.StaffAbsenceAudit) (workforce.StaffAbsenceAudit, error) {
	r := &StaffAbsenceAudit{AbsenceID: v.AbsenceID, FromStatus: v.FromStatus, ToStatus: v.ToStatus, ActorID: v.ActorID, Note: v.Note, ChangedAt: v.ChangedAt, TypeChange: (*StaffAbsenceTypeChange)(v.TypeChange)}
	r.TenantID = v.TenantID
	err := m.Create(ctx, r)
	v.ID = r.ID
	v.TenantID = r.TenantID
	v.ChangedAt = r.ChangedAt
	return v, err
}

func (m *recordingBalanceAdjustmentRepo) CreateStaffBalanceAdjustment(ctx context.Context, v workforce.StaffBalanceAdjustment) (workforce.StaffBalanceAdjustment, error) {
	r := adjustmentToRecord(v)
	err := m.Create(ctx, r)
	return StaffBalanceAdjustmentToCapability(r), err
}
func (m *recordingBalanceAdjustmentRepo) FindStaffBalanceAdjustment(ctx context.Context, id int64) (workforce.StaffBalanceAdjustment, error) {
	r, err := m.FindByID(ctx, id)
	if r == nil {
		return workforce.StaffBalanceAdjustment{}, err
	}
	return StaffBalanceAdjustmentToCapability(r), err
}
func (m *recordingBalanceAdjustmentRepo) DeleteStaffBalanceAdjustment(ctx context.Context, id int64) error {
	return m.Delete(ctx, id)
}
func (m *recordingBalanceAdjustmentRepo) ListStaffBalanceAdjustments(ctx context.Context, f workforce.StaffBalanceAdjustmentFilter) ([]workforce.StaffBalanceAdjustment, error) {
	rows, err := m.ListWithFilter(ctx, f)
	return adjustmentMockValues(rows), err
}

func (m *typeFilteringBalanceAdjustmentRepo) CreateStaffBalanceAdjustment(ctx context.Context, v workforce.StaffBalanceAdjustment) (workforce.StaffBalanceAdjustment, error) {
	r := adjustmentToRecord(v)
	err := m.Create(ctx, r)
	return StaffBalanceAdjustmentToCapability(r), err
}
func (m *typeFilteringBalanceAdjustmentRepo) FindStaffBalanceAdjustment(ctx context.Context, id int64) (workforce.StaffBalanceAdjustment, error) {
	r, err := m.FindByID(ctx, id)
	if r == nil {
		return workforce.StaffBalanceAdjustment{}, err
	}
	return StaffBalanceAdjustmentToCapability(r), err
}
func (m *typeFilteringBalanceAdjustmentRepo) DeleteStaffBalanceAdjustment(ctx context.Context, id int64) error {
	return m.Delete(ctx, id)
}
func (m *typeFilteringBalanceAdjustmentRepo) ListStaffBalanceAdjustments(ctx context.Context, f workforce.StaffBalanceAdjustmentFilter) ([]workforce.StaffBalanceAdjustment, error) {
	rows, err := m.ListWithFilter(ctx, f)
	return adjustmentMockValues(rows), err
}

func (m *failingBalanceAdjustmentRepo) CreateStaffBalanceAdjustment(ctx context.Context, v workforce.StaffBalanceAdjustment) (workforce.StaffBalanceAdjustment, error) {
	r := adjustmentToRecord(v)
	err := m.Create(ctx, r)
	return StaffBalanceAdjustmentToCapability(r), err
}
func (m *failingBalanceAdjustmentRepo) FindStaffBalanceAdjustment(ctx context.Context, id int64) (workforce.StaffBalanceAdjustment, error) {
	r, err := m.FindByID(ctx, id)
	if r == nil {
		return workforce.StaffBalanceAdjustment{}, err
	}
	return StaffBalanceAdjustmentToCapability(r), err
}
func (m *failingBalanceAdjustmentRepo) DeleteStaffBalanceAdjustment(ctx context.Context, id int64) error {
	return m.Delete(ctx, id)
}
func (m *failingBalanceAdjustmentRepo) ListStaffBalanceAdjustments(ctx context.Context, f workforce.StaffBalanceAdjustmentFilter) ([]workforce.StaffBalanceAdjustment, error) {
	rows, err := m.ListWithFilter(ctx, f)
	return adjustmentMockValues(rows), err
}
