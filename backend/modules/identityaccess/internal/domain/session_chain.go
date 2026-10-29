package domain

// SessionChain returns every session of the account that a logout of start
// must revoke (#5194): start, its successors along replacement_token, every
// session of a family any of them carries, and every predecessor that hands
// off into that set, until nothing more joins. A row the family backfill
// could not place carries no family; following its links instead of
// stopping at it means no session of its chain survives the logout. Each
// session joins once, so a cycle ends the walk. sessions are the account's
// sessions, start included.
func SessionChain(start AccountSession, sessions []AccountSession) []AccountSession {
	index := newSessionIndex(sessions)
	chain := map[int64]bool{start.ID: true}
	queue := []AccountSession{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range index.neighbours(current) {
			if !chain[next.ID] {
				chain[next.ID] = true
				queue = append(queue, next)
			}
		}
	}
	return inChain(start, sessions, chain)
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

// neighbours are the successors, the family members and the predecessors
// of a session.
func (i sessionIndex) neighbours(session AccountSession) []AccountSession {
	var linked []AccountSession
	if session.ReplacementTokenHash != nil {
		linked = append(linked, i.byToken[*session.ReplacementTokenHash]...)
	}
	if session.FamilyID != "" {
		linked = append(linked, i.byFamily[session.FamilyID]...)
	}
	return append(linked, i.byReplacement[session.TokenHash]...)
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
