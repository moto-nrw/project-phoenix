package architecture

import (
	"fmt"
	"strings"
)

// Runtime direction is independent of effect: a read-only callback into a
// workflow is not an ordinary domain dependency either (ADR 0071 point 2).
func validateEffectDirection(entry commandEffectEntry, ownerKinds map[string]string, classified map[string]Package) error {
	if (entry.Classification == "platform") != (ownerKinds[entry.TargetOwner] == "platform" && entry.Effect != "query") {
		return fmt.Errorf("platform classification %s is reserved for non-query platform protocol targets; platform reads are queries", entry.key())
	}
	if ownerKinds[entry.sourceOwner()] != "domain" || ownerKinds[entry.TargetOwner] != "workflow" {
		return nil
	}
	// The interface's policy role, not its owner's kind or the root binding's
	// location, distinguishes a transport entry from a domain callback.
	if methodSeparator := strings.LastIndex(entry.Port, "."); methodSeparator >= 0 {
		portType := entry.Port[:methodSeparator]
		if separator := strings.LastIndex(portType, "."); separator >= 0 {
			if source := classified[portType[:separator]]; source.Owner == entry.sourceOwner() && source.Role == "http" {
				return nil
			}
		}
	}
	switch entry.Classification {
	case "debt", "E1", "E2", "E3", "E4", "E5":
		return nil // The effect validator and immutable-base check require ADR evidence.
	case "workflow":
		if entry.sourceOwner() == "care-plan" && entry.TargetOwner == "care-exit" && careExitEffectCallback(entry.Target) {
			return nil
		}
	}
	return fmt.Errorf("domain-to-workflow binding %s needs debt or a named exception; workflow is limited to ADR 0071's Care Exit Roster/Bookings E2 callbacks", entry.key())
}

func careExitEffectCallback(target string) bool {
	const prefix = "github.com/moto-nrw/project-phoenix/workflows/careexit."
	switch strings.TrimPrefix(strings.TrimPrefix(target, "*"), prefix) {
	case "Roster.CloseOpenStudentAssignments", "Roster.LockOpenStudentAssignments",
		"Roster.LockPlannedRosterForCareExit", "Roster.ReconnectCareExitAssignmentPickupExceptions",
		"Roster.RemovePlannedRosterForCareExit", "Roster.RestoreRosterForCareExit",
		"Bookings.RestoreCareExitOfferingLinks":
		return strings.HasPrefix(strings.TrimPrefix(target, "*"), prefix)
	}
	return false
}
