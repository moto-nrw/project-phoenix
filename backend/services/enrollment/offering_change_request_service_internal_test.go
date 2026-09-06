package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestContiguousCarePeriodEnd_StopsBeforeGap(t *testing.T) {
	t.Parallel()

	initial := &StudentCarePeriod{
		ServiceStartDate: timezone.NewDate(2026, 8, 1),
		ServiceEndDate:   timezone.NewDate(2026, 8, 31),
	}
	consecutive := &StudentCarePeriod{
		ServiceStartDate: timezone.NewDate(2026, 9, 1),
		ServiceEndDate:   timezone.NewDate(2026, 9, 30),
	}
	afterGap := &StudentCarePeriod{
		ServiceStartDate: timezone.NewDate(2026, 10, 2),
		ServiceEndDate:   timezone.NewDate(2026, 10, 31),
	}

	latest := contiguousCarePeriodEnd(
		[]*StudentCarePeriod{afterGap, consecutive, initial},
		initial,
	)

	assert.Equal(t, consecutive.ServiceEndDate, latest)
}
