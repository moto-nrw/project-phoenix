import { describe, it, expect, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSidebarGroups } from "../use-sidebar-groups";

const STORAGE_KEY = "sidebar-open-groups";
// Die Voreinstellung kommt seit #6670 je Rolle von außen; die Fälle unten
// prüfen das Verhalten des Hooks mit einer festen Voreinstellung.
const DEFAULT = ["tagesbetrieb"] as const;

describe("useSidebarGroups", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("opens only the default groups on a first visit", () => {
    const { result } = renderHook(() =>
      useSidebarGroups("/dashboard", null, DEFAULT),
    );
    expect(result.current.openGroups).toEqual(["tagesbetrieb"]);
    expect(result.current.isGroupOpen("tagesbetrieb")).toBe(true);
    expect(result.current.isGroupOpen("eltern")).toBe(false);
  });

  it("opens the group of the current page in addition to the default", () => {
    const { result } = renderHook(() =>
      useSidebarGroups("/dienstplan", null, DEFAULT),
    );
    expect(result.current.openGroups).toEqual(["tagesbetrieb", "planung"]);
  });

  it("opens the group of the originating page on a student detail page", () => {
    const { result } = renderHook(() =>
      useSidebarGroups("/students/12", "/day-log", DEFAULT),
    );
    expect(result.current.isGroupOpen("verwaltung")).toBe(true);
  });

  it("toggles a group on and off and remembers it", () => {
    const { result } = renderHook(() =>
      useSidebarGroups("/dashboard", null, DEFAULT),
    );

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
    const { result } = renderHook(() =>
      useSidebarGroups("/dashboard", null, DEFAULT),
    );
    expect(result.current.openGroups).toEqual(["team", "verwaltung"]);
  });

  it("keeps the current page's group open even when the stored state closed it", () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(["team"]));
    const { result } = renderHook(() =>
      useSidebarGroups("/messages", null, DEFAULT),
    );
    expect(result.current.openGroups).toEqual(["team", "eltern"]);
  });

  it("never overwrites the stored state with the default on mount", () => {
    // Reacts doppeltes Einhängen im Entwicklungsmodus: der erste Durchlauf
    // darf den Speicher nicht mit dem Standard überschreiben, sonst liest
    // der zweite nur noch ["tagesbetrieb"].
    localStorage.setItem(STORAGE_KEY, JSON.stringify(["team"]));
    const first = renderHook(() =>
      useSidebarGroups("/dashboard", null, DEFAULT),
    );
    first.unmount();
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]")).toEqual([
      "team",
    ]);

    const second = renderHook(() =>
      useSidebarGroups("/dashboard", null, DEFAULT),
    );
    expect(second.result.current.openGroups).toEqual(["team"]);
  });

  it("ignores unknown or malformed stored values", () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(["planning", 3, null]));
    const { result } = renderHook(() =>
      useSidebarGroups("/dashboard", null, DEFAULT),
    );
    expect(result.current.openGroups).toEqual([]);

    localStorage.setItem(STORAGE_KEY, "not json");
    const second = renderHook(() =>
      useSidebarGroups("/dashboard", null, DEFAULT),
    );
    expect(second.result.current.openGroups).toEqual(["tagesbetrieb"]);
  });

  it("opens the group of a newly visited page without closing the others", () => {
    const { result, rerender } = renderHook(
      ({ pathname }) => useSidebarGroups(pathname, null, DEFAULT),
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

  it("follows a changed default until the person toggles a group (#6670)", () => {
    // Die Sitzung lädt nach: erst gilt die Voreinstellung ohne Leitung, dann
    // die der Leitung. Gespeichert wird dabei nichts.
    const { result, rerender } = renderHook(
      ({ defaults }) => useSidebarGroups("/dashboard", null, defaults),
      {
        initialProps: {
          defaults: ["tagesbetrieb", "team"] as readonly (
            "tagesbetrieb" | "team" | "planung"
          )[],
        },
      },
    );
    expect(result.current.openGroups).toEqual(["tagesbetrieb", "team"]);

    rerender({ defaults: ["tagesbetrieb", "planung"] });
    expect(result.current.openGroups).toEqual(["tagesbetrieb", "planung"]);
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();

    act(() => result.current.toggleGroup("eltern"));
    rerender({ defaults: ["tagesbetrieb", "team"] });
    expect(result.current.openGroups).toEqual([
      "tagesbetrieb",
      "planung",
      "eltern",
    ]);
  });

  // Seitenleiste und Mehr-Menü (#7048) hängen beide ein und teilen einen
  // Stand: Was die eine aufklappt, steht in der anderen offen, und keine
  // überschreibt den Speicher mit einem veralteten Stand.
  it("keeps every mounted instance on the same state", () => {
    const sidebar = renderHook(() =>
      useSidebarGroups("/dashboard", null, DEFAULT),
    );
    const moreMenu = renderHook(() =>
      useSidebarGroups("/dashboard", null, DEFAULT),
    );

    act(() => sidebar.result.current.toggleGroup("planung"));
    expect(moreMenu.result.current.isGroupOpen("planung")).toBe(true);

    act(() => moreMenu.result.current.toggleGroup("tagesbetrieb"));
    expect(sidebar.result.current.openGroups).toEqual(["planung"]);
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]")).toEqual([
      "planung",
    ]);
  });
});
