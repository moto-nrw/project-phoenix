package peopledirectory

import "context"

// StudentTagRelease clears the tags of the selected tenant children.
type StudentTagRelease interface {
	ReleaseStudentTagsByIDs(context.Context, []int64) (map[int64]string, error)
}
