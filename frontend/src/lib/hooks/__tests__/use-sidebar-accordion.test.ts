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

  it("expands the groups section for /ogs-groups", () => {
    const { result } = renderHook(() => useSidebarAccordion("/ogs-groups"));
    expect(result.current.expanded).toBe("groups");
  });

  it("expands 'supervisions' section for /active-supervisions path", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/active-supervisions"),
    );
    expect(result.current.expanded).toBe("supervisions");
  });

  it("expands 'database' section for /database path", () => {
    const { result } = renderHook(() => useSidebarAccordion("/database"));
    expect(result.current.expanded).toBe("database");
  });

  it("returns null for unrelated paths", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));
    expect(result.current.expanded).toBe(null);
  });

  it("expands groups from fromParam on child pages", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/students/123", "/ogs-groups"),
    );
    expect(result.current.expanded).toBe("groups");
  });

  it("expands 'supervisions' from fromParam on child pages", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/students/123", "/active-supervisions"),
    );
    expect(result.current.expanded).toBe("supervisions");
  });

  it("expands 'database' from fromParam on child pages", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/students/123", "/database"),
    );
    expect(result.current.expanded).toBe("database");
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

  it("expands 'enrollments' section for enrollment paths", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/admin/enrollments"),
    );
    expect(result.current.expanded).toBe("enrollments");
  });

  it("expands 'enrollments' from fromParam on child pages", () => {
    const { result } = renderHook(() =>
      useSidebarAccordion("/students/123", "/care-offerings"),
    );
    expect(result.current.expanded).toBe("enrollments");
  });

  it("expands 'planning' for all planning paths incl. legacy redirects (#1946)", () => {
    // Betreuungsplan, Dienstplan, Vertretung und Kalenderzeiträume sind
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

  it("restores 'enrollments' from localStorage when pathname does not determine section", () => {
    localStorageMock.getItem.mockReturnValueOnce("enrollments");

    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    expect(result.current.expanded).toBe("enrollments");
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
      result.current.toggle("supervisions");
    });
    expect(result.current.expanded).toBe("supervisions");

    act(() => {
      result.current.toggle("supervisions");
    });
    expect(result.current.expanded).toBe(null);
  });

  it("switches between sections exclusively", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    act(() => {
      result.current.toggle("supervisions");
    });
    expect(result.current.expanded).toBe("supervisions");

    act(() => {
      result.current.toggle("database");
    });
    expect(result.current.expanded).toBe("database");
  });

  it("persists expanded section to localStorage", () => {
    renderHook(() => useSidebarAccordion("/active-supervisions"));
    expect(localStorageMock.setItem).toHaveBeenCalledWith(
      "sidebar-accordion-expanded",
      "supervisions",
    );
  });

  it("removes from localStorage when collapsed", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    act(() => {
      result.current.toggle("supervisions");
    });

    act(() => {
      result.current.toggle("supervisions");
    });

    expect(localStorageMock.removeItem).toHaveBeenCalledWith(
      "sidebar-accordion-expanded",
    );
  });

  it("restores from localStorage when pathname does not determine section", () => {
    localStorageMock.getItem.mockReturnValueOnce("supervisions");

    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    // After useEffect runs, it should restore from localStorage
    expect(result.current.expanded).toBe("supervisions");
  });

  it("does not restore from localStorage when pathname determines section", () => {
    localStorageMock.getItem.mockReturnValueOnce("database");

    const { result } = renderHook(() =>
      useSidebarAccordion("/active-supervisions"),
    );

    // Should use pathname, not localStorage
    expect(result.current.expanded).toBe("supervisions");
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

    rerender({ pathname: "/ogs-groups" });
    expect(result.current.expanded).toBe("groups");

    rerender({ pathname: "/active-supervisions" });
    expect(result.current.expanded).toBe("supervisions");
  });

  it("collapses when navigating to unrelated page", () => {
    const { result, rerender } = renderHook(
      ({ pathname }) => useSidebarAccordion(pathname),
      { initialProps: { pathname: "/active-supervisions" } },
    );

    expect(result.current.expanded).toBe("supervisions");

    rerender({ pathname: "/dashboard" });
    expect(result.current.expanded).toBe(null);
  });
});
