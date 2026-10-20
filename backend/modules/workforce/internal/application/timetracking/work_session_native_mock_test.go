package timetracking

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Native test ports reuse the scenario callbacks so behavioral assertions stay
// on the same arrangements after the generic row port is removed.
func nativeTestSession(value *WorkSession, err error) (workforce.WorkSession, error) {
	if value == nil && err == nil {
		err = workforce.ErrWorkSessionNotFound
	}
	return WorkSessionToCapability(value), err
}
func nativeTestSessions(values []*WorkSession, err error) ([]workforce.WorkSession, error) {
	result := make([]workforce.WorkSession, 0, len(values))
	for _, value := range values {
		result = append(result, WorkSessionToCapability(value))
	}
	return result, err
}
func nativeTestBreaks(values []*WorkSessionBreak, err error) ([]workforce.WorkSessionBreak, error) {
	result := make([]workforce.WorkSessionBreak, 0, len(values))
	for _, value := range values {
		result = append(result, WorkSessionBreakToCapability(value))
	}
	return result, err
}

func (m *wsMockWorkSessionRepository) FindWorkSession(ctx context.Context, id int64) (workforce.WorkSession, error) {
	return nativeTestSession(m.FindByID(ctx, id))
}

func (m *wsMockWorkSessionRepository) TodayOpenWorkSession(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	return nativeTestSession(m.GetCurrentByStaffID(ctx, staffID))
}

func (m *wsMockWorkSessionRepository) LatestOpenWorkSession(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	return nativeTestSession(m.GetLatestOpenByStaffID(ctx, staffID))
}

func (m *wsMockWorkSessionRepository) OpenWorkSessionOn(ctx context.Context, staffID int64, date string) (workforce.WorkSession, error) {
	return nativeTestSession(m.GetOpenByStaffAndDate(ctx, staffID, Date(date)))
}

func (m *wsMockWorkSessionRepository) LockOpenWorkSessionOn(ctx context.Context, staffID int64, date string) (workforce.WorkSession, error) {
	return nativeTestSession(m.GetOpenByStaffAndDateForUpdate(ctx, staffID, Date(date)))
}

func (m *wsMockWorkSessionRepository) LockOpenWorkSession(ctx context.Context, id int64) (workforce.WorkSession, error) {
	return nativeTestSession(m.LockOpenByIDForUpdate(ctx, id))
}

func (m *wsMockWorkSessionRepository) CreateWorkSession(ctx context.Context, value workforce.WorkSession) (workforce.WorkSession, error) {
	record := workSessionToRecord(value)
	err := m.Create(ctx, record)
	return WorkSessionToCapability(record), err
}

func (m *wsMockWorkSessionRepository) UpdateWorkSession(ctx context.Context, value workforce.WorkSession) (workforce.WorkSession, error) {
	record := workSessionToRecord(value)
	err := m.Update(ctx, record)
	return WorkSessionToCapability(record), err
}

func (m *wsMockWorkSessionRepository) ListWorkSessions(ctx context.Context, filter workforce.WorkSessionFilter) ([]workforce.WorkSession, error) {
	if filter.StaffIDs != nil && (filter.DateFrom != "" || filter.DateTo != "") {
		grouped, err := m.GetHistoryByStaffIDs(ctx, filter.StaffIDs, Date(filter.DateFrom), Date(filter.DateTo))
		result := make([]workforce.WorkSession, 0)
		for _, id := range filter.StaffIDs {
			for _, value := range grouped[id] {
				result = append(result, WorkSessionToCapability(value))
			}
		}
		return result, err
	}
	if filter.StaffID != 0 && (filter.DateFrom != "" || filter.DateTo != "") {
		return nativeTestSessions(m.GetHistoryByStaffID(ctx, filter.StaffID, Date(filter.DateFrom), Date(filter.DateTo)))
	}
	if filter.Open != nil && *filter.Open && filter.DateBefore != "" {
		return nativeTestSessions(m.GetOpenSessions(ctx, Date(filter.DateBefore)))
	}
	return nativeTestSessions(m.ListWithFilter(ctx, filter))
}
func (m *wsMockWorkSessionRepository) ListOverlappingWorkSessions(ctx context.Context, ids []int64, from time.Time, to *time.Time) ([]workforce.WorkSession, error) {
	if len(ids) == 1 && m.listOverlappingByStaffIDsFunc == nil {
		return nativeTestSessions(m.ListOverlappingByStaffID(ctx, ids[0], from, to))
	}
	grouped, err := m.ListOverlappingByStaffIDs(ctx, ids, from, to)
	result := make([]workforce.WorkSession, 0)
	for _, id := range ids {
		for _, value := range grouped[id] {
			result = append(result, WorkSessionToCapability(value))
		}
	}
	return result, err
}
func (m *wsMockWorkSessionRepository) WorkPresenceMap(ctx context.Context) (map[int64]string, error) {
	return m.GetTodayPresenceMap(ctx)
}
func (m *wsMockWorkSessionRepository) CloseWorkSession(ctx context.Context, id int64, at time.Time, auto bool) (bool, error) {
	return m.CloseSession(ctx, id, at, auto)
}
func (m *wsMockWorkSessionRepository) SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (int64, error) {
	return 1, m.UpdateBreakMinutes(ctx, id, minutes)
}
func (m *wsMockWorkSessionRepository) CountWorkSessions(ctx context.Context, filter workforce.WorkSessionFilter) (int, error) {
	return m.CountWithFilter(ctx, filter)
}
func (m *wsMockWorkSessionRepository) OldestWorkSessionDate(ctx context.Context, column, before string) (string, error) {
	date, err := m.OldestBefore(ctx, column, optionalSessionDate(before))
	if date == nil {
		return "", err
	}
	return date.String(), err
}
func (m *wsMockWorkSessionRepository) DeleteWorkSessionsOlderThan(ctx context.Context, column, before string) (int64, error) {
	return m.DeleteOlderThan(ctx, column, Date(before))
}

