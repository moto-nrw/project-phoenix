## 1. Planning & Validation
- [ ] 1.1 Review current location badge usages and confirm data sources per surface (identify where SSE vs REST powers updates).
- [ ] 1.2 Align with stakeholders (UX + localisation) on canonical badge labels (German vs English copy), iconography, and gradient tokens; document approval.
- [ ] 1.3 Audit downstream consumers (exports, analytics, admin dashboards) to confirm dependencies on `in_house`, `school_yard`, or `wc` flags and plan communication if removal impacts them.

## 2. Shared Helper Implementation
- [ ] 2.1 Implement shared `getLocationBadge` utility (or ModernStatusBadge update) parsing `current_location` strings and optional room-status payloads.
- [ ] 2.2 Extract color/gradient tokens into a centralized theme map reused by helper/component.
- [ ] 2.3 Add logic unit tests covering every supported state (group room, other room, Schulhof, WC, Bus, Unterwegs, Zuhause, Unknown, `Anwesend - …`, `Anwesend in …`) and sample room name inputs.
- [ ] 2.4 Add Storybook stories or visual regression snapshots for the shared badge states to catch styling drift.
- [ ] 2.5 Implement a feature flag (configurable via Next.js runtime env) to toggle between the legacy badge paths and the shared helper.

## 3. Surface Integration
- [ ] 3.1 Refactor OGS group view to consume the shared helper while retaining SSE refresh; validate with a simulated SSE check-in/out event.
- [ ] 3.2 Update student search page to use the helper, correct room-name labeling, and confirm parity with OGS for identical students.
- [ ] 3.3 Replace My Room group badge with shared location badge; verify SSE updates re-render the badge and cover foreign-room visits.
- [ ] 3.4 Add the shared badge to the student detail modal, sourcing `/current-location`; adopt a periodic re-fetch (e.g., every 30s while open) to stay current and confirm no flicker with existing fetches.
- [ ] 3.5 Exercise the feature flag: document test cases for flag OFF (legacy behaviour) and flag ON (shared helper) across all four surfaces.

## 4. Cleanup & Validation
- [ ] 4.1 Remove deprecated boolean flag dependencies from frontend mapping (`student-helpers`) and UI branches, after confirming no remaining downstream consumers from 1.3; communicate change to affected stakeholders.
- [ ] 4.2 Run `npm run check`, logic/unit tests, Storybook/visual regression (if CI-integrated), and targeted manual QA for the four surfaces, including SSE-triggered updates and feature-flag toggling.
- [ ] 4.3 Document rollout steps, QA notes (including SSE validation results and flag exercise), and prepare PR summary referencing stakeholder approvals, updated tests, and flag configuration guidance.
