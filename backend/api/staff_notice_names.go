package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// staffNoticeNames binds the Tagesinformationen's acknowledger-name port
// (#2208) to the People Directory: account ids in, display names of the
// tenant's active persons out. The Timetable composition takes it as a plain
// function (#3418).
func staffNoticeNames(persons peopledirectory.Query) func(context.Context, []int64) (map[int64]string, error) {
	return func(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
		found, err := persons.ListPersonsByAccount(ctx, accountIDs)
		if err != nil {
			return nil, err
		}
		names := make(map[int64]string, len(found))
		for _, person := range found {
			if person.AccountID == nil {
				continue
			}
			names[*person.AccountID] = person.FullName()
		}
		return names, nil
	}
}
