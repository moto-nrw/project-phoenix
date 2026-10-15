package enrollment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func capacityBooking(childID, offeringID int64, from, until string) OfferingBooking {
	booking := OfferingBooking{RequestChildID: childID, CareOfferingID: offeringID}
	if from != "" {
		booking.ValidFrom = new(Date(from))
	}
	if until != "" {
		booking.ValidUntil = new(Date(until))
	}
	return booking
}

func TestOfferingCapacityPeaksCountsEachChildOnceInHalfOpenWindows(t *testing.T) {
	t.Parallel()
	children := map[int64]OfferingChildFacts{
		1: {ID: 1, Status: ChildStatusApproved},
		2: {ID: 2, Status: ChildStatusApproved},
		3: {ID: 3, Status: ChildStatusWaitlisted},
		4: {ID: 4, Status: ChildStatusRejected},
		5: {ID: 5, Status: ChildStatusWithdrawn},
		6: {ID: 6, Status: ChildStatusApproved},
	}
	bookings := []OfferingBooking{
		// Child 1 switches days at 2030-01-03: adjacent rows count once.
		capacityBooking(1, 10, "", "2030-01-03"),
		capacityBooking(1, 10, "2030-01-03", ""),
		// Child 2 ends exactly where child 3 starts: the end is exclusive.
		capacityBooking(2, 10, "", "2030-01-02"),
		capacityBooking(3, 10, "2030-01-02", ""),
		// Rejected and withdrawn children never occupy a place.
		capacityBooking(4, 10, "", ""),
		capacityBooking(5, 10, "", ""),
		// Child 6 is excluded by the caller; another offering is separate.
		capacityBooking(6, 10, "", ""),
		capacityBooking(1, 20, "2030-01-05", "2030-01-06"),
		// Outside the window entirely.
		capacityBooking(2, 20, "2030-02-01", ""),
	}
	peaks := offeringCapacityPeaks(bookings, children, []int64{6}, Date("2030-01-01"), Date("2030-01-10"))
	require.Equal(t, map[int64]int{10: 2, 20: 1}, peaks)

	unexcluded := offeringCapacityPeaks(bookings, children, nil, Date("2030-01-01"), Date("2030-01-10"))
	require.Equal(t, 3, unexcluded[10])
}

func TestOfferingGradeCountsGroupsChildrenByGradeWithUngradedLast(t *testing.T) {
	t.Parallel()
	children := map[int64]OfferingChildFacts{
		1: {ID: 1, Status: ChildStatusApproved, TargetGradeLevel: new(int16(2))},
		2: {ID: 2, Status: ChildStatusApproved, TargetGradeLevel: new(int16(1))},
		3: {ID: 3, Status: ChildStatusApproved},
		4: {ID: 4, Status: ChildStatusRejected, TargetGradeLevel: new(int16(1))},
		5: {ID: 5, Status: ChildStatusApproved, TargetGradeLevel: new(int16(1))},
	}
	bookings := []OfferingBooking{
		capacityBooking(1, 10, "", ""),
		capacityBooking(1, 10, "2030-01-03", ""),
		capacityBooking(2, 10, "", ""),
		capacityBooking(3, 10, "", ""),
		capacityBooking(4, 10, "", ""),
		// Starts at the exclusive end and ends at the inclusive start.
		capacityBooking(5, 10, "2030-01-10", ""),
		capacityBooking(5, 10, "", "2030-01-01"),
		capacityBooking(5, 5, "", ""),
	}
	counts := offeringGradeCounts(bookings, children, Date("2030-01-01"), Date("2030-01-10"))
	require.Len(t, counts, 4)
	require.Equal(t, int64(5), counts[0].CareOfferingID)
	require.Equal(t, int16(1), *counts[0].GradeLevel)
	require.Equal(t, 1, counts[0].Count)
	require.Equal(t, int64(10), counts[1].CareOfferingID)
	require.Equal(t, int16(1), *counts[1].GradeLevel)
	require.Equal(t, 1, counts[1].Count)
	require.Equal(t, int16(2), *counts[2].GradeLevel)
	require.Equal(t, 1, counts[2].Count, "two rows of one child count once")
	require.Nil(t, counts[3].GradeLevel)
	require.Equal(t, 1, counts[3].Count)
}

func TestFirstUpcomingIntervalReturnsOnlyTheEarliestFutureRows(t *testing.T) {
	t.Parallel()
	row := func(id int64, from string) *RequestChildOffering {
		value := &RequestChildOffering{ID: id}
		if from != "" {
			value.ValidFrom = new(Date(from))
		}
		return value
	}
	history := []*RequestChildOffering{
		row(1, ""), row(2, "2030-01-01"), row(3, "2030-02-01"), row(4, "2030-01-01"), row(5, "2029-12-01"),
	}
	upcoming := firstUpcomingInterval(history, Date("2029-12-15"))
	require.Len(t, upcoming, 2)
	require.Equal(t, int64(2), upcoming[0].ID)
	require.Equal(t, int64(4), upcoming[1].ID)
	require.Empty(t, firstUpcomingInterval(history, Date("2030-03-01")))
}
