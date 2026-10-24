package supervisiondashboard

import "sort"

// SortSessionsByRoom puts sessions into the order SessionDirectory delivers
// them in: by room name under compare, the German dictionary order the
// composition names. Sessions in equally named rooms keep their read order.
func SortSessionsByRoom(sessions []Session, compare func(a, b string) int) {
	sort.SliceStable(sessions, func(i, j int) bool {
		return compare(sessions[i].RoomName, sessions[j].RoomName) < 0
	})
}
