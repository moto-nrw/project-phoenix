package timetracking

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// WorkSessionRecords is the native work-block capability used by the clock.
// It carries typed identities and command results, not a generic row repository.
type WorkSessionRecords interface {
	FindWorkSession(context.Context, int64) (workforce.WorkSession, error)
	TodayOpenWorkSession(context.Context, int64) (workforce.WorkSession, error)
	LatestOpenWorkSession(context.Context, int64) (workforce.WorkSession, error)
	OpenWorkSessionOn(context.Context, int64, string) (workforce.WorkSession, error)
	LockOpenWorkSessionOn(context.Context, int64, string) (workforce.WorkSession, error)
	LockOpenWorkSession(context.Context, int64) (workforce.WorkSession, error)
	ListWorkSessions(context.Context, workforce.WorkSessionFilter) ([]workforce.WorkSession, error)
	ListOverlappingWorkSessions(context.Context, []int64, time.Time, *time.Time) ([]workforce.WorkSession, error)
	CreateWorkSession(context.Context, workforce.WorkSession) (workforce.WorkSession, error)
	UpdateWorkSession(context.Context, workforce.WorkSession) (workforce.WorkSession, error)
	LockStaffBalanceWrites(context.Context, int64) error
	WorkPresenceMap(context.Context) (map[int64]string, error)
	CloseWorkSession(context.Context, int64, time.Time, bool) (bool, error)
	SetWorkSessionBreakMinutes(context.Context, int64, int) (int64, error)
	CountWorkSessions(context.Context, workforce.WorkSessionFilter) (int, error)
	OldestWorkSessionDate(context.Context, string, string) (string, error)
	DeleteWorkSessionsOlderThan(context.Context, string, string) (int64, error)
}

// AbsenceRecords is the native Workforce absence command and query seam.
type AbsenceRecords interface {
	FindStaffAbsence(context.Context, int64) (workforce.StaffAbsence, error)
	CreateStaffAbsence(context.Context, *workforce.StaffAbsence) (workforce.StaffAbsence, error)
	UpdateStaffAbsence(context.Context, workforce.StaffAbsence) (workforce.StaffAbsence, error)
	DeleteStaffAbsence(context.Context, int64) error
	ListStaffAbsences(context.Context, workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error)
	CountStaffAbsences(context.Context, workforce.StaffAbsenceFilter) (int, error)
	OldestStaffAbsenceDate(context.Context, string, string) (string, error)
	DeleteStaffAbsencesOlderThan(context.Context, string, string) (int64, error)
	LockStaffAbsenceWrites(context.Context, int64) error
	StaffAbsenceMapForDate(context.Context, string) (map[int64]string, error)
	StaffAbsenceTypeIDMapForDate(context.Context, string) (map[int64]int64, error)
}

// AbsenceRequestRecords adds the cross-owner request enrichment the root binds.
type AbsenceRequestRecords interface {
	AbsenceRecords
	ListRequests(context.Context, AbsenceRequestFilter) ([]*AbsenceRequestRow, error)
}

type AbsenceAuditRecords interface {
	RecordStaffAbsenceAudit(context.Context, workforce.StaffAbsenceAudit) (workforce.StaffAbsenceAudit, error)
}

type BalanceAdjustmentRecords interface {
	FindStaffBalanceAdjustment(context.Context, int64) (workforce.StaffBalanceAdjustment, error)
	CreateStaffBalanceAdjustment(context.Context, workforce.StaffBalanceAdjustment) (workforce.StaffBalanceAdjustment, error)
	DeleteStaffBalanceAdjustment(context.Context, int64) error
	ListStaffBalanceAdjustments(context.Context, workforce.StaffBalanceAdjustmentFilter) ([]workforce.StaffBalanceAdjustment, error)
	LockStaffBalanceWrites(context.Context, int64) error
}

type VacationQuotaRecords interface {
	ListStaffVacationQuotas(context.Context, workforce.StaffVacationFilter) ([]workforce.StaffVacationQuota, error)
	UpsertStaffVacationQuota(context.Context, workforce.StaffVacationQuota) error
}

type VacationOpeningRecords interface {
	ListStaffVacationOpenings(context.Context, workforce.StaffVacationFilter) ([]workforce.StaffVacationOpening, error)
	CreateStaffVacationOpening(context.Context, workforce.StaffVacationOpening) (workforce.StaffVacationOpening, error)
	DeleteStaffVacationOpening(context.Context, int64) error
}

// WorkSessionBreakRecords is the native break capability used by the clock.
type WorkSessionBreakRecords interface {
	ListWorkSessionBreaks(context.Context, workforce.WorkSessionBreakFilter) ([]workforce.WorkSessionBreak, error)
	ExpiredWorkSessionBreaks(context.Context, time.Time) ([]workforce.WorkSessionBreak, error)
	CreateWorkSessionBreak(context.Context, workforce.WorkSessionBreak) (workforce.WorkSessionBreak, error)
	EndWorkSessionBreak(context.Context, int64, time.Time, int) (int64, error)
	SetWorkSessionBreakDuration(context.Context, int64, int, time.Time) (int64, error)
}
