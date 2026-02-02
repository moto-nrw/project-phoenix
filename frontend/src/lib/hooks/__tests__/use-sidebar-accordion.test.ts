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

  it("expands 'groups' section for /ogs-groups path", () => {
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

  it("expands 'groups' from fromParam on child pages", () => {
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

  it("toggles section on and off", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    act(() => {
      result.current.toggle("groups");
    });
    expect(result.current.expanded).toBe("groups");

    act(() => {
      result.current.toggle("groups");
    });
    expect(result.current.expanded).toBe(null);
  });

  it("switches between sections exclusively", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    act(() => {
      result.current.toggle("groups");
    });
    expect(result.current.expanded).toBe("groups");

    act(() => {
      result.current.toggle("database");
    });
    expect(result.current.expanded).toBe("database");
  });

  it("persists expanded section to localStorage", () => {
    renderHook(() => useSidebarAccordion("/ogs-groups"));
    expect(localStorageMock.setItem).toHaveBeenCalledWith(
      "sidebar-accordion-expanded",
      "groups",
    );
  });

  it("removes from localStorage when collapsed", () => {
    const { result } = renderHook(() => useSidebarAccordion("/dashboard"));

    act(() => {
      result.current.toggle("groups");
    });

    act(() => {
      result.current.toggle("groups");
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

    const { result } = renderHook(() => useSidebarAccordion("/ogs-groups"));

    // Should use pathname, not localStorage
    expect(result.current.expanded).toBe("groups");
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
      { initialProps: { pathname: "/ogs-groups" } },
    );

    expect(result.current.expanded).toBe("groups");

    rerender({ pathname: "/dashboard" });
    expect(result.current.expanded).toBe(null);
  });
});
