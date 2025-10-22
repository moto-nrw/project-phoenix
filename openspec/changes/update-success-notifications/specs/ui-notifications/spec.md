## ADDED Requirements
### Requirement: Unified Success Notifications
The system SHALL present success notifications consistently across all pages using a single global toast mechanism.

#### Scenario: Desktop placement
- WHEN a user action succeeds on a desktop viewport (≥ md breakpoint)
- THEN a success toast appears in the bottom-right corner
- AND multiple success toasts stack upward with a small gap

#### Scenario: Mobile placement
- WHEN a user action succeeds on a mobile viewport (< md breakpoint)
- THEN a success toast appears near the bottom edge
- AND it spans the width minus safe-area padding on both sides

#### Scenario: Visual consistency
- WHEN a success toast is shown
- THEN it uses the project theme success colors and iconography
- AND uses consistent padding, border radius, and shadow

#### Scenario: Duration and interaction
- WHEN a success toast is shown
- THEN it auto-dismisses after 3000ms by default
- AND it provides a visible close affordance to dismiss immediately
- AND on desktop, hovering pauses auto-dismiss

#### Scenario: Stacking and de-duplication
- GIVEN multiple success events occur close together
- WHEN toasts are enqueued
- THEN up to 3 are visible concurrently in a stack
- AND identical messages within 2 seconds are de-duplicated

#### Scenario: Accessibility
- WHEN a success toast is shown
- THEN it is announced by assistive technologies via role="status" and aria-live="polite"
- AND the announcement is atomic (aria-atomic="true")
- AND keyboard users can focus the close control to dismiss

#### Scenario: Reduced motion support
- GIVEN the user prefers reduced motion
- WHEN a success toast is shown
- THEN animations and progress transitions are disabled

