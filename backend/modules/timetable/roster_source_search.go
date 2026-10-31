package timetable

import "slices"

// MaxSourceSearchOfferings bounds the exact search for a suggested source
// per enrollment phase (#5139): 2^12 combinations stay cheap per
// Regeltermin. Beyond it, the offerings booking the most planned
// child-weekdays are searched and the result is never preselected.
const MaxSourceSearchOfferings = 12

// sourceCombination is an eligible, minimal choice of offerings within one
// enrollment phase, with the score that ranks it: planned child-weekdays it
// explains (more is better), unplanned ones it adds (fewer is better), and
// its size (fewer is better).
type sourceCombination struct {
	members []int // candidate indexes, in candidate rank order
	kept    int
	added   int
}

// compareSourceCombinations orders by score, then by the members' candidate
// ranks; the second key only makes the order stable, it never breaks a tie.
func compareSourceCombinations(left, right sourceCombination) int {
	if c := compareCounts(right.kept, left.kept); c != 0 {
		return c
	}
	if c := compareCounts(left.added, right.added); c != 0 {
		return c
	}
	if c := compareCounts(len(left.members), len(right.members)); c != 0 {
		return c
	}
	return slices.Compare(left.members, right.members)
}

func sameSourceScore(left, right sourceCombination) bool {
	return left.kept == right.kept && left.added == right.added && len(left.members) == len(right.members)
}

// bestSourceCombinations searches every enrollment phase exactly and returns
// all combinations sharing the best score, ordered, and whether the search
// bound left offerings out. A combination is eligible when it lies in one
// phase, holds an offering passing the rule of #5103 and keeps the rule's
// coverage as a whole; it is minimal when every offering explains a planned
// child-weekday no other member explains.
func bestSourceCombinations(candidates []deviationCandidate, planned map[rosterSlot]bool) ([]sourceCombination, bool) {
	slots := newSlotIndex(candidates, planned)
	var best []sourceCombination
	bounded := false
	for _, members := range searchablePhaseMembers(candidates, slots) {
		if len(members) > MaxSourceSearchOfferings {
			members, bounded = boundSearchMembers(slots, members), true
		}
		for _, combination := range phaseSourceCombinations(candidates, slots, members) {
			switch {
			case len(best) == 0:
				best = []sourceCombination{combination}
			case sameSourceScore(combination, best[0]):
				best = append(best, combination)
			case compareSourceCombinations(combination, best[0]) < 0:
				best = []sourceCombination{combination}
			}
		}
	}
	slices.SortFunc(best, compareSourceCombinations)
	return best, bounded
}

// searchablePhaseMembers groups the candidates that book at least one
// planned child-weekday by enrollment phase, each group in rank order.
func searchablePhaseMembers(candidates []deviationCandidate, slots slotIndex) [][]int {
	groups := make([][]int, 0)
	position := make(map[int64]int)
	for index := range candidates {
		if !slots.offerings[index].intersects(slots.planned) {
			continue
		}
		group, found := position[candidates[index].phaseID]
		if !found {
			group = len(groups)
			position[candidates[index].phaseID] = group
			groups = append(groups, nil)
		}
		groups[group] = append(groups[group], index)
	}
	return groups
}

// boundSearchMembers keeps the offerings booking the most planned
// child-weekdays, ties in rank order, back in rank order.
func boundSearchMembers(slots slotIndex, members []int) []int {
	ranked := slices.Clone(members)
	slices.SortStableFunc(ranked, func(left, right int) int {
		return compareCounts(slots.offerings[right].and(slots.planned).count(), slots.offerings[left].and(slots.planned).count())
	})
	kept := ranked[:MaxSourceSearchOfferings]
	slices.Sort(kept)
	return kept
}

