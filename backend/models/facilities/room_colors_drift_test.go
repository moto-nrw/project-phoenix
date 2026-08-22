package facilities_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/stretchr/testify/require"
)

// TestReservedRoomColors_MatchesFrontendLOCATION_COLORS guards the most
// fragile bit of the colour-blocking pipeline: the backend's reservedRoomColors
// list must stay in lock-step with the frontend's LOCATION_COLORS map. If
// they drift, an admin can pick a status colour the frontend offers but the
// backend rejects (or the other way round).
//
// The failure mode is silent — no compile error, no obvious runtime issue,
// just confused users. This test parses the TS source directly so a frontend
// palette tweak fails CI before merge instead of leaking into prod.
//
// We extract the hex values out of LOCATION_COLORS via regex. The TS file is
// hand-maintained and its layout is stable, so a regex is acceptable here;
// if the TS shape ever changes, this test will fail loudly and force a sync
// review.
func TestReservedRoomColors_MatchesFrontendLOCATION_COLORS(t *testing.T) {
	t.Parallel()

	frontendPath := findFrontendLocationHelper(t)

	bytes, err := os.ReadFile(frontendPath) // #nosec G304 — path computed inside repo
	require.NoError(t, err, "could not read %s", frontendPath)

	frontendHexes := extractLocationColorsHexes(t, string(bytes))
	backendHexes := exposedReservedHexes(t)

	require.NotEmpty(t, frontendHexes,
		"failed to extract any hex values from frontend LOCATION_COLORS — "+
			"the regex in this test probably needs updating to match the new layout")

	missingFromBackend := setDiff(frontendHexes, backendHexes)
	missingFromFrontend := setDiff(backendHexes, frontendHexes)

	if len(missingFromBackend) > 0 || len(missingFromFrontend) > 0 {
		t.Fatalf(
			"reservedRoomColors drift detected — keep "+
				"backend/models/facilities/room_colors.go in sync with "+
				"frontend/src/lib/location-helper.ts LOCATION_COLORS.\n"+
				"  Hex codes in frontend but not backend: %v\n"+
				"  Hex codes in backend but not frontend: %v\n"+
				"  Frontend file: %s",
			missingFromBackend, missingFromFrontend, frontendPath,
		)
	}
}

// findFrontendLocationHelper walks up from the package's working directory
// to the repo root, then down to frontend/src/lib/location-helper.ts. We
// can't hard-code the relative path because Go test working directory is
// the package dir, not the repo root, and depth varies depending on which
// package the test runs from.
func findFrontendLocationHelper(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, "frontend", "src", "lib", "location-helper.ts")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatalf("could not locate frontend/src/lib/location-helper.ts walking up from %s", dir)
	return ""
}

// extractLocationColorsHexes pulls every hex literal that appears inside the
// `export const LOCATION_COLORS = { ... } as const;` block. We deliberately
// scope the search to that block so unrelated hex literals elsewhere in the
// file (e.g. GROUP_ROOM_SHADES) don't pollute the comparison set.
func extractLocationColorsHexes(t *testing.T, source string) map[string]struct{} {
	t.Helper()

	startMarker := "export const LOCATION_COLORS"
	startIdx := strings.Index(source, startMarker)
	require.NotEqual(t, -1, startIdx,
		"could not find LOCATION_COLORS export — frontend file structure changed")

	// Block ends at the matching closing brace of the object literal.
	closeIdx := strings.Index(source[startIdx:], "} as const")
	require.NotEqual(t, -1, closeIdx,
		"could not find end of LOCATION_COLORS block — frontend file structure changed")
	block := source[startIdx : startIdx+closeIdx]

	hexPattern := regexp.MustCompile(`"(#[0-9A-Fa-f]{3,6})"`)
	literalMatches := hexPattern.FindAllStringSubmatch(block, -1)
	palettePattern := regexp.MustCompile(`MOTO_COLOR_PALETTE\.([A-Za-z][A-Za-z0-9]*)\.base`)
	paletteMatches := palettePattern.FindAllStringSubmatch(block, -1)
	paletteBaseHexes := extractPaletteBaseHexes(t, source)

	out := make(map[string]struct{}, len(literalMatches)+len(paletteMatches))
	for _, m := range literalMatches {
		out[strings.ToUpper(m[1])] = struct{}{}
	}
	for _, m := range paletteMatches {
		hex, ok := paletteBaseHexes[m[1]]
		require.True(t, ok, "could not resolve MOTO_COLOR_PALETTE.%s.base", m[1])
		out[hex] = struct{}{}
	}
	return out
}

