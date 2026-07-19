import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { useSidebarAccordion } from "./use-sidebar-accordion";

// The hook derives the open accordion section purely from the pathname (plus an
// optional `from` param) and persists it to localStorage. #1946 adds the
// "planning" section (Betreuungsplan / Dienstplan / Vertretung /
// Kalenderzeiträume + their legacy redirect frames).
describe("useSidebarAccordion — planning section (#1946)", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it.each([
    "/betreuungsplan",
    "/dienstplan",
    "/vertretung",
    "/calendar-periods",
    "/timetables",
    "/vertretungsplan",
    "/staff/dienstplan",
  ])("auto-expands planning on %s (AC3)", (pathname) => {
    const { result } = renderHook(() => useSidebarAccordion(pathname));
    expect(result.current.expanded).toBe("planning");
  });

  it("keeps planning open for a child page via the from param", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/students/5", "/dienstplan"),
    );
    expect(result.current.expanded).toBe("planning");
  });

  it("does not open planning on unrelated routes", () => {
    const { result } = renderHook(() => useSidebarAccordion("/staff"));
    expect(result.current.expanded).toBeNull();
  });

  it("persists planning to localStorage and restores it on reload", () => {
    // Navigate to a planning page → section derived + persisted.
    renderHook(() => useSidebarAccordion("/betreuungsplan"));
    expect(localStorage.getItem("sidebar-accordion-expanded")).toBe("planning");

    // Reload on a route that does not itself decide a section: the stored
    // "planning" value is restored on mount (AC3 — reload keeps the section).
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));
    expect(result.current.expanded).toBe("planning");
  });

  it("toggles the planning section closed and open again", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));
    expect(result.current.expanded).toBeNull();

    act(() => result.current.toggle("planning"));
    expect(result.current.expanded).toBe("planning");

    act(() => result.current.toggle("planning"));
    expect(result.current.expanded).toBeNull();
  });
});
