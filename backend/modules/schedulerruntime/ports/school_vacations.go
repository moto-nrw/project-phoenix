package ports

import "context"

// SchoolVacationSync renews the stored school vacations of every federal
// state from the external calendar (#5345) and reports how many states and
// vacations it stored. It runs once for all schools; a failed fetch keeps
// the stored vacations. The root binds the School Calendar.
type SchoolVacationSync func(ctx context.Context) (regions, vacations int, err error)
