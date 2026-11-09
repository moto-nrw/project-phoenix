import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { releaseFakeTimers } from "~/test/clock";

import { useSheetAboveKeyboard } from "./useSheetAboveKeyboard";

describe("useSheetAboveKeyboard", () => {
  const originalViewport = window.visualViewport;
  const originalInnerHeight = window.innerHeight;

  const mockViewport = (height: number, offsetTop: number, scale = 1) => {
    window.innerHeight = 812;
    const viewport = new EventTarget();
    Object.defineProperty(viewport, "height", { value: height });
    Object.defineProperty(viewport, "offsetTop", { value: offsetTop });
    Object.defineProperty(viewport, "scale", { value: scale });
    Object.defineProperty(window, "visualViewport", {
      value: viewport,
      configurable: true,
    });
  };

  /** The sheet as Vaul leaves it: lifted by the keyboard height. */
  const sheetAfterFocus = (active = true) => {
    const sheet = document.createElement("div");
    sheet.style.bottom = "396px";
    renderHook(() => useSheetAboveKeyboard({ current: sheet }, active));
    vi.advanceTimersByTime(1000);
    return sheet;
  };

  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    releaseFakeTimers();
    window.innerHeight = originalInnerHeight;
    Object.defineProperty(window, "visualViewport", {
      value: originalViewport,
      configurable: true,
    });
  });

  it("drops the lift when the screen already panned by the keyboard height", () => {
    // Installed iPhone app: keyboard 396 px, visual viewport panned by 396 px.
    mockViewport(416, 396);

    expect(sheetAfterFocus().style.bottom).toBe("0px");
  });

  it("keeps the lift when the screen did not pan", () => {
    mockViewport(416, 0);

    expect(sheetAfterFocus().style.bottom).toBe("396px");
  });

  it("leaves a pinch zoom alone", () => {
    mockViewport(406, 200, 2);

    expect(sheetAfterFocus().style.bottom).toBe("396px");
  });

  it("leaves side panels alone", () => {
    mockViewport(416, 396);

    expect(sheetAfterFocus(false).style.bottom).toBe("396px");
  });
});
