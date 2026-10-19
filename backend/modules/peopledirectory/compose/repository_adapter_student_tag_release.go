package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentTagReleaser frees the bracelets of students through the People
// Directory: the person rows belong to that owner, the student rows only
// supply the person ids. The care lifecycle uses it when a child's care ends
// (#2711 moved the grade-transition release into its workflow).
type StudentTagReleaser struct {
	peopledirectory.StudentTagRelease
}

func NewStudentTagReleaser(people peopledirectory.Capability) StudentTagReleaser {
	return StudentTagReleaser{StudentTagRelease: people}
}

// ReleaseStudentTags frees the bracelets of children whose care ended and
// reports how many it released: the care lifecycle's tag port.
func (r StudentTagReleaser) ReleaseStudentTags(ctx context.Context, studentIDs []int64) (int, error) {
	released, err := r.ReleaseStudentTagsByIDs(ctx, studentIDs)
	return len(released), err
}
