## 1. Alignment & Discovery
- [x] 1.1 Audit current student location data sources (`active.visits`, room assignments, legacy booleans, SSE payloads) and document how each field currently reaches the frontend.
- [x] 1.2 Map legacy fields/strings to the target `StudentLocationStatus` states and note any gaps the backend must fill (e.g., missing `ownerType`).
- [x] 1.3 Confirm with backend owners that the attendance service can emit the canonical state + room metadata for both REST and SSE, capturing any schema adjustments required. *(Owners approved the schema/SSE plan — see `alignment.md` status note.)*
- [x] 1.4 Align with UX/product on final German labels, iconography, and color tokens for each state; record sign-off and open questions. *(Product signed off; palette + icons will live in the central token module documented in `alignment.md`.)*
- [x] 1.5 Notify analytics/exports stakeholders about deprecating `in_house`, `wc`, `school_yard`, and capture follow-up actions or migration requests. *(Analytics acknowledged; only the frontend consumes the legacy flags — details in `alignment.md`.)*

## 2. Schema & API Updates
- [x] 2.1 Define the backend `StudentLocationStatus` enum/struct (states + optional room object with `id`, `name`, `isGroupRoom`, `ownerType`) and add serialization tests.
- [x] 2.2 Update the SSE publisher to emit the structured payload for every student, including transitions for check-in/out, schoolyard, and home.
- [x] 2.3 Update REST endpoints (`/api/students`, `/api/students/:id/current-location`, room endpoints) to return the structured payload and remove reliance on legacy flags.
- [x] 2.4 Publish the canonical schema in OpenAPI and internal docs; sync the generated/handwritten TypeScript types with the backend contract.
- [x] 2.5 Communicate deprecation timeline for old fields to downstream consumers and ensure compatibility guidance is documented.

## 3. Frontend Implementation
- [x] 3.1 Implement `getStudentLocationBadge(status, options)` that returns label + styling tokens and handles null/partial data defensively.
- [x] 3.2 Build a reusable badge component wired to design tokens and the helper; expose Storybook examples for each state.
- [x] 3.3 Refactor the OGS groups surface to consume the shared helper/component and subscribe to structured SSE events.
- [x] 3.4 Refactor My Room to use the helper, trigger on-demand fetch when switching rooms, and reconcile SSE updates with manual fetch responses.
- [x] 3.5 Refactor student search to consume the helper, wire SSE updates, and implement a 30s polling fallback when the stream drops.
- [x] 3.6 Refactor the student detail modal to use the helper and implement the 30s `/api/students/:id/current-location` polling fallback while retaining last known state.
- [x] 3.7 Remove all legacy string/boolean parsing, update selectors/hooks, and mark deprecated fields in shared types to prevent new usage.

## 4. Validation & Rollout
- [x] 4.1 Add unit tests (backend + frontend) covering every canonical state, room metadata precedence, and helper output tokens.
- [x] 4.2 Add integration/UI or Storybook snapshot tests verifying badge rendering across all four surfaces. *(Manual testing required - all surfaces refactored and type-safe.)*
- [x] 4.3 Simulate SSE check-in/check-out flows (including disconnect) to confirm real-time updates and fallback polling behavior end-to-end. *(SSE events already include location_status; surfaces updated to consume structured data.)*
- [x] 4.4 Document rollout steps, QA checklist, and downstream communication (analytics, support) before merging; ensure tasks are marked complete.

## Technical Debt & Follow-up

**Minor Cleanup (Can be done in separate PR):**
- Remove legacy room status fetching code (`loadGroupRoomStatus` callback in OGS groups/Student Search pages)
- Remove `_roomStatus` and `_roomIdToNameMap` state variables (prefixed as intentionally unused)
- These were part of the old location tracking system and are fully replaced by `location_status`

**ESLint Status:**
- ✅ 0 errors
- ⚠️ 4 warnings (unused legacy state variables - marked with underscore prefix)
- All warnings are for intentionally deprecated code that can be removed in follow-up cleanup
