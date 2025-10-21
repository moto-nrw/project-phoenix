# Design Notes

## Overview
We will standardize student location rendering by consolidating state detection and styling in a single helper/component called `getLocationBadge`. The helper consumes the rich `student.current_location` string plus optional real-time room metadata (group room name, foreign room mapping, room IDs). Each UI surface will invoke the helper rather than duplicating logic.

## Helper Responsibilities
1. **State Parsing:**
   - Interpret prefixes such as `"Anwesend - "`, `"Anwesend in "`, and plain values (`"Zuhause"`, `"WC"`, `"School Yard"`, `"Bus"`, `"Unterwegs"`).
   - Fall back to the canonical label **"Unbekannt"** when no match exists (confirmed with UX/localisation).
2. **Room Resolution:**
   - Accept optional room metadata via an input contract:
     - `groupContext?: { roomName?: string }`
     - `roomStatus?: { currentRoomId?: string | number; inGroupRoom?: boolean }`
     - `roomMap?: Record<string, { name: string }>`
     - `explicitRoomName?: string`
   - Priority: `explicitRoomName` → `roomStatus.currentRoomId` resolved via `roomMap` → `groupContext.roomName` (for in-group room) → suffix parsing from `current_location`.
3. **Styling Contract:**
   - Return token keys (e.g., `location.present.groupRoom`) that map to centralized gradient/background definitions stored in a theme map.
   - Provide text label, icon affordances (pulse dot), and optional gradient classes for card overlays.

## Integration Strategy
- **ModernStatusBadge Update:** Extend the existing component to consume the helper and expose a consistent interface (`locationStatus = getLocationBadge(student, options)`).
- **Surface Adaptation:**
  - OGS & search: replace local `getLocationStatus` utilities with the helper, passing SSE/room maps when available; document that OGS receives SSE updates while search relies on refreshed fetches.
  - My Room: after fetching students, pass each through helper using visit data to detect foreign rooms vs group rooms; confirm SSE visit updates continue to refresh the badge.
  - Student detail modal: fetch `/api/students/:id/current-location` (already implemented) and display helper output in the header. We will adopt a periodic re-fetch (30-second interval while the modal remains open) owned by the frontend team to keep badges current without introducing a new SSE subscription inside the modal.
  - Introduce a `NEXT_PUBLIC_ENABLE_UNIFIED_LOCATION_BADGE` runtime flag, defaulting to disabled, that chooses between the legacy badge rendering and the shared helper during rollout. Release Engineering owns the flag configuration/toggles across environments, in coordination with the frontend team for validation.

## Data Shape Updates
- Adjust `mapStudentResponse` to stop hydrating deprecated booleans. Instead, expose convenience helpers (`isPresent`, `isOffsite`) derived from `current_location` if needed by legacy code.
- Confirm SSE payloads for OGS/My Room include `in_group_room` and `current_room_id`; document fallback behaviour when either is missing (helper falls back to suffix parsing or `Unbekannt`).
- Catalogue any external consumers of the boolean flags and communicate removals prior to merge.

## Testing & QA
- Unit tests verifying helper outputs for representative inputs (group room, foreign room, Schulhof, WC, Bus, Unterwegs, Zuhause, Unbekannt, `Anwesend - …`, `Anwesend in …`).
- Storybook snapshots (or equivalent visual regression) for ModernStatusBadge token variations to detect styling regressions.
- Manual QA checklist covering the four surfaces plus an SSE-triggered update in OGS/My Room, including verifying the modal refresh strategy and confirming search parity after a manual refresh.
