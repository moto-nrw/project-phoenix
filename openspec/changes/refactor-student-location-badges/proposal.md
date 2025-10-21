## Why
- Location badges diverge across OGS groups (SSE + room-status endpoint), student search (manual fetch only), My Room (SSE + visits API), and the student detail modal (REST + current-location API). The same student can show different labels, gradients, or no badge at all, which erodes trust in location accuracy.
- Deprecated boolean flags (`in_house`, `school_yard`, `wc`) still drive UI logic even though the mapper auto-populates or disables them; downstream tools (exports, analytics) may also rely on them, so we must retire them with an explicit audit and communication plan.
- My Room and the student detail modal omit location status altogether despite fetching the necessary data, reducing supervisors' situational awareness and creating parity gaps with the OGS and search surfaces.

## What Changes
- Introduce a shared location badge helper/component that maps `student.current_location` and optional room metadata into unified labels, styling tokens, and gradients, covering every canonical state (group room, other room, Schulhof, WC, Bus, Unterwegs, Zuhause, Unknown).
- Update OGS groups (SSE), student search (fetch polling), My Room (SSE), and the student detail modal (REST + periodic 30-second re-fetch) to rely on the shared helper so real-time streams and static payloads render the same state; document any SSE vs REST freshness differences and provide validation steps.
- Normalize state detection by parsing the rich `current_location` string (supporting `Anwesend - …`, `Anwesend in …`, `Bus`, `WC`, etc.) and remove dependencies on deprecated boolean flags after auditing external consumers and coordinating with any affected owners.
- Provide regression coverage (logic unit tests plus visual snapshots/Storybook stories) that lock down badge output for each supported state and ensure SSE updates remain visible.

## Impact
- UX: Consistent badge labels and visuals everywhere, plus restored visibility in My Room and the detail modal.
- DX: Single source of truth for location styling, simplifying future tweaks.
- Risk: Requires careful mapping changes, SSE vs REST sanity checks, boolean deprecation across other tooling, and a guarded rollout; mitigated through documented validation (SSE simulation, stakeholder sign-off) and a feature flag that can revert to legacy badges if issues surface.
- Dependencies: Coordination with frontend UX (badge copy/colors), backend/data owners (confirm boolean retirement impact), QA for SSE regression coverage, and Release Engineering to host the rollout flag; backend changes only if additional location data is needed (not expected).
