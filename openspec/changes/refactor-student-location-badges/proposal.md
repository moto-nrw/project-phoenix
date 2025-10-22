## Why
- Current location badges rely on loosely parsed strings (e.g., "Anwesend - Raum") and legacy booleans (in_house, wc, school_yard) that no longer represent the real student state.
- Surfaces update inconsistently: OGS groups and My Room stream SSE events, while student search and the student detail modal fall back to manual fetches, producing stale data.
- Supervisors need a trustworthy view of where students are (home, in their group room, in another room, on the schoolyard, or in transit) with consistent styling across the entire UI.

## What Changes
- Introduce a structured `StudentLocationStatus` model that captures canonical states (`PRESENT_IN_ROOM`, `TRANSIT`, `SCHOOLYARD`, `HOME`) plus room metadata (`id`, `name`, `isGroupRoom`, `ownerType`) sourced from the real-time attendance backend instead of ad-hoc strings.
- Build a single badge helper/component that renders unified labels and styling for every surface (OGS groups, My Room, student search, student detail modal) using the structured status.
- Extend SSE integration so all four surfaces receive live updates; fall back to the last known state if the connection drops while indicating SSE health elsewhere.
- Remove legacy location strings/booleans (including the old WC handling) and replace any badge logic tied to the Bus permission flag with the new structured status.

## Impact
- Product: Supervisors immediately see accurate, real-time student locations with consistent visuals regardless of where they are in the app.
- Engineering: Single source of truth for badges, simpler future refinements, and elimination of fragile string parsing.
- Data: Clearly defined location schema enables analytics/exports to consume structured location data.
- Risk: Requires synchronizing SSE payload/schema updates and updating every surface simultaneously; mitigated with thorough tests, coordinated rollout, and communication to downstream consumers.

## Dependencies
- Backend attendance service must expose the structured location payload (via SSE events and REST endpoints) containing canonical state + room metadata.
- UX/product sign-off on final badge labels/colors.
