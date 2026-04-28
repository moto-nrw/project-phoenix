import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

import { useScrollY } from "./use-scroll-y";

describe("useScrollY", () => {
  let originalRAF: typeof window.requestAnimationFrame;
  let originalCancelRAF: typeof window.cancelAnimationFrame;
  let pendingCallback: FrameRequestCallback | null;
  let cancelled: boolean;

  beforeEach(() => {
    originalRAF = window.requestAnimationFrame;
    originalCancelRAF = window.cancelAnimationFrame;
    pendingCallback = null;
    cancelled = false;
    // Synchronous-controllable rAF: store the callback so the test can
    // decide when to run the throttled update.
    window.requestAnimationFrame = vi.fn((cb: FrameRequestCallback) => {
      pendingCallback = cb;
      return 1;
    }) as typeof window.requestAnimationFrame;
    window.cancelAnimationFrame = vi.fn(() => {
      cancelled = true;
    });
    Object.defineProperty(window, "scrollY", {
      value: 0,
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    window.requestAnimationFrame = originalRAF;
    window.cancelAnimationFrame = originalCancelRAF;
  });

  it("initialises to current window.scrollY on mount", () => {
    Object.defineProperty(window, "scrollY", {
      value: 42,
      writable: true,
      configurable: true,
    });
    const { result } = renderHook(() => useScrollY());
    expect(result.current).toBe(42);
  });

  it("updates after a scroll event when the rAF callback fires", () => {
    const { result } = renderHook(() => useScrollY());
    expect(result.current).toBe(0);

    Object.defineProperty(window, "scrollY", {
      value: 120,
      writable: true,
      configurable: true,
    });

    act(() => {
      window.dispatchEvent(new Event("scroll"));
    });
    // rAF was scheduled but not yet executed — scrollY hasn't propagated.
    expect(result.current).toBe(0);

    // Flush the scheduled callback.
    act(() => {
      pendingCallback?.(0);
    });
    expect(result.current).toBe(120);
  });

  it("throttles bursts of scroll events with a single rAF in flight", () => {
    renderHook(() => useScrollY());

    act(() => {
      window.dispatchEvent(new Event("scroll"));
      window.dispatchEvent(new Event("scroll"));
      window.dispatchEvent(new Event("scroll"));
    });

    // Three scrolls but only one rAF scheduled — that's the throttle.
    expect(window.requestAnimationFrame).toHaveBeenCalledTimes(1);
  });

  it("cancels a pending frame on unmount", () => {
    const { unmount } = renderHook(() => useScrollY());

    act(() => {
      window.dispatchEvent(new Event("scroll"));
    });
    expect(pendingCallback).not.toBeNull();

    unmount();
    expect(cancelled).toBe(true);
  });

  it("removes the scroll listener on unmount", () => {
    const removeSpy = vi.spyOn(window, "removeEventListener");
    const { unmount } = renderHook(() => useScrollY());
    unmount();
    expect(removeSpy).toHaveBeenCalledWith("scroll", expect.any(Function));
    removeSpy.mockRestore();
  });
});