func extractPaletteBaseHexes(t *testing.T, source string) map[string]string {
	t.Helper()

	startIdx := strings.Index(source, "export const MOTO_COLOR_PALETTE")
	require.NotEqual(t, -1, startIdx,
		"could not find MOTO_COLOR_PALETTE export — frontend file structure changed")

	closeIdx := strings.Index(source[startIdx:], "} as const")
	require.NotEqual(t, -1, closeIdx,
		"could not find end of MOTO_COLOR_PALETTE block — frontend file structure changed")
	block := source[startIdx : startIdx+closeIdx]

	basePattern := regexp.MustCompile(`(?ms)^\s*([A-Za-z][A-Za-z0-9]*):\s*\{.*?^\s*base:\s*"(#[0-9A-Fa-f]{3,6})"`)
	matches := basePattern.FindAllStringSubmatch(block, -1)

	out := make(map[string]string, len(matches))
	for _, m := range matches {
		out[m[1]] = strings.ToUpper(m[2])
	}
	return out
}

// exposedReservedHexes mirrors what reservedRoomColors holds, EXCLUDING
// entries that are intentionally backend-only (e.g. the legacy bug-default
// #4F46E5, reserved for migration-restore-correctness reasons rather than
// because it's a frontend status badge). The drift test only cares about
// cross-codebase agreement on the palette half — backend-only entries are
// asserted separately below.
//
// We probe via IsReservedRoomColor against a candidate set — that's the
// public contract, so testing through it avoids reaching into unexported
// state.
func exposedReservedHexes(t *testing.T) map[string]struct{} {
	t.Helper()

	// Hard-coded mirror of the palette half of reservedRoomColors. Backend-
	// only entries (legacy bug-default) live in a separate assertion below.
	knownReserved := []string{
		"#83CD2D",
		"#5080D8",
		"#6B7280",
		"#F78C10",
		"#D946EF",
		"#78716C",
		"#DC2626",
		"#7C3AED",
		"#0891B2",
		"#365D83",
		"#EAB308",
	}

	out := make(map[string]struct{}, len(knownReserved))
	for _, hex := range knownReserved {
		require.True(t, facilities.IsReservedRoomColor(hex),
			"backend room_colors.go knownReserved is out of sync with "+
				"reservedRoomColors map: %s should be reserved but isn't",
			hex)
		out[strings.ToUpper(hex)] = struct{}{}
	}
	return out
}

// TestReservedRoomColors_LegacyBugDefault pins the backend-only entry that
// the drift test deliberately excludes. Tests intent rather than mechanics:
// even if someone refactors the palette half, this assertion stays put and
// keeps #4F46E5 reserved.
func TestReservedRoomColors_LegacyBugDefault(t *testing.T) {
	t.Parallel()

	require.True(t, facilities.IsReservedRoomColor("#4F46E5"),
		"legacy bug-default #4F46E5 must remain reserved — see "+
			"migration 1.15.45 / room_colors.go for the rationale")
}

// TestReservedRoomColors_RetiredStatusColors pins the hexes that used to be
// frontend status colors. They are backend-only now — LOCATION_COLORS no
// longer carries them, so exposedReservedHexes deliberately excludes them and
// nothing else would fail if they were dropped from reservedRoomColors.
//
// They must stay reserved regardless: rooms created while these were the live
// HOME/SICK colors are still out there, and re-opening the hexes for picking
// would let a room masquerade as a status for anyone whose client still maps
// the old palette.
func TestReservedRoomColors_RetiredStatusColors(t *testing.T) {
	t.Parallel()

	for _, hex := range []string{
		"#FF3130", // previous HOME status color
	} {
		require.True(t, facilities.IsReservedRoomColor(hex),
			"retired status color %s must remain reserved — see the "+
				"reservedRoomColors comment in room_colors.go", hex)
	}
}

func setDiff(a, b map[string]struct{}) []string {
	out := make([]string, 0)
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
