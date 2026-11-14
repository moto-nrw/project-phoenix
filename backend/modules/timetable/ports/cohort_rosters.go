package ports

import (
	"log/slog"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
)

// CohortRosterDependencies are the rows the cohort roster follow-up (#6841)
// reads and writes. CareBounds may be nil (no child has a last care day).
type CohortRosterDependencies struct {
	Groups       timetable.ActivityGroupQuery
	Students     StudentDirectory
	Enrollments  timetable.StudentEnrollmentQuery
	Directories  RepositoryDirectories
	CareBounds   CareBoundReader
	Instances    InstanceRows
	Participants ParticipantRows
	Logger       *slog.Logger
}
