import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useModalAnimation } from "./useModalAnimation";

describe("useModalAnimation", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  describe("initial state", () => {
    it("starts with isAnimating false when closed", () => {
      const onClose = vi.fn();
      const { result } = renderHook(() => useModalAnimation(false, onClose));

      expect(result.current.isAnimating).toBe(false);
    });

    it("starts with isExiting false when closed", () => {
      const onClose = vi.fn();
      const { result } = renderHook(() => useModalAnimation(false, onClose));

      expect(result.current.isExiting).toBe(false);
    });
  });

  describe("when isOpen becomes true", () => {
    it("sets isAnimating to true after entrance delay", () => {
      const onClose = vi.fn();
      const { result, rerender } = renderHook(
        ({ isOpen }) => useModalAnimation(isOpen, onClose),
        { initialProps: { isOpen: false } },
      );

      expect(result.current.isAnimating).toBe(false);

      rerender({ isOpen: true });

      expect(result.current.isAnimating).toBe(false);

      act(() => {
        vi.advanceTimersByTime(10);
      });

      expect(result.current.isAnimating).toBe(true);
    });

    it("uses custom entrance delay when provided", () => {
      const onClose = vi.fn();
      const { result, rerender } = renderHook(
        ({ isOpen }) => useModalAnimation(isOpen, onClose, 250, 50),
        { initialProps: { isOpen: false } },
      );

      rerender({ isOpen: true });

      act(() => {
        vi.advanceTimersByTime(10);
      });

      expect(result.current.isAnimating).toBe(false);

      act(() => {
        vi.advanceTimersByTime(40);
      });

      expect(result.current.isAnimating).toBe(true);
    });

    it("clears entrance timer on unmount", () => {
      const onClose = vi.fn();
      const { unmount, rerender } = renderHook(
        ({ isOpen }) => useModalAnimation(isOpen, onClose),
        { initialProps: { isOpen: false } },
      );

      rerender({ isOpen: true });

      const clearTimeoutSpy = vi.spyOn(global, "clearTimeout");

      unmount();

      expect(clearTimeoutSpy).toHaveBeenCalled();
    });
  });

  describe("when isOpen becomes false", () => {
    it("resets isAnimating to false", () => {
      const onClose = vi.fn();
      const { result, rerender } = renderHook(
        ({ isOpen }) => useModalAnimation(isOpen, onClose),
        { initialProps: { isOpen: true } },
      );

      act(() => {
        vi.advanceTimersByTime(10);
      });

      expect(result.current.isAnimating).toBe(true);

      rerender({ isOpen: false });

      expect(result.current.isAnimating).toBe(false);
    });

    it("resets isExiting to false", () => {
      const onClose = vi.fn();
      const { result, rerender } = renderHook(
        ({ isOpen }) => useModalAnimation(isOpen, onClose),
        { initialProps: { isOpen: true } },
      );

      act(() => {
        result.current.handleClose();
      });

      expect(result.current.isExiting).toBe(true);

      rerender({ isOpen: false });

      expect(result.current.isExiting).toBe(false);
    });
  });

  describe("handleClose", () => {
    it("sets isExiting to true immediately", () => {
      const onClose = vi.fn();
      const { result } = renderHook(() => useModalAnimation(true, onClose));

      expect(result.current.isExiting).toBe(false);

      act(() => {
        result.current.handleClose();
      });

      expect(result.current.isExiting).toBe(true);
    });

    it("sets isAnimating to false immediately", () => {
      const onClose = vi.fn();
      const { result } = renderHook(() => useModalAnimation(true, onClose));

      act(() => {
        vi.advanceTimersByTime(10);
      });

      expect(result.current.isAnimating).toBe(true);

      act(() => {
        result.current.handleClose();
      });

      expect(result.current.isAnimating).toBe(false);
    });

    it("calls onClose after animation delay (250ms default)", () => {
      const onClose = vi.fn();
      const { result } = renderHook(() => useModalAnimation(true, onClose));

      act(() => {
        result.current.handleClose();
      });

      expect(onClose).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(250);
      });

      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("uses custom animation delay when provided", () => {
      const onClose = vi.fn();
      const { result } = renderHook(() =>
        useModalAnimation(true, onClose, 500),
      );

      act(() => {
        result.current.handleClose();
      });

      act(() => {
        vi.advanceTimersByTime(250);
      });

      expect(onClose).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(250);
      });

      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("maintains stable reference across renders", () => {
      const onClose = vi.fn();
      const { result, rerender } = renderHook(() =>
        useModalAnimation(true, onClose),
      );

      const firstHandleClose = result.current.handleClose;

      rerender();

      expect(result.current.handleClose).toBe(firstHandleClose);
    });

    it("updates when onClose changes", () => {
      const onClose1 = vi.fn();
      const onClose2 = vi.fn();

      const { result, rerender } = renderHook(
        ({ callback }) => useModalAnimation(true, callback),
        { initialProps: { callback: onClose1 } },
      );

      act(() => {
        result.current.handleClose();
      });

      act(() => {
        vi.advanceTimersByTime(250);
      });

      expect(onClose1).toHaveBeenCalledTimes(1);
      expect(onClose2).not.toHaveBeenCalled();

      rerender({ callback: onClose2 });

      act(() => {
        result.current.handleClose();
      });

      act(() => {
        vi.advanceTimersByTime(250);
      });

      expect(onClose1).toHaveBeenCalledTimes(1);
      expect(onClose2).toHaveBeenCalledTimes(1);
    });
  });

  describe("edge cases", () => {
    it("handles rapid open/close transitions", () => {
      const onClose = vi.fn();
      const { result, rerender } = renderHook(
        ({ isOpen }) => useModalAnimation(isOpen, onClose),
        { initialProps: { isOpen: false } },
      );

      // Open
      rerender({ isOpen: true });

      act(() => {
        vi.advanceTimersByTime(10);
      });

      expect(result.current.isAnimating).toBe(true);

      // Close immediately
      rerender({ isOpen: false });

      expect(result.current.isAnimating).toBe(false);
      expect(result.current.isExiting).toBe(false);
    });

    it("handles opening while close animation is in progress", () => {
      const onClose = vi.fn();
      const { result, rerender } = renderHook(
        ({ isOpen }) => useModalAnimation(isOpen, onClose),
        { initialProps: { isOpen: true } },
      );

      act(() => {
        vi.advanceTimersByTime(10);
      });

      // Start closing via handleClose
      act(() => {
        result.current.handleClose();
      });

      expect(result.current.isExiting).toBe(true);

      // Close first (resets state), then reopen
      rerender({ isOpen: false });

      expect(result.current.isExiting).toBe(false);

      // Re-open
      rerender({ isOpen: true });

      act(() => {
        vi.advanceTimersByTime(10);
      });

      expect(result.current.isAnimating).toBe(true);
    });

    it("prevents onClose from being called if modal reopens", () => {
      const onClose = vi.fn();
      const { result, rerender } = renderHook(
        ({ isOpen }) => useModalAnimation(isOpen, onClose),
        { initialProps: { isOpen: true } },
      );

      act(() => {
        result.current.handleClose();
      });

      // Re-open before animation delay completes (doesn't prevent timer)
      rerender({ isOpen: true });

      act(() => {
        vi.advanceTimersByTime(250);
      });

      // onClose still called because setTimeout wasn't cancelled
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });
});
