package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
)

// int64Ptr returns a pointer to the given int64.
// (strPtr is already defined in pickup_schedule_bind_test.go for this package.)
func int64Ptr(v int64) *int64 { return &v }

func TestApplyStammraumOverride_StudentInOwnStammraum_SetsGreen(t *testing.T) {
	resp := &StudentResponse{BadgeColor: strPtr("#06B6D4")}
	info := common.StudentLocationInfo{
		RoomID:     int64Ptr(5),
		BadgeColor: strPtr("#06B6D4"),
	}
	group := &educationModel.Group{RoomID: int64Ptr(5)}

	applyStammraumOverride(resp, info, group)

	if resp.BadgeColor == nil {
		t.Fatalf("expected BadgeColor to be set, got nil")
	}
	if *resp.BadgeColor != common.BadgeStammraumGreen {
		t.Errorf("expected BadgeColor=%q (Stammraum green), got %q", common.BadgeStammraumGreen, *resp.BadgeColor)
	}
}

func TestApplyStammraumOverride_StudentInDifferentRoom_PreservesRoomColor(t *testing.T) {
	roomColor := "#06B6D4"
	resp := &StudentResponse{BadgeColor: &roomColor}
	info := common.StudentLocationInfo{
		RoomID:     int64Ptr(7),
		BadgeColor: &roomColor,
	}
	group := &educationModel.Group{RoomID: int64Ptr(5)}

	applyStammraumOverride(resp, info, group)

	if resp.BadgeColor == nil || *resp.BadgeColor != roomColor {
		t.Errorf("expected BadgeColor to stay %q, got %v", roomColor, resp.BadgeColor)
	}
}

func TestApplyStammraumOverride_NilGroup_NoOverride(t *testing.T) {
	roomColor := "#06B6D4"
	resp := &StudentResponse{BadgeColor: &roomColor}
	info := common.StudentLocationInfo{
		RoomID:     int64Ptr(5),
		BadgeColor: &roomColor,
	}

	applyStammraumOverride(resp, info, nil)

	if resp.BadgeColor == nil || *resp.BadgeColor != roomColor {
		t.Errorf("expected BadgeColor to stay %q, got %v", roomColor, resp.BadgeColor)
	}
}

func TestApplyStammraumOverride_GroupWithoutStammraum_NoOverride(t *testing.T) {
	roomColor := "#06B6D4"
	resp := &StudentResponse{BadgeColor: &roomColor}
	info := common.StudentLocationInfo{
		RoomID:     int64Ptr(5),
		BadgeColor: &roomColor,
	}
	group := &educationModel.Group{RoomID: nil}

	applyStammraumOverride(resp, info, group)

	if resp.BadgeColor == nil || *resp.BadgeColor != roomColor {
		t.Errorf("expected BadgeColor to stay %q, got %v", roomColor, resp.BadgeColor)
	}
}

func TestApplyStammraumOverride_StudentNotInRoom_NoOverride(t *testing.T) {
	resp := &StudentResponse{BadgeColor: nil}
	info := common.StudentLocationInfo{RoomID: nil}
	group := &educationModel.Group{RoomID: int64Ptr(5)}

	applyStammraumOverride(resp, info, group)

	if resp.BadgeColor != nil {
		t.Errorf("expected BadgeColor to stay nil, got %q", *resp.BadgeColor)
	}
}

func TestApplyStammraumOverride_NilBadgeColorAndStammraumMatch_StillSetsGreen(t *testing.T) {
	// Simulates a stammraum that has no custom color set: backend sends nil
	// from the resolver, but stammraum match still wins.
	resp := &StudentResponse{BadgeColor: nil}
	info := common.StudentLocationInfo{
		RoomID:     int64Ptr(5),
		BadgeColor: nil,
	}
	group := &educationModel.Group{RoomID: int64Ptr(5)}

	applyStammraumOverride(resp, info, group)

	if resp.BadgeColor == nil {
		t.Fatalf("expected BadgeColor to be green, got nil")
	}
	if *resp.BadgeColor != common.BadgeStammraumGreen {
		t.Errorf("expected BadgeColor=%q, got %q", common.BadgeStammraumGreen, *resp.BadgeColor)
	}
}
