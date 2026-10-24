package devicescan

import "context"

type SupervisionQuery interface {
	ActiveSupervisions(context.Context, int64) ([]Supervision, error)
}

type Supervision struct {
	StaffID int64
	Role    string
	Ended   bool
}
