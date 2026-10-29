package domain

import "errors"

// ErrSessionChainAmbiguous refuses a logout whose chain continues through a
// link that names more than one session: the live continuation of the
// chain cannot be told apart, so the logout cannot confirm it revoked it.
var ErrSessionChainAmbiguous = errors.New("session chain continues ambiguously")

// SessionChain returns every session of the account that a logout of start
// must revoke (#5194): start, every session of its family, and the sessions
// linked to them by replacement_token, until nothing more joins. A row the
// family backfill could not place carries no family; following its links
// means no session of its chain survives the logout.
//
// A link counts only when it is unique: exactly one session has the token,
// on the same account and one generation apart, as the refresh hand-off
// requires. An ambiguous link joins nothing, so an independent family whose
// token collides stays valid. If the chain's own continuation is such a link
// and none of its candidates already belongs to the chain, the result is not
// complete and the logout must fail rather than report success. Each session
// joins once, so a cycle ends the walk. sessions are the account's sessions,
// start included.
func SessionChain(start AccountSession, sessions []AccountSession) ([]AccountSession, error) {
	index := newSessionIndex(sessions)
	chain := map[int64]bool{start.ID: true}
	queue := []AccountSession{start}
	var unresolved [][]AccountSession
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		linked, ambiguous := index.neighbours(current)
		if ambiguous != nil {
			unresolved = append(unresolved, ambiguous)
		}
		for _, next := range linked {
			if !chain[next.ID] {
				chain[next.ID] = true
				queue = append(queue, next)
			}
		}
	}
	for _, candidates := range unresolved {
		if !anyInChain(candidates, chain) {
			return nil, ErrSessionChainAmbiguous
		}
	}
	return inChain(start, sessions, chain), nil
}

// sessionIndex finds the sessions linked to one session.
type sessionIndex struct {
	byToken, byFamily, byReplacement map[string][]AccountSession
}

func newSessionIndex(sessions []AccountSession) sessionIndex {
	index := sessionIndex{
		byToken:       make(map[string][]AccountSession, len(sessions)),
		byFamily:      make(map[string][]AccountSession),
		byReplacement: make(map[string][]AccountSession),
	}
	for _, session := range sessions {
		index.byToken[session.TokenHash] = append(index.byToken[session.TokenHash], session)
		if session.FamilyID != "" {
			index.byFamily[session.FamilyID] = append(index.byFamily[session.FamilyID], session)
		}
		if session.ReplacementTokenHash != nil {
			index.byReplacement[*session.ReplacementTokenHash] = append(index.byReplacement[*session.ReplacementTokenHash], session)
		}
	}
	return index
}

// neighbours are the family members, the unique successor and the unique
// predecessor of a session. ambiguous holds the candidates of a successor
// link that names more than one session.
func (i sessionIndex) neighbours(session AccountSession) (linked, ambiguous []AccountSession) {
	if session.FamilyID != "" {
		linked = append(linked, i.byFamily[session.FamilyID]...)
	}
	if session.ReplacementTokenHash != nil {
		candidates := i.byToken[*session.ReplacementTokenHash]
		if len(candidates) > 1 {
			ambiguous = candidates
		} else if len(candidates) == 1 && lineage(session, candidates[0]) {
			linked = append(linked, candidates[0])
		}
	}
	if predecessors := i.byReplacement[session.TokenHash]; len(predecessors) == 1 && len(i.byToken[session.TokenHash]) == 1 && lineage(predecessors[0], session) {
		linked = append(linked, predecessors[0])
	}
	return linked, ambiguous
}

// lineage reports whether next continues current: same account, one
// generation on.
func lineage(current, next AccountSession) bool {
	return next.AccountID == current.AccountID && next.Generation == current.Generation+1
}

func anyInChain(candidates []AccountSession, chain map[int64]bool) bool {
	for _, candidate := range candidates {
		if chain[candidate.ID] {
			return true
		}
	}
	return false
}

// inChain lists the chain in the order of sessions; start comes along even
// when the listing missed it.
func inChain(start AccountSession, sessions []AccountSession, chain map[int64]bool) []AccountSession {
	result := make([]AccountSession, 0, len(chain))
	listed := false
	for _, session := range sessions {
		if chain[session.ID] {
			result = append(result, session)
			listed = listed || session.ID == start.ID
		}
	}
	if !listed {
		result = append(result, start)
	}
	return result
}
