import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useScrollLock } from "./useScrollLock";

describe("useScrollLock", () => {
  beforeEach(() => {
    // Reset html element classes before each test
    document.documentElement.className = "";
  });

  afterEach(() => {
    // Clean up
    document.documentElement.className = "";
  });

  describe("modal-open class management", () => {
    it("should add modal-open class to html element when locked", () => {
      renderHook(() => useScrollLock(true));

      expect(document.documentElement.classList.contains("modal-open")).toBe(true);
    });

    it("should not add modal-open class when not locked", () => {
      renderHook(() => useScrollLock(false));

      expect(document.documentElement.classList.contains("modal-open")).toBe(false);
    });

    it("should remove modal-open class when unlocked", () => {
      const { rerender } = renderHook(
        ({ isLocked }) => useScrollLock(isLocked),
        {
          initialProps: { isLocked: true },
        },
      );

      expect(document.documentElement.classList.contains("modal-open")).toBe(true);

      rerender({ isLocked: false });

      expect(document.documentElement.classList.contains("modal-open")).toBe(false);
    });

    it("should remove modal-open class on unmount", () => {
      const { unmount } = renderHook(() => useScrollLock(true));

      expect(document.documentElement.classList.contains("modal-open")).toBe(true);

      unmount();

      expect(document.documentElement.classList.contains("modal-open")).toBe(false);
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

      expect(document.documentElement.classList.contains("modal-open")).toBe(false);

      rerender({ isLocked: true });
      expect(document.documentElement.classList.contains("modal-open")).toBe(true);

      rerender({ isLocked: false });
      expect(document.documentElement.classList.contains("modal-open")).toBe(false);

      rerender({ isLocked: true });
      expect(document.documentElement.classList.contains("modal-open")).toBe(true);

      rerender({ isLocked: false });
      expect(document.documentElement.classList.contains("modal-open")).toBe(false);
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

      expect(document.documentElement.classList.contains("modal-open")).toBe(false);
    });

    it("should not affect other html element classes", () => {
      document.documentElement.classList.add("existing-class");

      const { unmount } = renderHook(() => useScrollLock(true));

      expect(document.documentElement.classList.contains("modal-open")).toBe(true);
      expect(document.documentElement.classList.contains("existing-class")).toBe(true);

      unmount();

      expect(document.documentElement.classList.contains("modal-open")).toBe(false);
      expect(document.documentElement.classList.contains("existing-class")).toBe(true);
    });
  });
});
