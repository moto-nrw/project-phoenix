package active

import (
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The dashboard's OGS classification used to look a session's group_id up in a
// map keyed by education.groups.id. active.groups.group_id is a foreign key to
// activities.groups(id), and both tables draw from independent BIGSERIAL
// sequences — so that lookup matched by numeric coincidence only (#2178).
//
// These fixtures deliberately collide: education group 7 and activities
// template 7 exist side by side, exactly as they do in a freshly seeded
// instance where both sequences start at 1.
const (
	collidingID           int64 = 7  // both an education group and an unrelated AG
	careTemplateID        int64 = 44 // Betreuungsgruppe: care + education binding
	careWithoutGroupID    int64 = 45 // care block serving no specific group
	activityWithTargetID  int64 = 46 // AG whose Zielgruppe is a group
	unknownTemplateID     int64 = 99 // no template row loaded
	collidingEducationRef int64 = 7  // the education group the care template serves
)

func templateFixtures() map[int64]*activitiesModels.Group {
	educationRef := collidingEducationRef

	collidingActivity := &activitiesModels.Group{
		Name:            "Fußball",
		Type:            activitiesModels.GroupTypeActivity,
		TargetGroupType: activitiesModels.TargetGroupTypeNone,
	}
	collidingActivity.ID = collidingID

	careGroup := &activitiesModels.Group{
		Name:             "Gruppenzeit Bären",
		Type:             activitiesModels.GroupTypeCare,
		TargetGroupType:  activitiesModels.TargetGroupTypeGruppe,
		EducationGroupID: &educationRef,
	}
	careGroup.ID = careTemplateID

	careWithoutGroup := &activitiesModels.Group{
		Name:            "Mittagessen 1. Schicht",
		Type:            activitiesModels.GroupTypeCare,
		TargetGroupType: activitiesModels.TargetGroupTypeNone,
	}
	careWithoutGroup.ID = careWithoutGroupID

	activityWithTarget := &activitiesModels.Group{
		Name:             "Basteln für die Bären",
		Type:             activitiesModels.GroupTypeActivity,
		TargetGroupType:  activitiesModels.TargetGroupTypeGruppe,
		EducationGroupID: &educationRef,
	}
	activityWithTarget.ID = activityWithTargetID

	return map[int64]*activitiesModels.Group{
		collidingID:          collidingActivity,
		careTemplateID:       careGroup,
		careWithoutGroupID:   careWithoutGroup,
		activityWithTargetID: activityWithTarget,
	}
}

func activeSession(id int64, templateID *int64, roomID int64) *activeModels.Group {
	session := &activeModels.Group{
		StartTime: time.Now().Add(-10 * time.Minute),
		GroupID:   templateID,
		RoomID:    roomID,
	}
	session.Model = modelBase.Model{ID: id}
	return session
}

func emptyRoomData() *dashboardRoomData {
	return &dashboardRoomData{
		roomByID:        map[int64]*facilityModels.Room{},
		occupiedRooms:   map[int64]bool{},
		roomStudentsMap: map[int64]map[int64]struct{}{},
	}
}

func TestIsOGSGroupTemplate(t *testing.T) {
	templates := templateFixtures()

	tests := []struct {
		name     string
		template *activitiesModels.Group
		want     bool
	}{
		{"care block bound to an education group", templates[careTemplateID], true},
		{"AG whose id collides with an education group id", templates[collidingID], false},
		{"AG with a group Zielgruppe is not a Betreuungsgruppe", templates[activityWithTargetID], false},
		{"care block without an education binding", templates[careWithoutGroupID], false},
		{"missing template", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isOGSGroupTemplate(tt.template))
		})
	}
}

func TestProcessActiveGroupsCountsOnlyEducationBoundCareSessions(t *testing.T) {
	templates := templateFixtures()
	colliding := collidingID
	care := careTemplateID
	careOpen := careWithoutGroupID
	targeted := activityWithTargetID
	unknown := unknownTemplateID

	sessions := []*activeModels.Group{
		activeSession(1001, &colliding, 501), // must NOT count despite the id collision
		activeSession(1002, &care, 502),      // the only real Betreuungsgruppe
		activeSession(1003, &careOpen, 503),
		activeSession(1004, &targeted, 504),
		activeSession(1005, &unknown, 505),
		activeSession(1006, nil, 506), // spontaneous session (WP-B6)
	}

	roomData := emptyRoomData()
	totalCount, ogsCount, _ := processActiveGroups(sessions, map[int64][]*activeModels.Visit{}, templates, roomData)

	assert.Equal(t, len(sessions), totalCount)
	assert.Equal(t, 1, ogsCount, "only the education-bound care session is a Betreuungsgruppe")
}

// A dashboard whose activities templates failed to load must report zero OGS
// groups rather than inventing them from colliding education ids.
func TestProcessActiveGroupsWithoutTemplatesCountsNoOGSGroups(t *testing.T) {
	colliding := collidingID
	sessions := []*activeModels.Group{activeSession(1001, &colliding, 501)}

	_, ogsCount, _ := processActiveGroups(
		sessions,
		map[int64][]*activeModels.Visit{},
		map[int64]*activitiesModels.Group{},
		emptyRoomData(),
	)

	assert.Equal(t, 0, ogsCount)
}

func TestResolveGroupNameAndTypeUsesTemplateNotEducationID(t *testing.T) {
	templates := templateFixtures()
	colliding := collidingID
	care := careTemplateID
	targeted := activityWithTargetID
	unknown := unknownTemplateID

	tests := []struct {
		name     string
		groupID  *int64
		wantName string
		wantType string
	}{
		{"colliding AG keeps its own name and type", &colliding, "Fußball", "activity"},
		{"care session is an OGS group", &care, "Gruppenzeit Bären", "ogs_group"},
		{"AG with group Zielgruppe stays an activity", &targeted, "Basteln für die Bären", "activity"},
		{"unknown template falls back", &unknown, "Gruppe 99", "activity"},
		{"spontaneous session", nil, "Spontane Aktivität", "spontaneous"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotType := resolveGroupNameAndType(tt.groupID, templates)
			assert.Equal(t, tt.wantName, gotName)
			assert.Equal(t, tt.wantType, gotType)

			assert.Equal(t, tt.wantName, resolveGroupName(tt.groupID, templates),
				"both resolvers must agree on the display name")
		})
	}
}

func TestBuildActiveGroupsSummaryLabelsCollidingSessionAsActivity(t *testing.T) {
	templates := templateFixtures()
	colliding := collidingID
	care := careTemplateID

	sessions := []*activeModels.Group{
		activeSession(1001, &colliding, 501),
		activeSession(1002, &care, 502),
	}

	summary := buildActiveGroupsSummary(sessions, templates, emptyRoomData())
	require.Len(t, summary, 2)

	assert.Equal(t, "Fußball", summary[0].Name)
	assert.Equal(t, "activity", summary[0].Type)
	assert.Equal(t, "Gruppenzeit Bären", summary[1].Name)
	assert.Equal(t, "ogs_group", summary[1].Type)
}
