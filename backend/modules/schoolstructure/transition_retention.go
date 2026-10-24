package schoolstructure

import (
	"context"
	"errors"
)

// GradeTransitionRetention deletes a school's grade transitions once its
// window for children after their last care day has passed (#4045): their
// history names children, with their transponders and the class-list names
// they moved. An applied transition counts from its application, a reverted
// one from its reversal and a draft from its last change. A deleted
// transition can no longer be reverted.
type GradeTransitionRetention interface {
	DeleteExpiredGradeTransitions(ctx context.Context, retentionDays int) (int, error)
}

// ErrGradeTransitionRetentionWindow refuses a retention window that is not
// positive: nothing is deleted on a wrong period, and the run is caught up
// once the setting is fixed.
var ErrGradeTransitionRetentionWindow = errors.New("grade transition retention window must be positive")
