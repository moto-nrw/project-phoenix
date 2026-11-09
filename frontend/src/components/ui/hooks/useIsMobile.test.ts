import { afterEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { DESKTOP_WIDTH, setViewportWidth } from "~/test/helpers/viewport";
import { useIsMobile } from "./useIsMobile";

describe("useIsMobile", () => {
  afterEach(() => {
    document.documentElement.style.fontSize = "";
    setViewportWidth(DESKTOP_WIDTH);
    vi.restoreAllMocks();
  });

  it("returns false for desktop width", () => {
    setViewportWidth(1024);

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(false);
  });

  it("returns true for mobile width", () => {
    setViewportWidth(375);

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(true);
  });

  it.each([
    { width: 767.5, fontSize: 16, mobile: true },
    { width: 768, fontSize: 16, mobile: false },
    { width: 959.5, fontSize: 20, mobile: true },
    { width: 960, fontSize: 20, mobile: false },
  ])(
    "matches Tailwind's md at $width px with a $fontSize px browser font",
    ({ width, fontSize, mobile }) => {
      document.documentElement.style.fontSize = `${fontSize}px`;
      setViewportWidth(width);

      const { result } = renderHook(() => useIsMobile());

      expect(result.current).toBe(mobile);
    },
  );

  it("updates on resize from desktop to mobile", () => {
    setViewportWidth(1024);
    const { result } = renderHook(() => useIsMobile());
    expect(result.current).toBe(false);

    setViewportWidth(375);

    expect(result.current).toBe(true);
  });

  it("reads mobile to desktop when mounted wide", () => {
    // Happy DOM reports no `change` when a matching query stops matching
    // (see setViewportWidth), so the wide case mounts after the resize.
    setViewportWidth(375);
    const narrow = renderHook(() => useIsMobile());
    expect(narrow.result.current).toBe(true);
    narrow.unmount();

    setViewportWidth(1024);
    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(false);
  });

  it("removes its listener from the same media query list on unmount", () => {
    const lists: MediaQueryList[] = [];
    const realMatchMedia = window.matchMedia.bind(window);
    vi.spyOn(window, "matchMedia").mockImplementation((query) => {
      const list = realMatchMedia(query);
      vi.spyOn(list, "addEventListener");
      vi.spyOn(list, "removeEventListener");
      lists.push(list);
      return list;
    });

    const { unmount } = renderHook(() => useIsMobile());
    const [list] = lists;
    const listener = vi.mocked(list!.addEventListener).mock.calls[0]?.[1];
    expect(listener).toBeTypeOf("function");
    unmount();

    expect(list!.removeEventListener).toHaveBeenCalledWith("change", listener);
  });
});
