package application

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The billing unit child-month (ADR 0158, Nachtrag Kind-Monat): the monthly
// run bills each child once a month. childMonthHolds decides which children
// it holds back for a duplicate contract (E1 to E4); holdSnapshot is what a
// release stores of them and holdView what a released run shows (E5). Both are pure: they read only their
// arguments.

// childMonthHolds groups the contracts of the month by child (E1), lays
// down the days every contract bills under the rule (E2) and finds a
// duplicate where two contracts of a child bill the same module on the
// same calendar day (E3). A child with a duplicate is held with all its
// contracts (E4). It returns one entry per doubly billed module and
// weekday, each naming every contract of its child, and the held contracts.
func childMonthHolds(month calendar.Date, rule string, byContract map[int64][]parentbilling.CareContractVersion, order []int64) ([]parentbilling.CareDuplicate, map[int64]bool) {
	duplicates := []parentbilling.CareDuplicate{}
	held := map[int64]bool{}
	entries := monthEntries(month, byContract, order)
	for _, child := range childrenOf(byContract, order) {
		doubled := doublyBilled(month, rule, child, byContract, entries)
		if len(doubled) == 0 {
			continue
		}
		contracts := childContracts(child, byContract)
		head := byContract[child[0]][0]
		for _, slot := range doubled {
			duplicates = append(duplicates, parentbilling.CareDuplicate{
				ChildFirstName: head.ChildFirstName, ChildLastName: head.ChildLastName,
				ModuleName: slot.name, Weekday: slot.weekday, Contracts: contracts,
			})
		}
		for _, contractID := range child {
			held[contractID] = true
		}
	}
	return duplicates, held
}

// holdSnapshot is what a release stores of the children it holds back
// (E5): per child, in the order of the run, its name, every contract with
// its school and the modules and weekdays billed twice.
func holdSnapshot(duplicates []parentbilling.CareDuplicate) []parentbilling.CareRunHold {
	holds := []parentbilling.CareRunHold{}
	index := map[int64]int{}
	for _, duplicate := range duplicates {
		key := duplicate.Contracts[0].ContractID
		i, seen := index[key]
		if !seen {
			i = len(holds)
			index[key] = i
			holds = append(holds, parentbilling.CareRunHold{
				Position: i + 1, Reason: parentbilling.HoldReasonDuplicateContract,
				ChildFirstName: duplicate.ChildFirstName, ChildLastName: duplicate.ChildLastName,
				Contracts: slices.Clone(duplicate.Contracts), Slots: []parentbilling.CareHeldSlot{},
			})
		}
		holds[i].Slots = append(holds[i].Slots, parentbilling.CareHeldSlot{ModuleName: duplicate.ModuleName, Weekday: duplicate.Weekday})
	}
	return holds
}

// holdView is what a released run shows of the children it held back: the
// stored snapshot and nothing else (E5), one entry per module and weekday
// billed twice, as the preview showed it at the release.
func holdView(holds []parentbilling.CareRunHold) []parentbilling.CareDuplicate {
	shown := []parentbilling.CareDuplicate{}
	for _, hold := range holds {
		for _, slot := range hold.Slots {
			shown = append(shown, parentbilling.CareDuplicate{
				ChildFirstName: hold.ChildFirstName, ChildLastName: hold.ChildLastName,
				ModuleName: slot.ModuleName, Weekday: slot.Weekday, Contracts: slices.Clone(hold.Contracts),
			})
		}
	}
	return shown
}

// childrenOf groups the contracts by child (E1), in the order of the
// contracts: by the child record, without one by the normalized name and
// date of birth. A contract without a record joins the child whose record's
// contracts carry the same name and date of birth when exactly one does.
func childrenOf(byContract map[int64][]parentbilling.CareContractVersion, order []int64) [][]int64 {
	recordsOf := map[string][]int64{}
	for _, contractID := range order {
		version := byContract[contractID][0]
		if version.StudentID != nil && !slices.Contains(recordsOf[personOf(version)], *version.StudentID) {
			recordsOf[personOf(version)] = append(recordsOf[personOf(version)], *version.StudentID)
		}
	}
	groups := map[string][]int64{}
	var keys []string
	for _, contractID := range order {
		key := childKeyOf(byContract[contractID][0], recordsOf)
		if _, seen := groups[key]; !seen {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], contractID)
	}
	children := make([][]int64, len(keys))
	for i, key := range keys {
		children[i] = groups[key]
	}
	return children
}

// childKeys names per contract the child it bills, as childrenOf groups
// them (ADR 0158 E1).
func childKeys(byContract map[int64][]parentbilling.CareContractVersion, order []int64) map[int64]string {
	keys := map[int64]string{}
	for i, child := range childrenOf(byContract, order) {
		for _, contractID := range child {
			keys[contractID] = "child:" + strconv.Itoa(i)
		}
	}
	return keys
}

