## ADDED Requirements

### Requirement: Structured Student Location Status
- Backend APIs and SSE events MUST deliver a `StudentLocationStatus` object per student with a canonical `state` (`PRESENT_IN_ROOM`, `TRANSIT`, `SCHOOLYARD`, `HOME`) and optional room metadata when applicable.
- Room metadata MUST include `id`, `name`, `isGroupRoom`, and `ownerType` (`GROUP` or `ACTIVITY`) so the UI can distinguish group vs. activity rooms without string parsing.

#### Scenario: Present in home group room
- **GIVEN** backend emits `{ state: "PRESENT_IN_ROOM", room: { id: "12", name: "Gruppenraum 3", isGroupRoom: true } }`
- **WHEN** the frontend receives the payload
- **THEN** the helper MUST treat the student as present in their group room using the provided label.

#### Scenario: Present in foreign room
- **GIVEN** backend emits `{ state: "PRESENT_IN_ROOM", room: { id: "42", name: "Werkraum", isGroupRoom: false } }`
- **WHEN** the frontend receives the payload
- **THEN** the helper MUST label the badge "Werkraum" and style it as “other room”.

#### Scenario: Transit state without room data
- **GIVEN** backend emits `{ state: "TRANSIT" }`
- **WHEN** the frontend receives the payload
- **THEN** the badge MUST display "Unterwegs".

#### Scenario: Schoolyard state
- **GIVEN** backend emits `{ state: "SCHOOLYARD" }`
- **WHEN** the frontend receives the payload
- **THEN** the badge MUST display "Schulhof".

#### Scenario: Home state
- **GIVEN** backend emits `{ state: "HOME" }`
- **WHEN** the frontend receives the payload
- **THEN** the badge MUST display "Zuhause".

#### Scenario: Room metadata precedence
- **GIVEN** backend emits `{ state: "PRESENT_IN_ROOM", room: { id: "99", name: "Musikraum", isGroupRoom: false, ownerType: "ACTIVITY" } }`
- **AND** no additional overrides are provided
- **WHEN** the helper resolves the badge
- **THEN** it MUST use the provided room name without consulting legacy suffix parsing.

#### Scenario: Transit only between check-out and next check-in
- **GIVEN** a student checks out of room A at 14:00 and has not yet checked into another room
- **WHEN** backend emits the next SSE event
- **THEN** it MUST provide `{ state: "TRANSIT" }` until the student checks into the next room.

### Requirement: Unified Badge Helper & Styling
- Frontend MUST expose a single helper/component that maps `StudentLocationStatus` to badge label and styling tokens shared across OGS groups, My Room, student search, and the student detail modal.
- Styling tokens for group room, other room, transit, schoolyard, and home MUST be defined in one theme map to enable consistent updates.

#### Scenario: Shared styling across surfaces
- **GIVEN** the helper is applied to OGS groups, My Room, student search, and the student detail modal
- **WHEN** each surface renders a student in the same state (e.g., `PRESENT_IN_ROOM` with `isGroupRoom: false`)
- **THEN** every surface MUST display the same label and styling tokens.

### Requirement: Live Updates via SSE
- All four surfaces MUST subscribe to student location SSE updates and refresh badges immediately upon receiving new events.
- When SSE is unavailable, surfaces MAY fall back to a 30-second polling loop but MUST keep displaying the last known state.

#### Scenario: SSE update propagates to surfaces
- **GIVEN** a student is displayed on OGS groups, My Room, student search, and the detail modal with state `HOME`
- **WHEN** an SSE event arrives with `{ state: "PRESENT_IN_ROOM", room: { id: "42", name: "Werkraum", isGroupRoom: false } }`
- **THEN** all four surfaces MUST update the badge to "Werkraum" without requiring a manual refresh.

#### Scenario: SSE outage falls back to polling
- **GIVEN** the student detail modal is open and SSE disconnects
- **WHEN** 30 seconds pass since the last successful update
- **THEN** the modal MUST fetch `/api/students/:id/current-location` to refresh the badge while keeping the last known state visible.

#### Scenario: My Room fetches on demand
- **GIVEN** a supervisor switches from Room A to Room B in My Room
- **WHEN** Room B becomes active
- **THEN** the frontend MUST request the current `StudentLocationStatus` set for Room B (via SSE subscription or on-demand fetch) without preloading all rooms.

#### Scenario: Student search includes home students
- **GIVEN** student search results include a student whose status is `{ state: "HOME" }`
- **WHEN** the list renders
- **THEN** the badge MUST display "Zuhause", confirming that at-home students remain visible.


### Requirement: Privacy-Aware Display
- Supervisors MUST see badges for any student currently in the rooms they supervise, regardless of origin group, but detailed history remains restricted to direct educators.

#### Scenario: Foreign-room supervisor sees limited info
- **GIVEN** a supervisor is viewing My Room for an activity-owned room
- **AND** a visiting student from another educational group is present
- **WHEN** the badge renders via the shared helper
- **THEN** the supervisor MUST see the location badge (e.g., room name) but MUST NOT rely on the badge to expose restricted history data.

### Requirement: Deprecate Legacy Location Flags
- Frontend MUST remove usage of legacy booleans (`in_house`, `wc`, `school_yard`) and string parsing patterns (e.g., "Anwesend - ...").
- Backend documentation MUST mark those fields as deprecated for downstream consumers.

#### Scenario: Legacy flag removal
- **GIVEN** the frontend receives a student payload
- **WHEN** mapping the data for UI
- **THEN** it MUST ignore legacy boolean flags and rely solely on `StudentLocationStatus` for badge decisions.

#### Scenario: Deprecated schema announcement
- **GIVEN** release notes are published for the rollout
- **THEN** they MUST highlight the new structured location schema and the deprecation of legacy flags to downstream teams.
