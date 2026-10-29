package domain

import (
	"fmt"
	"sort"
	"testing"
)

func chainIDs(sessions []AccountSession) []int64 {
	ids := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func link(id int64, token, next, family string) AccountSession {
	session := AccountSession{ID: id, TokenHash: token, FamilyID: family}
	if next != "" {
		session.ReplacementTokenHash = &next
	}
	return session
}

func TestSessionChain(t *testing.T) {
	t.Parallel()
	sessions := []AccountSession{
		// A chain whose oldest rows the backfill could not place.
		link(1, "a", "b", ""),
		link(2, "b", "c", ""),
		link(3, "c", "d", "family-1"),
		link(4, "d", "", "family-1"),
		// Another family and a stray row of the account stay.
		link(5, "x", "", "family-2"),
		link(6, "y", "", ""),
		// A cycle ends the walk.
		link(7, "p", "q", ""),
		link(8, "q", "p", ""),
	}
	cases := []struct {
		start AccountSession
		want  []int64
	}{
		{sessions[0], []int64{1, 2, 3, 4}},
		{sessions[1], []int64{1, 2, 3, 4}},
		{sessions[3], []int64{1, 2, 3, 4}},
		{sessions[4], []int64{5}},
		{sessions[5], []int64{6}},
		{sessions[6], []int64{7, 8}},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("from %d", tc.start.ID), func(t *testing.T) {
			t.Parallel()
			if got := chainIDs(SessionChain(tc.start, sessions)); fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("SessionChain() = %v, want %v", got, tc.want)
			}
		})
	}
}
