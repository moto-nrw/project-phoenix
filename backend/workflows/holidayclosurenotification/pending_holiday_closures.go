package holidayclosurenotification

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// appendPendingNotices builds the office view and its approval scope from the shared plans.
func appendPendingNotices(pending *Pending, notices []schoolcalendar.HolidayClosureNotice, schools map[int64]string, planned map[int64]enrollment.HolidayClosureImpact) []string {
	var tokens []string
	for _, notice := range notices {
		item, impact, keep := traegerItem(notice, schools, planned)
		if !keep {
			continue
		}
		if notice.Status == schoolcalendar.HolidayClosureSuggested {
			pending.Weeks = append(pending.Weeks, impact.Weeks...)
			pending.Children += impact.Children
			pending.Families += impact.Families
			tokens = append(tokens, fmt.Sprintf("%d:%d:%s", notice.ProposalID, notice.Revision, impact.Token))
		}
		pending.Notices = append(pending.Notices, item)
	}
	return tokens
}
