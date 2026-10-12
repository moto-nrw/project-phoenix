package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// SetExternalID stores the key another system, such as IServ, knows the child
// by (#1446). One key names one child of a school: a graduate that still holds
// it gives it up to the child now known by it, an active child keeps it and
// the write is refused. A child that already holds the key is left alone.
func (s *StudentService) SetExternalID(ctx context.Context, id int64, key string) error {
	key = strings.TrimSpace(key)
	return s.run(ctx, "set_student_external_id", s.runStudentWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		done, err := s.releaseExternalID(txCtx, id, key, stats)
		if err != nil || done {
			return err
		}
		updated, setStats, err := s.store.SetExternalID(txCtx, id, &key)
		stats.Add(setStats)
		if err != nil {
			return err
		}
		if !updated {
			return domain.ErrStudentNotFound
		}
		return nil
	})
}

// releaseExternalID frees key for the child id. done is true when the child
// already holds it, so nothing is left to write.
func (s *StudentService) releaseExternalID(ctx context.Context, id int64, key string, stats *domain.OperationStats) (done bool, err error) {
	holder, held, findStats, err := s.store.FindExternalIDHolder(ctx, key)
	stats.Add(findStats)
	if err != nil || !held {
		return false, err
	}
	if holder.ID == id {
		return true, nil
	}
	if holder.Status != domain.StudentStatusAlumnus {
		return false, domain.ErrStudentExternalIDTaken
	}
	_, releaseStats, err := s.store.SetExternalID(ctx, holder.ID, nil)
	stats.Add(releaseStats)
	return false, err
}
