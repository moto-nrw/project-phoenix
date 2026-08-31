import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSidebarCollapsed } from "~/lib/hooks/use-sidebar-collapsed";

// Die Testumgebung simuliert einen Desktop-Viewport (1920px, vitest.config.ts),
// daher ist der Default ohne gespeicherte Wahl "ausgeklappt".
describe("useSidebarCollapsed", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("defaults to expanded on wide viewports", () => {
    const { result } = renderHook(() => useSidebarCollapsed());

    expect(result.current.collapsed).toBe(false);
  });

  it("defaults to collapsed when the viewport is narrower than 1280px", () => {
    vi.spyOn(window, "matchMedia").mockReturnValue({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);

    const { result } = renderHook(() => useSidebarCollapsed());

    expect(result.current.collapsed).toBe(true);
  });

  it("prefers the stored choice over the viewport default", () => {
    vi.spyOn(window, "matchMedia").mockReturnValue({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList);
    localStorage.setItem("sidebar-collapsed", "false");

    const { result } = renderHook(() => useSidebarCollapsed());

    expect(result.current.collapsed).toBe(false);
  });

  it("toggles and persists per device", () => {
    const { result } = renderHook(() => useSidebarCollapsed());

    act(() => {
      result.current.toggleCollapsed();
    });

    expect(result.current.collapsed).toBe(true);
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");

    act(() => {
      result.current.toggleCollapsed();
    });

    expect(result.current.collapsed).toBe(false);
    expect(localStorage.getItem("sidebar-collapsed")).toBe("false");
  });

  it("expandSidebar always lands on expanded and persists it", () => {
    localStorage.setItem("sidebar-collapsed", "true");
    const { result } = renderHook(() => useSidebarCollapsed());

    expect(result.current.collapsed).toBe(true);

    act(() => {
      result.current.expandSidebar();
    });

    expect(result.current.collapsed).toBe(false);
    expect(localStorage.getItem("sidebar-collapsed")).toBe("false");
  });

  it("stays in sync across multiple consumers in the same document", () => {
    const first = renderHook(() => useSidebarCollapsed());
    const second = renderHook(() => useSidebarCollapsed());

    act(() => {
      first.result.current.toggleCollapsed();
    });

    expect(second.result.current.collapsed).toBe(true);
  });
});
