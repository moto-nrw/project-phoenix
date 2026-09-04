import { describe, it, expect, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSidebarGroups } from "../use-sidebar-groups";

const STORAGE_KEY = "sidebar-open-groups";

describe("useSidebarGroups", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("opens only Tagesbetrieb on a first visit", () => {
    const { result } = renderHook(() => useSidebarGroups("/dashboard"));
    expect(result.current.openGroups).toEqual(["tagesbetrieb"]);
    expect(result.current.isGroupOpen("tagesbetrieb")).toBe(true);
    expect(result.current.isGroupOpen("eltern")).toBe(false);
  });

  it("opens the group of the current page in addition to the default", () => {
    const { result } = renderHook(() => useSidebarGroups("/dienstplan"));
    expect(result.current.openGroups).toEqual(["tagesbetrieb", "planung"]);
  });

  it("opens the group of the originating page on a student detail page", () => {
    const { result } = renderHook(() =>
      useSidebarGroups("/students/12", "/day-log"),
    );
    expect(result.current.isGroupOpen("verwaltung")).toBe(true);
  });

  it("toggles a group on and off and remembers it", () => {
    const { result } = renderHook(() => useSidebarGroups("/dashboard"));

    act(() => result.current.toggleGroup("eltern"));
    expect(result.current.isGroupOpen("eltern")).toBe(true);
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]")).toEqual([
      "tagesbetrieb",
      "eltern",
    ]);

    act(() => result.current.toggleGroup("tagesbetrieb"));
    expect(result.current.isGroupOpen("tagesbetrieb")).toBe(false);
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]")).toEqual([
      "eltern",
    ]);
  });

  it("restores the stored groups instead of the default", () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(["team", "verwaltung"]));
    const { result } = renderHook(() => useSidebarGroups("/dashboard"));
    expect(result.current.openGroups).toEqual(["team", "verwaltung"]);
  });

  it("keeps the current page's group open even when the stored state closed it", () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(["team"]));
    const { result } = renderHook(() => useSidebarGroups("/messages"));
    expect(result.current.openGroups).toEqual(["team", "eltern"]);
  });

  it("never overwrites the stored state with the default on mount", () => {
    // Reacts doppeltes Einhängen im Entwicklungsmodus: der erste Durchlauf
    // darf den Speicher nicht mit dem Standard überschreiben, sonst liest
    // der zweite nur noch ["tagesbetrieb"].
    localStorage.setItem(STORAGE_KEY, JSON.stringify(["team"]));
    const first = renderHook(() => useSidebarGroups("/dashboard"));
    first.unmount();
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]")).toEqual([
      "team",
    ]);

    const second = renderHook(() => useSidebarGroups("/dashboard"));
    expect(second.result.current.openGroups).toEqual(["team"]);
  });

  it("ignores unknown or malformed stored values", () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(["planning", 3, null]));
    const { result } = renderHook(() => useSidebarGroups("/dashboard"));
    expect(result.current.openGroups).toEqual([]);

    localStorage.setItem(STORAGE_KEY, "not json");
    const second = renderHook(() => useSidebarGroups("/dashboard"));
    expect(second.result.current.openGroups).toEqual(["tagesbetrieb"]);
  });

  it("opens the group of a newly visited page without closing the others", () => {
    const { result, rerender } = renderHook(
      ({ pathname }) => useSidebarGroups(pathname),
      { initialProps: { pathname: "/dashboard" } },
    );
    act(() => result.current.toggleGroup("eltern"));

    rerender({ pathname: "/time-tracking" });
    expect(result.current.openGroups).toEqual([
      "tagesbetrieb",
      "eltern",
      "team",
    ]);

    rerender({ pathname: "/dashboard" });
    expect(result.current.openGroups).toEqual([
      "tagesbetrieb",
      "eltern",
      "team",
    ]);
  });
});
