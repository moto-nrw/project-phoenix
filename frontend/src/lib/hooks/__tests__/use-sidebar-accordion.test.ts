import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSidebarAccordion } from "../use-sidebar-accordion";

// Mock localStorage with accessible store for test isolation
let store: Record<string, string> = {};

const localStorageMock = {
  getItem: vi.fn((key: string) => store[key] ?? null),
  setItem: vi.fn((key: string, value: string) => {
    store[key] = value;
  }),
  removeItem: vi.fn((key: string) => {
    delete store[key];
  }),
  clear: vi.fn(() => {
    store = {};
  }),
};

Object.defineProperty(window, "localStorage", { value: localStorageMock });

describe("useSidebarAccordion", () => {
  beforeEach(() => {
    store = {};
    // mockReset clears the mockReturnValueOnce queue (vi.clearAllMocks does NOT),
    // then re-attach the store-backed implementations.
    localStorageMock.getItem
      .mockReset()
      .mockImplementation((key: string) => store[key] ?? null);
    localStorageMock.setItem
      .mockReset()
      .mockImplementation((key: string, value: string) => {
        store[key] = value;
      });
    localStorageMock.removeItem
      .mockReset()
      .mockImplementation((key: string) => {
        delete store[key];
      });
  });

  it("returns null for unrelated paths", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));
    expect(result.current.expanded).toBe(null);
  });

  it("expands 'eltern' section for /eltern hub path", () => {
    const { result } = renderHook(() => useSidebarAccordion("/eltern"));
    expect(result.current.expanded).toBe("eltern");
  });

  it("expands 'eltern' section for parent sub-pages", () => {
    // /admin/change-requests gehört seit #2429 nicht mehr zum Eltern-Bereich —
    // die Route ist nur noch ein Redirect auf das Top-Level-Modul /anfragen.
    for (const path of [
      "/messages",
      "/admin/guardian-approvals",
      "/parent-announcements",
      "/meal-plan",
    ]) {
      const { result } = renderHook(() => useSidebarAccordion(path));
      expect(result.current.expanded).toBe("eltern");
    }
  });

  it("expands 'eltern' from fromParam on child pages", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/students/123", "/messages"),
    );
    expect(result.current.expanded).toBe("eltern");
  });

  it("expands 'eltern' for the enrollment block", () => {
    // Die Anmeldungen sind seit dem Navigationsumbau ein Block im
    // Eltern-Bereich, kein eigener Bereich mehr.
    const { result } = renderHook(() =>
      useSidebarAccordion("/admin/enrollments"),
    );
    expect(result.current.expanded).toBe("eltern");
  });

  it("expands 'eltern' from an enrollment fromParam on child pages", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/students/123", "/care-offerings"),
    );
    expect(result.current.expanded).toBe("eltern");
  });

  it("expands 'team' and 'reports' for their sections", () => {
    expect(
      renderHook(() => useSidebarAccordion("/team-chat")).result.current
        .expanded,
    ).toBe("team");
    expect(
      renderHook(() => useSidebarAccordion("/dateien")).result.current.expanded,
    ).toBe("team");
    expect(
      renderHook(() => useSidebarAccordion("/statistics")).result.current
        .expanded,
    ).toBe("reports");
    expect(
      renderHook(() => useSidebarAccordion("/payroll")).result.current.expanded,
    ).toBe("reports");
  });

  it("expands 'planning' for all planning paths incl. legacy redirects (#1946)", () => {
    // Betreuungsplan, Dienstplan, Vertretung und Zeiträume sind
    // Unterpunkte des Planung-Akkordeons; die Redirect-Stubs zählen dazu.
    for (const path of [
      "/calendar-periods",
      "/timetables",
      "/staff/dienstplan",
      "/betreuungsplan",
      "/dienstplan",
      "/vertretung",
      "/vertretungsplan",
    ]) {
      const { result } = renderHook(() => useSidebarAccordion(path));
      expect(result.current.expanded).toBe("planning");
    }
  });

  it("restores a stored 'planning' value from localStorage", () => {
    localStorageMock.getItem.mockReturnValueOnce("planning");

    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    expect(result.current.expanded).toBe("planning");
  });

  it("restores 'eltern' from localStorage when pathname does not determine section", () => {
    localStorageMock.getItem.mockReturnValueOnce("eltern");

    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    expect(result.current.expanded).toBe("eltern");
  });

  it("returns null when fromParam is unrelated", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/students/123", "/dashboard"),
    );
    expect(result.current.expanded).toBe(null);
  });

  it("toggles section on and off", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    act(() => {
      result.current.toggle("team");
    });
    expect(result.current.expanded).toBe("team");

    act(() => {
      result.current.toggle("team");
    });
    expect(result.current.expanded).toBe(null);
  });

  it("switches between sections exclusively", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    act(() => {
      result.current.toggle("team");
    });
    expect(result.current.expanded).toBe("team");

    act(() => {
      result.current.toggle("reports");
    });
    expect(result.current.expanded).toBe("reports");
  });

  it("persists expanded section to localStorage", () => {
    renderHook(() => useSidebarAccordion("/team-chat"));
    expect(localStorageMock.setItem).toHaveBeenCalledWith(
      "sidebar-accordion-expanded",
      "team",
    );
  });

  it("removes from localStorage when collapsed", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    act(() => {
      result.current.toggle("team");
    });

    act(() => {
      result.current.toggle("team");
    });

    expect(localStorageMock.removeItem).toHaveBeenCalledWith(
      "sidebar-accordion-expanded",
    );
  });

  it("restores from localStorage when pathname does not determine section", () => {
    localStorageMock.getItem.mockReturnValueOnce("reports");

    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    // After useEffect runs, it should restore from localStorage
    expect(result.current.expanded).toBe("reports");
  });

  it("does not restore from localStorage when pathname determines section", () => {
    localStorageMock.getItem.mockReturnValueOnce("reports");

    const { result } = renderHook(() => useSidebarAccordion("/team-chat"));

    // Should use pathname, not localStorage
    expect(result.current.expanded).toBe("team");
  });

  it("ignores invalid localStorage values", () => {
    localStorageMock.getItem.mockReturnValueOnce("invalid-section");

    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    expect(result.current.expanded).toBe(null);
  });

  it("auto-expands when pathname changes", () => {
    const { result, rerender } = renderHook(
      ({ pathname }) => useSidebarAccordion(pathname),
      { initialProps: { pathname: "/dashboard" } },
    );

    expect(result.current.expanded).toBe(null);

    rerender({ pathname: "/team-chat" });
    expect(result.current.expanded).toBe("team");

    rerender({ pathname: "/statistics" });
    expect(result.current.expanded).toBe("reports");
  });

  it("collapses when navigating to unrelated page", () => {
    const { result, rerender } = renderHook(
      ({ pathname }) => useSidebarAccordion(pathname),
      { initialProps: { pathname: "/team-chat" } },
    );

    expect(result.current.expanded).toBe("team");

    rerender({ pathname: "/dashboard" });
    expect(result.current.expanded).toBe(null);
  });
});