// childKeyOf is the child a contract bills: its record, the one record
// with its name and date of birth, else that name and date of birth.
func childKeyOf(version parentbilling.CareContractVersion, recordsOf map[string][]int64) string {
	if version.StudentID != nil {
		return fmt.Sprintf("student:%d", *version.StudentID)
	}
	if records := recordsOf[personOf(version)]; len(records) == 1 {
		return fmt.Sprintf("student:%d", records[0])
	}
	return "person:" + personOf(version)
}

// personOf is the normalized name and date of birth of the child of a
// contract.
func personOf(version parentbilling.CareContractVersion) string {
	birth := ""
	if version.ChildBirthDate != nil {
		birth = version.ChildBirthDate.String()
	}
	return normalizedName(version.ChildFirstName) + "|" + normalizedName(version.ChildLastName) + "|" + birth
}

func normalizedName(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

// slotKey is one module on one calendar day of the month.
type slotKey struct {
	offeringID int64
	day        calendar.Date
}

// doubledSlot is a module and weekday two contracts of a child both bill.
type doubledSlot struct {
	offeringID int64
	name       string
	weekday    string
}

// doublyBilled lists, in the order of the days, every module and weekday
// that two contracts of the child bill on the same calendar day (E3), each
// once.
func doublyBilled(month calendar.Date, rule string, child []int64, byContract map[int64][]parentbilling.CareContractVersion, entries map[int64]bool) []doubledSlot {
	billers := map[slotKey]int{}
	names := map[int64]string{}
	for _, contractID := range child {
		for slot, name := range billedSlots(month, rule, byContract[contractID], entries[contractID]) {
			billers[slot]++
			names[slot.offeringID] = name
		}
	}
	var doubled []doubledSlot
	for day := month; !day.After(monthEnd(month)); day = day.AddDays(1) {
		for offeringID, name := range names {
			slot := doubledSlot{offeringID: offeringID, name: name, weekday: weekdayOf(day)}
			if billers[slotKey{offeringID, day}] >= 2 && !slices.Contains(doubled, slot) {
				doubled = append(doubled, slot)
			}
		}
	}
	slices.SortStableFunc(doubled, func(a, b doubledSlot) int {
		return firstNonZero(compareOrdered(a.name, b.name), compareOrdered(a.offeringID, b.offeringID),
			compareOrdered(weekdayOrder[a.weekday], weekdayOrder[b.weekday]))
	})
	return doubled
}

// billedSlots are the calendar days a contract bills each module on under
// the rule (E2): the tracks of the month, the same ones its invoice bills.
func billedSlots(month calendar.Date, rule string, versions []parentbilling.CareContractVersion, entry bool) map[slotKey]string {
	slots := map[slotKey]string{}
	for _, track := range tracksOf(month, rule, versions, entry) {
		for _, run := range track.runs {
			for day := run.from; !day.After(run.until); day = day.AddDays(1) {
				if slices.Contains(run.weekdays, weekdayOf(day)) {
					slots[slotKey{track.offeringID, day}] = track.name
				}
			}
		}
	}
	return slots
}

// monthEntries names, per child without a contract in force on the first
// of the month, the first contract that begins in the month (E2 under the
// full month): it bills the whole month by its first version. Any further
// contract that begins later bills nothing that month, and a child with a
// contract on the first has no entry. On the same start day the earlier
// contract of the order is the entry.
func monthEntries(month calendar.Date, byContract map[int64][]parentbilling.CareContractVersion, order []int64) map[int64]bool {
	entries := map[int64]bool{}
	for _, child := range childrenOf(byContract, order) {
		entry := int64(0)
		for _, contractID := range child {
			versions := byContract[contractID]
			if _, onFirst := versionOn(versions, month); onFirst {
				entry = 0
				break
			}
			if entry == 0 || versions[0].From.Before(byContract[entry][0].From) {
				entry = contractID
			}
		}
		if entry != 0 {
			entries[entry] = true
		}
	}
	return entries
}

// childContracts names every contract of a child with its school.
func childContracts(child []int64, byContract map[int64][]parentbilling.CareContractVersion) []parentbilling.CareDuplicateContract {
	contracts := make([]parentbilling.CareDuplicateContract, len(child))
	for i, contractID := range child {
		head := byContract[contractID][0]
		contracts[i] = parentbilling.CareDuplicateContract{ContractID: contractID, SchoolID: head.SchoolID, SchoolName: head.SchoolName}
	}
	return contracts
}
