/**
 * Geometry every floating panel of the kit shares: select menus, calendars and
 * anchored popovers keep the same distance from the viewport edge and from
 * their trigger. Each panel used to carry its own copy of these numbers.
 */

/** Distance a panel keeps from every viewport edge. */
export const PANEL_VIEWPORT_MARGIN = 8;

/** Distance between the trigger and the panel opened from it. */
export const PANEL_GAP = 4;

/** Room below and above `anchor` for a panel, after gap and margin. */
export function spaceAroundAnchor(
  anchor: { readonly top: number; readonly bottom: number },
  viewportHeight: number,
): { readonly below: number; readonly above: number } {
  return {
    below: viewportHeight - anchor.bottom - PANEL_GAP - PANEL_VIEWPORT_MARGIN,
    above: anchor.top - PANEL_GAP - PANEL_VIEWPORT_MARGIN,
  };
}

/**
 * Clamps the start offset (left, or right for an end-anchored panel) of a
 * panel of `size` so its far edge stays inside the viewport margin. A panel
 * wider than the viewport allows keeps the margin at its start.
 */
export function clampPanelStart(
  start: number,
  size: number,
  viewportSize: number,
): number {
  return Math.min(
    Math.max(start, PANEL_VIEWPORT_MARGIN),
    Math.max(
      viewportSize - size - PANEL_VIEWPORT_MARGIN,
      PANEL_VIEWPORT_MARGIN,
    ),
  );
}