// phaseSourceCombinations enumerates the eligible, minimal combinations of
// one phase's members. union[mask] is built from union[mask without its
// lowest member], so every subset costs one bitset OR.
func phaseSourceCombinations(candidates []deviationCandidate, slots slotIndex, members []int) []sourceCombination {
	count := len(members)
	unions := make([]slotSet, 1<<count)
	unions[0] = newSlotSet(slots.size)
	passing := 0
	for position, index := range members {
		if candidates[index].passes {
			passing |= 1 << position
		}
	}
	combinations := make([]sourceCombination, 0)
	for mask := 1; mask < 1<<count; mask++ {
		lowest := lowestMember(mask)
		unions[mask] = unions[mask&(mask-1)].or(slots.offerings[members[lowest]])
		if mask&passing == 0 {
			continue
		}
		kept := unions[mask].and(slots.planned).count()
		added := unions[mask].count() - kept
		if float64(kept)/float64(kept+added) < RosterDeviationMinCoverage || !minimalSourceMask(slots, members, unions, mask) {
			continue
		}
		combinations = append(combinations, sourceCombination{members: maskMembers(members, mask), kept: kept, added: added})
	}
	return combinations
}

// minimalSourceMask reports whether every member explains a planned
// child-weekday that the other members do not.
func minimalSourceMask(slots slotIndex, members []int, unions []slotSet, mask int) bool {
	for position, index := range members {
		bit := 1 << position
		if mask&bit == 0 {
			continue
		}
		own := slots.offerings[index].and(slots.planned)
		if !own.andNot(unions[mask^bit]).any() {
			return false
		}
	}
	return true
}

func maskMembers(members []int, mask int) []int {
	result := make([]int, 0, len(members))
	for position, index := range members {
		if mask&(1<<position) != 0 {
			result = append(result, index)
		}
	}
	return result
}

// slotIndex numbers every child-weekday a candidate books and holds each
// candidate's slots and the planned ones as bitsets.
type slotIndex struct {
	size      int
	planned   slotSet
	offerings []slotSet
}

func newSlotIndex(candidates []deviationCandidate, planned map[rosterSlot]bool) slotIndex {
	number := make(map[rosterSlot]int)
	for _, candidate := range candidates {
		for slot := range candidate.booked {
			if _, found := number[slot]; !found {
				number[slot] = len(number)
			}
		}
	}
	index := slotIndex{size: len(number), planned: newSlotSet(len(number)), offerings: make([]slotSet, len(candidates))}
	for slot, position := range number {
		if planned[slot] {
			index.planned.set(position)
		}
	}
	for position, candidate := range candidates {
		index.offerings[position] = newSlotSet(len(number))
		for slot := range candidate.booked {
			index.offerings[position].set(number[slot])
		}
	}
	return index
}

type slotSet []uint64

func newSlotSet(size int) slotSet { return make(slotSet, (size+63)/64) }

func (s slotSet) set(position int) { s[position/64] |= 1 << (position % 64) }

func (s slotSet) or(other slotSet) slotSet {
	result := slices.Clone(s)
	for word := range result {
		result[word] |= other[word]
	}
	return result
}

func (s slotSet) and(other slotSet) slotSet {
	result := slices.Clone(s)
	for word := range result {
		result[word] &= other[word]
	}
	return result
}

func (s slotSet) andNot(other slotSet) slotSet {
	result := slices.Clone(s)
	for word := range result {
		result[word] &^= other[word]
	}
	return result
}

func (s slotSet) intersects(other slotSet) bool { return s.and(other).any() }

func (s slotSet) any() bool {
	for _, word := range s {
		if word != 0 {
			return true
		}
	}
	return false
}

func (s slotSet) count() int {
	total := 0
	for _, word := range s {
		for ; word != 0; word &= word - 1 {
			total++
		}
	}
	return total
}

func compareCounts(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}

// lowestMember is the position of the lowest set bit of a non-zero mask.
func lowestMember(mask int) int {
	position := 0
	for mask&1 == 0 {
		mask >>= 1
		position++
	}
	return position
}