func (m *absWorkSessionRepoMock) FindWorkSession(ctx context.Context, id int64) (workforce.WorkSession, error) {
	return nativeTestSession(m.FindByID(ctx, id))
}

func (m *absWorkSessionRepoMock) TodayOpenWorkSession(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	return nativeTestSession(m.GetCurrentByStaffID(ctx, staffID))
}

func (m *absWorkSessionRepoMock) LatestOpenWorkSession(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	return nativeTestSession(m.GetLatestOpenByStaffID(ctx, staffID))
}

func (m *absWorkSessionRepoMock) OpenWorkSessionOn(ctx context.Context, staffID int64, date string) (workforce.WorkSession, error) {
	return nativeTestSession(m.GetOpenByStaffAndDate(ctx, staffID, Date(date)))
}

func (m *absWorkSessionRepoMock) LockOpenWorkSessionOn(ctx context.Context, staffID int64, date string) (workforce.WorkSession, error) {
	return nativeTestSession(m.GetOpenByStaffAndDateForUpdate(ctx, staffID, Date(date)))
}

func (m *absWorkSessionRepoMock) LockOpenWorkSession(ctx context.Context, id int64) (workforce.WorkSession, error) {
	return nativeTestSession(m.LockOpenByIDForUpdate(ctx, id))
}

func (m *absWorkSessionRepoMock) CreateWorkSession(ctx context.Context, value workforce.WorkSession) (workforce.WorkSession, error) {
	record := workSessionToRecord(value)
	err := m.Create(ctx, record)
	return WorkSessionToCapability(record), err
}

func (m *absWorkSessionRepoMock) UpdateWorkSession(ctx context.Context, value workforce.WorkSession) (workforce.WorkSession, error) {
	record := workSessionToRecord(value)
	err := m.Update(ctx, record)
	return WorkSessionToCapability(record), err
}

