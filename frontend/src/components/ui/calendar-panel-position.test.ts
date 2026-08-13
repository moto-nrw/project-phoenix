import { describe, it, expect } from "vitest";

import {
  CALENDAR_PANEL_MARGIN,
  clampCalendarWidth,
  computeCalendarPanelPosition,
} from "./calendar-panel-position";

const VIEWPORT = { width: 1440, height: 900 };

describe("clampCalendarWidth", () => {
  it("keeps day buttons legible under narrow triggers like the w-44 settings field", () => {
    // A 176px trigger must not produce a 176px panel: after the compact card
    // padding (12px/side), the borders and the 8px per-cell gutter, that left
    // ~13px per day button. The floor guarantees a 20px button.
    const width = clampCalendarWidth(176, VIEWPORT.width);
    const dayButton = (width - 2 * 12 - 2 * 1) / 7 - 8;
    expect(dayButton).toBeGreaterThanOrEqual(20);
  });

  it("keeps the complete header visible under an icon-only trigger", () => {
    expect(clampCalendarWidth(40, VIEWPORT.width)).toBe(304);
  });

  it("shrinks the panel to the available width below 320px", () => {
    expect(clampCalendarWidth(40, 280)).toBe(264);
  });

  it("still matches the trigger width when the trigger is wide enough", () => {
    expect(clampCalendarWidth(380, VIEWPORT.width)).toBe(380);
  });
});

describe("computeCalendarPanelPosition", () => {
  it("clamps the left-flush position to the viewport margin for a trigger hanging off the left edge", () => {
    const geometry = computeCalendarPanelPosition(
      { top: 100, bottom: 140, left: -40, right: 184 },
      224,
      VIEWPORT,
    );
    expect(geometry.left).toBe(CALENDAR_PANEL_MARGIN);
  });

  it("keeps an on-screen trigger left-flush", () => {
    const geometry = computeCalendarPanelPosition(
      { top: 100, bottom: 140, left: 120, right: 344 },
      224,
      VIEWPORT,
    );
    expect(geometry.left).toBe(120);
  });
});
