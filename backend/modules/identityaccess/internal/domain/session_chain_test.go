package domain

import (
	"errors"
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

func link(id int64, generation int, token, next, family string) AccountSession {
	session := AccountSession{ID: id, AccountID: 7, Generation: generation, TokenHash: token, FamilyID: family}
	if next != "" {
		session.ReplacementTokenHash = &next
	}
	return session
}

func TestSessionChain(t *testing.T) {
	t.Parallel()
	sessions := []AccountSession{
		// A chain whose oldest rows the backfill could not place.
		link(1, 0, "a", "b", ""),
		link(2, 1, "b", "c", ""),
		link(3, 2, "c", "d", "family-1"),
		link(4, 3, "d", "", "family-1"),
		// Another family and a stray row of the account stay.
		link(5, 0, "x", "", "family-2"),
		link(6, 0, "y", "", ""),
		// A cycle breaks the generation order, so it is no lineage.
		link(7, 0, "p", "q", ""),
		link(8, 1, "q", "p", ""),
		// A link across a generation gap is no lineage.
		link(9, 0, "g", "h", ""),
		link(10, 2, "h", "", "family-3"),
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
		{sessions[8], []int64{9}},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("from %d", tc.start.ID), func(t *testing.T) {
			t.Parallel()
			chain, err := SessionChain(tc.start, sessions)
			if err != nil {
				t.Fatalf("SessionChain() error = %v", err)
			}
			if got := chainIDs(chain); fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("SessionChain() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSessionChainCollidingToken pins that a colliding token hash never
// pulls an independent family into a logout, and that an unresolved
// ambiguity fails the logout instead of reporting success.
func TestSessionChainCollidingToken(t *testing.T) {
	t.Parallel()
	own := []AccountSession{
		link(1, 0, "own-0", "shared", "family-own"),
		link(2, 1, "shared", "", "family-own"),
	}
	other := link(3, 0, "shared", "", "family-other")
	chain, err := SessionChain(own[0], append(append([]AccountSession{}, own...), other))
	if err != nil {
		t.Fatalf("SessionChain() error = %v", err)
	}
	if got := chainIDs(chain); fmt.Sprint(got) != "[1 2]" {
		t.Fatalf("SessionChain() = %v, want the own family only", got)
	}

	// Without a family, the own continuation cannot be told apart.
	legacy := link(4, 0, "legacy-0", "twin", "")
	twins := []AccountSession{legacy, link(5, 1, "twin", "", "family-a"), link(6, 1, "twin", "", "family-b")}
	if _, err := SessionChain(legacy, twins); !errors.Is(err, ErrSessionChainAmbiguous) {
		t.Fatalf("SessionChain() error = %v, want ErrSessionChainAmbiguous", err)
	}
}

// TestSessionChainFollowsSwitchLineage pins #5422: a logout reaches every
// family a switch minted from the chain and the family the chain was
// switched from, in both directions and transitively, but not another
// login of the account nor a family of another account that names it.
func TestSessionChainFollowsSwitchLineage(t *testing.T) {
	t.Parallel()
	switched := func(id int64, account int64, family, from string) AccountSession {
		return AccountSession{ID: id, AccountID: account, TokenHash: fmt.Sprintf("t%d", id), FamilyID: family, SwitchedFromFamilyID: from}
	}
	sessions := []AccountSession{
		switched(1, 7, "login", ""),
		switched(2, 7, "first", "login"),
		switched(3, 7, "first", "login"),
		switched(4, 7, "second", "login"),
		switched(5, 7, "third", "second"),
		switched(6, 7, "other-login", ""),
		switched(7, 8, "foreign", "login"),
	}
	lineage := []int64{1, 2, 3, 4, 5}
	for _, start := range []AccountSession{sessions[0], sessions[1], sessions[4]} {
		chain, err := SessionChain(start, sessions)
		if err != nil {
			t.Fatalf("from %d: %v", start.ID, err)
		}
		if got := chainIDs(chain); fmt.Sprint(got) != fmt.Sprint(lineage) {
			t.Fatalf("from %d: chain = %v, want %v", start.ID, got, lineage)
		}
	}
	chain, err := SessionChain(sessions[5], sessions)
	if err != nil {
		t.Fatal(err)
	}
	if got := chainIDs(chain); fmt.Sprint(got) != "[6]" {
		t.Fatalf("another login joins nothing: chain = %v", got)
	}
}
