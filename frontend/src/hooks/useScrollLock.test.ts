import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { MockInstance } from "vitest";
import { renderHook } from "@testing-library/react";
import { useScrollLock } from "./useScrollLock";

describe("useScrollLock", () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let scrollToSpy: MockInstance<any>;

  beforeEach(() => {
    // Reset body classes before each test
    document.body.className = "";
    // Reset CSS variable
    document.documentElement.style.removeProperty("--scroll-y");
    // Mock scrollY
    Object.defineProperty(globalThis, "scrollY", {
      value: 0,
      writable: true,
      configurable: true,
    });
    // Spy on scrollTo
    scrollToSpy = vi.spyOn(globalThis, "scrollTo").mockImplementation(() => {
      // Mock implementation - does nothing
    });
  });

  afterEach(() => {
    // Clean up
    document.body.className = "";
    document.documentElement.style.removeProperty("--scroll-y");
    scrollToSpy.mockRestore();
  });

  describe("modal-open class management", () => {
    it("should add modal-open class to body when locked", () => {
      renderHook(() => useScrollLock(true));

      expect(document.body.classList.contains("modal-open")).toBe(true);
    });

    it("should not add modal-open class when not locked", () => {
      renderHook(() => useScrollLock(false));

      expect(document.body.classList.contains("modal-open")).toBe(false);
    });

    it("should remove modal-open class when unlocked", () => {
      const { rerender } = renderHook(
        ({ isLocked }) => useScrollLock(isLocked),
        {
          initialProps: { isLocked: true },
        },
      );

      expect(document.body.classList.contains("modal-open")).toBe(true);

      rerender({ isLocked: false });

      expect(document.body.classList.contains("modal-open")).toBe(false);
    });

    it("should remove modal-open class on unmount", () => {
      const { unmount } = renderHook(() => useScrollLock(true));

      expect(document.body.classList.contains("modal-open")).toBe(true);

      unmount();

      expect(document.body.classList.contains("modal-open")).toBe(false);
    });
  });

  describe("CSS variable for scroll position", () => {
    it("should set --scroll-y CSS variable when locked", () => {
      Object.defineProperty(globalThis, "scrollY", {
        value: 150,
        writable: true,
        configurable: true,
      });

      renderHook(() => useScrollLock(true));

      expect(
        document.documentElement.style.getPropertyValue("--scroll-y"),
      ).toBe("150px");
    });

    it("should set --scroll-y to 0px when scrollY is 0", () => {
      Object.defineProperty(globalThis, "scrollY", {
        value: 0,
        writable: true,
        configurable: true,
      });

      renderHook(() => useScrollLock(true));

      expect(
        document.documentElement.style.getPropertyValue("--scroll-y"),
      ).toBe("0px");
    });

    it("should remove --scroll-y CSS variable on unlock", () => {
      const { rerender } = renderHook(
        ({ isLocked }) => useScrollLock(isLocked),
        {
          initialProps: { isLocked: true },
        },
      );

      expect(
        document.documentElement.style.getPropertyValue("--scroll-y"),
      ).toBe("0px");

      rerender({ isLocked: false });

      expect(
        document.documentElement.style.getPropertyValue("--scroll-y"),
      ).toBe("");
    });

    it("should remove --scroll-y CSS variable on unmount", () => {
      const { unmount } = renderHook(() => useScrollLock(true));

      expect(
        document.documentElement.style.getPropertyValue("--scroll-y"),
      ).toBe("0px");

      unmount();

      expect(
        document.documentElement.style.getPropertyValue("--scroll-y"),
      ).toBe("");
    });
  });

  describe("scroll position restoration", () => {
    it("should restore scroll position on unlock", () => {
      Object.defineProperty(globalThis, "scrollY", {
        value: 200,
        writable: true,
        configurable: true,
      });

      const { rerender } = renderHook(
        ({ isLocked }) => useScrollLock(isLocked),
        {
          initialProps: { isLocked: true },
        },
      );

      rerender({ isLocked: false });

      expect(scrollToSpy).toHaveBeenCalledWith(0, 200);
    });

    it("should restore scroll position on unmount", () => {
      Object.defineProperty(globalThis, "scrollY", {
        value: 300,
        writable: true,
        configurable: true,
      });

      const { unmount } = renderHook(() => useScrollLock(true));
      unmount();

      expect(scrollToSpy).toHaveBeenCalledWith(0, 300);
    });

    it("should not call scrollTo when never locked", () => {
      renderHook(() => useScrollLock(false));

      expect(scrollToSpy).not.toHaveBeenCalled();
    });
  });

  describe("multiple lock/unlock cycles", () => {
    it("should handle multiple lock/unlock cycles correctly", () => {
      const { rerender } = renderHook(
        ({ isLocked }) => useScrollLock(isLocked),
        {
          initialProps: { isLocked: false },
        },
      );

      expect(document.body.classList.contains("modal-open")).toBe(false);

      rerender({ isLocked: true });
      expect(document.body.classList.contains("modal-open")).toBe(true);

      rerender({ isLocked: false });
      expect(document.body.classList.contains("modal-open")).toBe(false);

      rerender({ isLocked: true });
      expect(document.body.classList.contains("modal-open")).toBe(true);

      rerender({ isLocked: false });
      expect(document.body.classList.contains("modal-open")).toBe(false);
    });

    it("should preserve scroll position across multiple cycles", () => {
      Object.defineProperty(globalThis, "scrollY", {
        value: 100,
        writable: true,
        configurable: true,
      });

      const { rerender } = renderHook(
        ({ isLocked }) => useScrollLock(isLocked),
        {
          initialProps: { isLocked: true },
        },
      );

      rerender({ isLocked: false });
      expect(scrollToSpy).toHaveBeenCalledWith(0, 100);

      // Simulate new scroll position
      Object.defineProperty(globalThis, "scrollY", {
        value: 250,
        writable: true,
        configurable: true,
      });
      scrollToSpy.mockClear();

      rerender({ isLocked: true });
      expect(
        document.documentElement.style.getPropertyValue("--scroll-y"),
      ).toBe("250px");

      rerender({ isLocked: false });
      expect(scrollToSpy).toHaveBeenCalledWith(0, 250);
    });
  });

  describe("edge cases", () => {
    it("should handle rapid lock/unlock without errors", () => {
      const { rerender } = renderHook(
        ({ isLocked }) => useScrollLock(isLocked),
        {
          initialProps: { isLocked: false },
        },
      );

      // Rapidly toggle
      for (let i = 0; i < 10; i++) {
        rerender({ isLocked: true });
        rerender({ isLocked: false });
      }

      expect(document.body.classList.contains("modal-open")).toBe(false);
    });

    it("should handle zero scroll position", () => {
      Object.defineProperty(globalThis, "scrollY", {
        value: 0,
        writable: true,
        configurable: true,
      });

      const { rerender } = renderHook(
        ({ isLocked }) => useScrollLock(isLocked),
        {
          initialProps: { isLocked: true },
        },
      );

      rerender({ isLocked: false });

      expect(scrollToSpy).toHaveBeenCalledWith(0, 0);
    });
  });
});
