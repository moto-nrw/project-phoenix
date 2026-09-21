package postgres_test

import (
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// sessionGroups returns the session repository behind the composed session
// records of a repository factory.
func sessionGroups(records studentpresence.SessionRecords) ports.ActiveGroupRepository {
	groups, _ := presenceCompose.SessionRepositories(records)
	return groups
}

// sessionSupervisors returns the supervision repository behind the composed
// session records of a repository factory.
func sessionSupervisors(records studentpresence.SessionRecords) ports.GroupSupervisorRepository {
	_, supervisors := presenceCompose.SessionRepositories(records)
	return supervisors
}

// supervisorOf copies a stored fixture supervision into the value the
// repository reads and writes.
func supervisorOf(row *testpkg.GroupSupervisorRow) *ports.GroupSupervisor {
	return &ports.GroupSupervisor{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID,
		StaffID: row.StaffID, GroupID: row.GroupID, Role: row.Role, StartDate: row.StartDate, EndDate: row.EndDate,
	}
}