func (m *absWorkSessionRepoMock) ListWorkSessions(ctx context.Context, filter workforce.WorkSessionFilter) ([]workforce.WorkSession, error) {
	if filter.StaffIDs != nil && (filter.DateFrom != "" || filter.DateTo != "") {
		grouped, err := m.GetHistoryByStaffIDs(ctx, filter.StaffIDs, Date(filter.DateFrom), Date(filter.DateTo))
		result := make([]workforce.WorkSession, 0)
		for _, id := range filter.StaffIDs {
			for _, value := range grouped[id] {
				result = append(result, WorkSessionToCapability(value))
			}
		}
		return result, err
	}
	if filter.StaffID != 0 && (filter.DateFrom != "" || filter.DateTo != "") {
		return nativeTestSessions(m.GetHistoryByStaffID(ctx, filter.StaffID, Date(filter.DateFrom), Date(filter.DateTo)))
	}
	if filter.Open != nil && *filter.Open && filter.DateBefore != "" {
		return nativeTestSessions(m.GetOpenSessions(ctx, Date(filter.DateBefore)))
	}
	return nativeTestSessions(m.ListWithFilter(ctx, filter))
}
func (m *absWorkSessionRepoMock) ListOverlappingWorkSessions(ctx context.Context, ids []int64, from time.Time, to *time.Time) ([]workforce.WorkSession, error) {
	if len(ids) == 1 {
		return nativeTestSessions(m.ListOverlappingByStaffID(ctx, ids[0], from, to))
	}
	grouped, err := m.ListOverlappingByStaffIDs(ctx, ids, from, to)
	result := make([]workforce.WorkSession, 0)
	for _, id := range ids {
		for _, value := range grouped[id] {
			result = append(result, WorkSessionToCapability(value))
		}
	}
	return result, err
}
func (m *absWorkSessionRepoMock) WorkPresenceMap(ctx context.Context) (map[int64]string, error) {
	return m.GetTodayPresenceMap(ctx)
}
func (m *absWorkSessionRepoMock) CloseWorkSession(ctx context.Context, id int64, at time.Time, auto bool) (bool, error) {
	return m.CloseSession(ctx, id, at, auto)
}
func (m *absWorkSessionRepoMock) SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (int64, error) {
	return 1, m.UpdateBreakMinutes(ctx, id, minutes)
}
func (m *absWorkSessionRepoMock) CountWorkSessions(ctx context.Context, filter workforce.WorkSessionFilter) (int, error) {
	return m.CountWithFilter(ctx, filter)
}
func (m *absWorkSessionRepoMock) OldestWorkSessionDate(ctx context.Context, column, before string) (string, error) {
	date, err := m.OldestBefore(ctx, column, optionalSessionDate(before))
	if date == nil {
		return "", err
	}
	return date.String(), err
}
func (m *absWorkSessionRepoMock) DeleteWorkSessionsOlderThan(ctx context.Context, column, before string) (int64, error) {
	return m.DeleteOlderThan(ctx, column, Date(before))
}

func (m *wsMockWorkSessionBreakRepository) ListWorkSessionBreaks(ctx context.Context, filter workforce.WorkSessionBreakFilter) ([]workforce.WorkSessionBreak, error) {
	if filter.Active != nil && *filter.Active {
		value, err := m.GetActiveBySessionID(ctx, filter.SessionID)
		if value == nil {
			return nil, err
		}
		return []workforce.WorkSessionBreak{WorkSessionBreakToCapability(value)}, err
	}
	if filter.SessionIDs != nil {
		grouped, err := m.GetBySessionIDs(ctx, filter.SessionIDs)
		result := make([]workforce.WorkSessionBreak, 0)
		for _, id := range filter.SessionIDs {
			for _, value := range grouped[id] {
				result = append(result, WorkSessionBreakToCapability(value))
			}
		}
		return result, err
	}
	return nativeTestBreaks(m.GetBySessionID(ctx, filter.SessionID))
}
func (m *wsMockWorkSessionBreakRepository) ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) ([]workforce.WorkSessionBreak, error) {
	return nativeTestBreaks(m.GetExpiredBreaks(ctx, before))
}
func (m *wsMockWorkSessionBreakRepository) CreateWorkSessionBreak(ctx context.Context, value workforce.WorkSessionBreak) (workforce.WorkSessionBreak, error) {
	record := workSessionBreakToRecord(value)
	err := m.Create(ctx, record)
	return WorkSessionBreakToCapability(record), err
}
func (m *wsMockWorkSessionBreakRepository) EndWorkSessionBreak(ctx context.Context, id int64, at time.Time, minutes int) (int64, error) {
	return 1, m.EndBreak(ctx, id, at, minutes)
}
func (m *wsMockWorkSessionBreakRepository) SetWorkSessionBreakDuration(ctx context.Context, id int64, minutes int, at time.Time) (int64, error) {
	return 1, m.UpdateDuration(ctx, id, minutes, at)
}
func (m *wtmMockSessionReader) ListOverlappingWorkSessions(ctx context.Context, ids []int64, from time.Time, to *time.Time) ([]workforce.WorkSession, error) {
	return nativeTestSessions(m.ListOverlappingByStaffID(ctx, ids[0], from, to))
}
func (m *wtmMockBreakReader) ListWorkSessionBreaks(ctx context.Context, filter workforce.WorkSessionBreakFilter) ([]workforce.WorkSessionBreak, error) {
	grouped, err := m.GetBySessionIDs(ctx, filter.SessionIDs)
	result := make([]workforce.WorkSessionBreak, 0)
	for _, id := range filter.SessionIDs {
		for _, value := range grouped[id] {
			result = append(result, WorkSessionBreakToCapability(value))
		}
	}
	return result, err
}

func optionalSessionDate(value string) *calendar.Date {
	if value == "" {
		return nil
	}
	date := calendar.Date(value)
	return &date
}
