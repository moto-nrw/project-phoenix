## Why
Success notifications are implemented inconsistently across pages (local stateful `<SimpleAlert />` usages, inline banners, and missing toasts in some success flows). This leads to uneven UX, duplicated code, and positioning differences between mobile and desktop.

## What Changes
- Add a global, accessible success notification system with a single provider and hook.
- Standardize placement: bottom-right on desktop; full-width near bottom on mobile with safe-area padding.
- Unify visuals (icon, colors, spacing, animation) via theme tokens.
- Replace page-level success alert state with `toast.success(message)` calls.
- Add de-duplication and stacking behavior (max 3 concurrent).
- Ensure a11y with `role="status"`, `aria-live="polite"`, and motion preferences.

## Impact
- Affected specs: `ui-notifications` (new capability)
- Affected code (indicative, not exhaustive):
  - frontend/src/components/simple/SimpleAlert.tsx
  - frontend/src/contexts/AlertContext.tsx
  - frontend/src/app/providers.tsx
  - Pages using `<SimpleAlert />`, e.g.:
    - frontend/src/components/ui/database/database-page.tsx:681
    - frontend/src/app/profile/page.tsx:512
    - frontend/src/app/settings/page.tsx:749
    - frontend/src/app/database/devices/page.tsx:299
    - frontend/src/app/database/rooms/page.tsx:405
    - frontend/src/components/teachers/teacher-permission-management-modal.tsx:390
  - Hooks / helpers that prepare success messages (e.g., `getDbOperationMessage`)

## Non-Goals
- Changing backend response shapes or messages.
- Converting persistent, contextual inline confirmations that should remain in the page body.

## Risks / Mitigations
- Risk: Navigation immediately hides newly shown toasts. Mitigation: show before navigation; optionally allow a small delay or prefetch.
- Risk: Double notifications (inline + toast). Mitigation: replace local alert instances; add de-dupe window.

## Rollout Plan
1) Implement provider and hook with theme + a11y.
2) Migrate top-traffic pages to use `toast.success`.
3) Sweep remaining pages (tracked in tasks) and remove dead alert states.
4) QA on mobile and desktop; validate a11y and motion preferences.

