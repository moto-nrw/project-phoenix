## Context
We need a single, accessible, theme-aligned success notification experience with consistent placement and behavior across desktop and mobile. The current approach duplicates per-page alert state and visuals via `<SimpleAlert />` and sometimes omits toasts entirely.

## Goals / Non-Goals
- Goals: central provider, consistent UI rules, minimal API (`toast.success`), easy migration.
- Non-Goals: backend changes; persistent inline banners that are context-specific.

## Decisions
- Introduce a `ToastProvider` that renders a stack container once at app root and exposes a `useToast()` hook.
- Reuse/refactor `SimpleAlert` visuals into a `ToastItem` component to minimize churn.
- Positioning: bottom-right (desktop), full-width near bottom with safe-area (mobile).
- Stack limit: 3; de-dup window: 2s; default duration: 3000ms; pause on hover (desktop).
- Accessibility: `role="status"`, `aria-live="polite"`, `aria-atomic="true"`; keyboard-close.
- Motion preferences: detect `prefers-reduced-motion` and disable animations/progress.

## Alternatives Considered
- External libraries (e.g., react-hot-toast): rejected to avoid dependency weight and styling mismatches; current visuals are adequate and already implemented.

## Risks / Trade-offs
- Navigation may hide toast instantly; we’ll show the toast before navigation and may add a short delay where necessary.
- Temporary duplication (old alerts and new toasts) during migration; mitigated by tracking pages in tasks.

## Migration Plan
1) Implement provider + hook with minimal internal API.
2) Switch top-priority flows to `toast.success`.
3) Remove local alert state and components where replaced.
4) Sweep remaining pages and verify a11y and responsiveness.

