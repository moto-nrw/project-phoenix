package e2e

import (
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	hostsFilePath     = "/etc/hosts"
	hostsBeginMarker  = "# project-phoenix E2E BEGIN (managed by `phoenix e2e hosts sync`)"
	hostsEndMarker    = "# project-phoenix E2E END (managed by `phoenix e2e hosts sync`)"
	hostsLegacyMarker = "# project-phoenix E2E (managed by scripts/setup-e2e-hosts.sh)"
)

func SyncHostsForScenario(name string) error {
	hosts, err := HostsForScenario(name)
	if err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("syncing /etc/hosts requires sudo; run `sudo go run . e2e hosts sync --scenario %s` from backend/", name)
	}

	current, err := os.ReadFile(hostsFilePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", hostsFilePath, err)
	}

	managed := make([]string, 0, len(hosts))
	for _, host := range hosts {
		managed = append(managed, "127.0.0.1 "+host)
	}

	updated := renderHostsFile(string(current), managed)
	if updated == string(current) {
		fmt.Println("/etc/hosts already matches the canonical E2E scenario.")
		return nil
	}

	if err := os.WriteFile(hostsFilePath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", hostsFilePath, err)
	}

	fmt.Println("Updated /etc/hosts for the canonical E2E scenario:")
	for _, entry := range managed {
		fmt.Printf("  %s\n", entry)
	}
	return nil
}

func ensureScenarioHostsResolvable(name string) error {
	hosts, err := HostsForScenario(name)
	if err != nil {
		return err
	}

	unresolved := make([]string, 0)
	for _, host := range hosts {
		if _, err := net.LookupHost(host); err != nil {
			unresolved = append(unresolved, host)
		}
	}
	if len(unresolved) == 0 {
		return nil
	}

	if os.Geteuid() == 0 {
		return SyncHostsForScenario(name)
	}

	return fmt.Errorf(
		"the canonical localtest.me hostnames are not resolvable locally (%s). Run `sudo go run . e2e hosts sync --scenario %s` from backend/ once and retry",
		strings.Join(unresolved, ", "),
		name,
	)
}

func renderHostsFile(current string, entries []string) string {
	lines := strings.Split(current, "\n")
	out := make([]string, 0, len(lines)+len(entries)+4)
	inManaged := false
	inLegacy := false

	for _, line := range lines {
		switch {
		case line == hostsBeginMarker:
			inManaged = true
			continue
		case line == hostsEndMarker:
			inManaged = false
			continue
		case inManaged:
			continue
		case line == hostsLegacyMarker:
			inLegacy = true
			continue
		case inLegacy && (line == "" || strings.HasPrefix(line, "127.0.0.1 ")):
			continue
		case inLegacy:
			inLegacy = false
		}
		out = append(out, line)
	}

	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}

	out = append(out, "", hostsBeginMarker)
	out = append(out, entries...)
	out = append(out, hostsEndMarker, "")
	return strings.Join(out, "\n")
}
