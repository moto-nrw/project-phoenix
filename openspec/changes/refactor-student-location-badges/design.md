# Design Notes

## Location Model
- Define `StudentLocationStatus`:
  ```ts
  type StudentLocationStatus =
    | { state: "PRESENT_IN_ROOM"; room: { id: string; name: string; isGroupRoom: boolean; ownerType: "GROUP" | "ACTIVITY" } }
    | { state: "TRANSIT" }
    | { state: "SCHOOLYARD" }
    | { state: "HOME" };
  ```
- Backend derives this structure from `active.visits` (current room) and ownership metadata (educational group or activity). `TRANSIT` is emitted strictly between checkout and the next check-in; `SCHOOLYARD` reflects the yard flag in attendance, and `HOME` represents not checked in today.
- Structure is intentionally extensible so additional states (e.g., future offsite activities) can be added without redesigning the helper.
- REST endpoints and SSE events must emit this object for every student so the frontend no longer parses strings.

## Badge Helper
- Implement `getStudentLocationBadge(status, options)` that returns `{ label, colorToken, gradientToken, icon }`:
  - PRESENT_IN_ROOM: label = `status.room.name`; color depends on `isGroupRoom` (group room vs other room).
  - TRANSIT: label = "Unterwegs".
  - SCHOOLYARD: label = "Schulhof".
  - HOME: label = "Zuhause".
- Styling tokens live in a single map so any color tweak updates all surfaces.

## Surface Integration
- **OGS Groups:** Subscribe to SSE channel delivering `StudentLocationStatus`; update badges immediately.
- **My Room:** Same SSE feed for room supervisors; fetch student status on demand when a supervisor switches to a room, and display each student’s origin group separately (outside the badge) when needed.
- **Student Search:** Add SSE subscription for live updates (covering students at home as well); if SSE disconnects, poll `/api/students` every 30 seconds until the stream resumes.
- **Student Detail Modal:** Subscribe while open; if SSE unavailable, poll `/api/students/:id/current-location` every 30 seconds and keep last known state.
- All surfaces use the shared badge helper; legacy code paths and boolean flags are removed.

## Visibility Rules
| Surface | Supervisor Sees | Restricted Data | Notes |
|---------|------------------|------------------|-------|
| OGS Groups | Students in their educational group (group rooms) | Student history/notes remain within direct educators | SSE delivers structured status for each student |
| My Room | All students physically in the supervised room (group-owned or activity-owned) | Detailed history for visiting students stays hidden unless supervisor belongs to the student's home group | Fetch on demand when switching rooms, then rely on SSE |
| Student Search | All students the user is permitted to view, including those at HOME | Detailed history limited to direct educators | SSE keeps list current; HOME renders as "Zuhause" |
| Student Detail Modal | Students the viewer already has permission to open | No additional data exposed beyond modal permissions | Poll `/api/students/:id/current-location` every 30s if SSE drops |


## Deprecated Fields
- `in_house`, `wc`, `school_yard`: removed from frontend decision logic; replaced by `StudentLocationStatus.state`.
- Legacy location strings (e.g., "Anwesend - Raum X"): removed; room labels now supplied via metadata.
- Bus permission flag ("Buskind"): remains an administrative attribute but is no longer interpreted as a location state by badges.

## Error Handling
- If SSE disconnects, surfaces continue showing the last known badge and rely on existing SSE health indicator text elsewhere in the UI.
- Helper defaults to "Zuhause" only when the backend explicitly emits HOME; no implicit fallbacks.
- UI components respect privacy rules: supervisors see who is in their room, while detailed histories remain restricted to the student’s direct educators.

## Rollout & Communication
- No feature flag; migrate all surfaces in a single release after UX sign-off on German labels and styling tokens.
