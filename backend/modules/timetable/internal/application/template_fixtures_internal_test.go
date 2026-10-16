package application

import (
	"strings"
	"sync"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// testSchoolClassRules mirrors School Structure's supported grade range and
// class-name identity, which the composition root binds in production.
var testSchoolClassRules = ports.SchoolClassRules{
	MinGradeLevel: 1,
	MaxGradeLevel: 13,
	Normalize: func(class string) string {
		return strings.ToLower(strings.TrimSpace(class))
	},
}

type staffingAnnouncement struct {
	tenantID int64
	source   string
}

// recordingStaffingAnnouncer records every staffing announcement a template
// write makes after its commit.
type recordingStaffingAnnouncer struct {
	mu    sync.Mutex
	calls []staffingAnnouncement
}

func (a *recordingStaffingAnnouncer) AnnounceStaffingChanged(tenantID int64, source string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, staffingAnnouncement{tenantID: tenantID, source: source})
	return nil
}
