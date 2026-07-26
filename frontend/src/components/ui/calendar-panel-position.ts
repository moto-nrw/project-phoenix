/**
 * One geometry rule for every calendar panel in the kit.
 *
 * Before this existed, each picker positioned itself differently: the single
 * date picker left-aligned with a content-driven width, the range picker
 * right-aligned via `sm:right-0`. Opening two date fields on the same screen
 * showed two different behaviours, and neither guaranteed a flush edge.
 *
 * The rule:
 *   1. A panel that can adapt takes the trigger's width, so the calendar's
 *      edges line up with the field's edges. The only limits are physical: a
 *      legible minimum day-cell size, and the viewport.
 *   2. A panel that cannot adapt (the two-month range calendar with its preset
 *      column) keeps its content width.
 *   3. The panel's LEFT edge sits on the trigger's left edge. If that would
 *      push it past the viewport, it flips so its RIGHT edge sits on the
 *      trigger's right edge. Only if neither fits does it clamp to the
 *      viewport — so in practice one edge is always flush with the field.
 */

export const CALENDAR_PANEL_MARGIN = 8;

/**
 * Narrowest a stretchable calendar gets: 7 × (20px day button + 8px cell
 * gutter) + 12px compact card padding per side + the 1px border. The previous
 * 176px floor was derived from the 20px *cell*, forgetting that the px-1
 * gutter lives inside the cell — which left the clickable day button at ~13px,
 * too narrow for two-digit dates. 20px is what the BUTTON needs.
 */
const CALENDAR_MIN_WIDTH = 222;

/**
 * Height used for the flip-up decision: the worst case, a six-week month.
 * Measured at 316px for five rows with 36px cells, so a sixth row plus its gap
 * lands just under 360. Underestimating it would let the panel open downward
 * and run off the bottom of the viewport.
 */
const CALENDAR_PANEL_HEIGHT = 360;

export interface PanelGeometry {
  readonly top: number;
  readonly left: number;
  readonly width: number;
}

/**
 * Width a stretchable panel adopts under a trigger of `triggerWidth`.
 *
 * The panel matches the field, full stop — there is no taste-based ceiling. The
 * only bounds are physical: it cannot be narrower than a legible day grid, and
 * it cannot be wider than the viewport.
 */
export function clampCalendarWidth(
  triggerWidth: number,
  viewportWidth: number,
): number {
  const available = viewportWidth - 2 * CALENDAR_PANEL_MARGIN;
  return Math.min(Math.max(CALENDAR_MIN_WIDTH, triggerWidth), available);
}

/**
 * Places a panel of `width` under `trigger`.
 *
 * `width` is what the caller will actually apply — pass the clamped width for a
 * stretchable calendar, or the measured content width for one that cannot
 * shrink. Passing a width the panel does not really have is what previously
 * pulled the card 22px left of its own trigger.
 */
export function computeCalendarPanelPosition(
  trigger: { top: number; bottom: number; left: number; right: number },
  width: number,
  viewport: { width: number; height: number },
  height = CALENDAR_PANEL_HEIGHT,
): PanelGeometry {
  let top = trigger.bottom + 4;
  if (top + height > viewport.height - CALENDAR_PANEL_MARGIN) {
    const above = trigger.top - 4 - height;
    top =
      above >= CALENDAR_PANEL_MARGIN
        ? above
        : Math.max(
            CALENDAR_PANEL_MARGIN,
            viewport.height - CALENDAR_PANEL_MARGIN - height,
          );
  }

  const maxLeft = viewport.width - CALENDAR_PANEL_MARGIN - width;
  // Left-flush is the default. When it overflows, right-flush with the trigger
  // keeps an edge aligned instead of drifting to an arbitrary clamped offset.
  // A trigger hanging past the left viewport edge must not drag the panel with
  // it — an unclamped negative left puts the first day columns off-screen.
  let left = Math.max(CALENDAR_PANEL_MARGIN, trigger.left);
  if (left > maxLeft) {
    const rightFlush = trigger.right - width;
    left =
      rightFlush >= CALENDAR_PANEL_MARGIN
        ? rightFlush
        : Math.max(CALENDAR_PANEL_MARGIN, maxLeft);
  }

  return { top, left, width };
}
