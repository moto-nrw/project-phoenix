## 1. Alignment & Discovery
- [ ] 1.1 Document how current locations are stored (active.visits, group room assignments) and confirm backend ability to emit canonical states + room metadata (id, name, isGroupRoom, ownerType).
- [ ] 1.2 Align with UX/product (including German labels and colors) on final badge terminology; capture approval before implementation starts.
- [ ] 1.3 Notify analytics/exports stakeholders about the planned deprecation of legacy boolean flags and capture feedback/requirements.

## 2. Schema & API Updates
- [ ] 2.1 Introduce a structured `StudentLocationStatus` payload (state enum + optional room object including ownerType) in the backend responses and SSE events.
- [ ] 2.2 Define and publish the canonical backend schema/OpenAPI contract for `StudentLocationStatus` (field names, nullability, enum values) and align it with the frontend TypeScript type.
- [ ] 2.3 Update REST endpoints and SSE channel to deliver the structured payload for OGS groups, My Room, student search, and student detail modal.

## 3. Frontend Implementation
- [ ] 3.1 Build a shared `getStudentLocationBadge` helper + badge component that accepts `StudentLocationStatus` and returns unified label/styling tokens.
- [ ] 3.2 Refactor OGS groups, My Room, student search, and the student detail modal to consume the shared helper and structured status while keeping detailed history restricted to authorized supervisors.
- [ ] 3.3 Ensure all four surfaces subscribe to SSE updates; apply a 30s fallback poll only if SSE is unavailable and keep the last known state otherwise. My Room MUST fetch student status on demand when switching rooms.
- [ ] 3.4 Remove parsing of legacy strings/booleans (e.g., “Anwesend - …”, in_house, school_yard, wc) and mark deprecated fields accordingly; ensure student search still lists students at home.

## 4. Validation & Rollout
- [ ] 4.1 Add unit tests covering each canonical state, including room metadata precedence.
- [ ] 4.2 Add integration/UI tests (or Storybook visual baselines) for the unified badge across surfaces.
- [ ] 4.3 Verify SSE-driven refresh on every surface by simulating check-in/check-out events; confirm fallback polling behavior.
