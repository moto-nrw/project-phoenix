package authorize

import "testing"

func TestDatabaseStatsCapabilitiesUseCanonicalPermissions(t *testing.T) {
	t.Parallel()

	users := DatabaseStatsCapabilities{students: true, teachers: true}
	roles := DatabaseStatsCapabilities{roles: true, permissionCatalog: true}
	all := DatabaseStatsCapabilities{
		students: true, teachers: true, rooms: true, activities: true, groups: true,
		roles: true, devices: true, permissionCatalog: true, timetables: true, gradeTransitions: true,
	}
	tests := []struct {
		name     string
		claims   []string
		expected DatabaseStatsCapabilities
	}{
		{name: "none"},
		{name: "users read", claims: []string{"users:read"}, expected: users},
		{name: "users list", claims: []string{"users:list"}, expected: users},
		{name: "rooms read", claims: []string{"rooms:read"}, expected: DatabaseStatsCapabilities{rooms: true}},
		{name: "rooms list", claims: []string{"rooms:list"}, expected: DatabaseStatsCapabilities{rooms: true}},
		{name: "activities read", claims: []string{"activities:read"}, expected: DatabaseStatsCapabilities{activities: true}},
		{name: "activities list", claims: []string{"activities:list"}, expected: DatabaseStatsCapabilities{activities: true}},
		{name: "groups read", claims: []string{"groups:read"}, expected: DatabaseStatsCapabilities{groups: true}},
		{name: "groups list", claims: []string{"groups:list"}, expected: DatabaseStatsCapabilities{groups: true}},
		{name: "auth manage", claims: []string{"auth:manage"}, expected: roles},
		{name: "iot read", claims: []string{"iot:read"}, expected: DatabaseStatsCapabilities{devices: true}},
		{name: "iot manage", claims: []string{"iot:manage"}, expected: DatabaseStatsCapabilities{devices: true}},
		{name: "schedules read", claims: []string{"schedules:read"}, expected: DatabaseStatsCapabilities{timetables: true}},
		{name: "schedules list", claims: []string{"schedules:list"}, expected: DatabaseStatsCapabilities{timetables: true}},
		{name: "grade transitions", claims: []string{"grade_transitions:read"}, expected: DatabaseStatsCapabilities{gradeTransitions: true}},
		{name: "admin wildcard", claims: []string{"admin:*"}, expected: all},
		{name: "full access", claims: []string{"*:*"}, expected: all},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := NewDatabaseStatsCapabilities(test.claims); actual != test.expected {
				t.Fatalf("capabilities = %+v, want %+v", actual, test.expected)
			}
		})
	}
}
