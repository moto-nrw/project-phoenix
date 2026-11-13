package peopledirectory

import "context"

// StudentRowLocks holds only the owned student rows, without loading foreign
// student facts. Missing students are refused before the caller can write.
type StudentRowLocks interface {
	LockStudentRows(context.Context, []int64) (int, error)
}
