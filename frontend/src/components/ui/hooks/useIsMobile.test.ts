import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useIsMobile } from "./useIsMobile";

describe("useIsMobile", () => {
  let originalInnerWidth: number;
  let resizeListeners: Array<(event: Event) => void>;

  beforeEach(() => {
    // Store original innerWidth
    originalInnerWidth = globalThis.innerWidth;

    // Track resize listeners
    resizeListeners = [];

    // Mock addEventListener and removeEventListener
    const addEventListener = vi.fn(
      (event: string, listener: (event: Event) => void) => {
        if (event === "resize") {
          resizeListeners.push(listener);
        }
      },
    );

    const removeEventListener = vi.fn(
      (event: string, listener: (event: Event) => void) => {
        if (event === "resize") {
          const index = resizeListeners.indexOf(listener);
          if (index > -1) {
            resizeListeners.splice(index, 1);
          }
        }
      },
    );

    globalThis.addEventListener =
      addEventListener as unknown as typeof globalThis.addEventListener;
    globalThis.removeEventListener =
      removeEventListener as unknown as typeof globalThis.removeEventListener;
  });

  afterEach(() => {
    // Restore original innerWidth
    Object.defineProperty(globalThis, "innerWidth", {
      value: originalInnerWidth,
      writable: true,
      configurable: true,
    });

    resizeListeners = [];
  });

  it("returns false for desktop width (>=768px)", () => {
    Object.defineProperty(globalThis, "innerWidth", {
      value: 1024,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(false);
  });

  it("returns true for mobile width (<768px)", () => {
    Object.defineProperty(globalThis, "innerWidth", {
      value: 375,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(true);
  });

  it("returns false exactly at breakpoint (768px)", () => {
    Object.defineProperty(globalThis, "innerWidth", {
      value: 768,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(false);
  });

  it("updates on resize from desktop to mobile", () => {
    Object.defineProperty(globalThis, "innerWidth", {
      value: 1024,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(false);

    // Simulate resize to mobile width
    act(() => {
      Object.defineProperty(globalThis, "innerWidth", {
        value: 375,
        writable: true,
        configurable: true,
      });

      // Trigger all resize listeners
      resizeListeners.forEach((listener) => {
        listener(new Event("resize"));
      });
    });

    expect(result.current).toBe(true);
  });

  it("updates on resize from mobile to desktop", () => {
    Object.defineProperty(globalThis, "innerWidth", {
      value: 375,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(true);

    // Simulate resize to desktop width
    act(() => {
      Object.defineProperty(globalThis, "innerWidth", {
        value: 1024,
        writable: true,
        configurable: true,
      });

      // Trigger all resize listeners
      resizeListeners.forEach((listener) => {
        listener(new Event("resize"));
      });
    });

    expect(result.current).toBe(false);
  });

  it("removes event listener on unmount", () => {
    Object.defineProperty(globalThis, "innerWidth", {
      value: 1024,
      writable: true,
      configurable: true,
    });

    const { unmount } = renderHook(() => useIsMobile());

    const listenerCountBeforeUnmount = resizeListeners.length;
    expect(listenerCountBeforeUnmount).toBe(1);

    unmount();

    expect(resizeListeners.length).toBe(0);
  });

  it("handles multiple resize events correctly", () => {
    Object.defineProperty(globalThis, "innerWidth", {
      value: 1024,
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useIsMobile());

    expect(result.current).toBe(false);

    // Multiple rapid resizes
    act(() => {
      Object.defineProperty(globalThis, "innerWidth", {
        value: 500,
        writable: true,
        configurable: true,
      });
      resizeListeners.forEach((listener) => listener(new Event("resize")));
    });

    expect(result.current).toBe(true);

    act(() => {
      Object.defineProperty(globalThis, "innerWidth", {
        value: 800,
        writable: true,
        configurable: true,
      });
      resizeListeners.forEach((listener) => listener(new Event("resize")));
    });

    expect(result.current).toBe(false);

    act(() => {
      Object.defineProperty(globalThis, "innerWidth", {
        value: 600,
        writable: true,
        configurable: true,
      });
      resizeListeners.forEach((listener) => listener(new Event("resize")));
    });

    expect(result.current).toBe(true);
  });
});
