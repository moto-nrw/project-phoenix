import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useScrollLock } from "./useScrollLock";

describe("useScrollLock", () => {
  beforeEach(() => {
    // Reset body classes before each test
    document.body.className = "";
    // Reset scroll position
    Object.defineProperty(globalThis, "pageYOffset", {
      value: 0,
      writable: true,
    });
  });

  afterEach(() => {
    // Clean up any event listeners
    document.body.className = "";
  });

  it("should add modal-open class to body when locked", () => {
    renderHook(() => useScrollLock(true));

    expect(document.body.classList.contains("modal-open")).toBe(true);
  });

  it("should not add modal-open class when not locked", () => {
    renderHook(() => useScrollLock(false));

    expect(document.body.classList.contains("modal-open")).toBe(false);
  });

  it("should remove modal-open class when unlocked", () => {
    const { rerender } = renderHook(({ isLocked }) => useScrollLock(isLocked), {
      initialProps: { isLocked: true },
    });

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

  describe("wheel event blocking", () => {
    it("should prevent wheel scroll on non-modal content", () => {
      renderHook(() => useScrollLock(true));

      const wheelEvent = new WheelEvent("wheel", {
        bubbles: true,
        cancelable: true,
      });
      const preventDefaultSpy = vi.spyOn(wheelEvent, "preventDefault");

      document.dispatchEvent(wheelEvent);

      expect(preventDefaultSpy).toHaveBeenCalled();
    });

    it("should allow wheel scroll inside modal content", async () => {
      renderHook(() => useScrollLock(true));

      // Create modal content element
      const modalContent = document.createElement("div");
      modalContent.setAttribute("data-modal-content", "true");
      document.body.appendChild(modalContent);

      // Create child element inside modal
      const childElement = document.createElement("div");
      modalContent.appendChild(childElement);

      // Wait for MutationObserver to process
      await new Promise((resolve) => setTimeout(resolve, 0));

      const wheelEvent = new WheelEvent("wheel", {
        bubbles: true,
        cancelable: true,
      });
      const preventDefaultSpy = vi.spyOn(wheelEvent, "preventDefault");

      // Dispatch from child element inside modal content
      childElement.dispatchEvent(wheelEvent);

      // Should NOT prevent default because it's inside modal content
      expect(preventDefaultSpy).not.toHaveBeenCalled();

      // Cleanup
      document.body.removeChild(modalContent);
    });

    it("should set up wheel event listener when locked", () => {
      const addEventListenerSpy = vi.spyOn(document, "addEventListener");

      renderHook(() => useScrollLock(true));

      expect(addEventListenerSpy).toHaveBeenCalledWith(
        "wheel",
        expect.any(Function),
        { passive: false },
      );

      addEventListenerSpy.mockRestore();
    });
  });

  describe("touch event blocking", () => {
    it("should prevent touchmove on non-modal content", () => {
      renderHook(() => useScrollLock(true));

      const touchEvent = new TouchEvent("touchmove", {
        bubbles: true,
        cancelable: true,
      });
      const preventDefaultSpy = vi.spyOn(touchEvent, "preventDefault");

      document.dispatchEvent(touchEvent);

      expect(preventDefaultSpy).toHaveBeenCalled();
    });

    it("should allow touchmove inside modal content", async () => {
      renderHook(() => useScrollLock(true));

      // Create modal content element
      const modalContent = document.createElement("div");
      modalContent.setAttribute("data-modal-content", "true");
      document.body.appendChild(modalContent);

      // Wait for MutationObserver to process
      await new Promise((resolve) => setTimeout(resolve, 0));

      const touchEvent = new TouchEvent("touchmove", {
        bubbles: true,
        cancelable: true,
      });
      const preventDefaultSpy = vi.spyOn(touchEvent, "preventDefault");

      // Dispatch from modal content element
      modalContent.dispatchEvent(touchEvent);

      // Should NOT prevent default because it's inside modal content
      expect(preventDefaultSpy).not.toHaveBeenCalled();

      // Cleanup
      document.body.removeChild(modalContent);
    });

    it("should set up touchmove event listener when locked", () => {
      const addEventListenerSpy = vi.spyOn(document, "addEventListener");

      renderHook(() => useScrollLock(true));

      expect(addEventListenerSpy).toHaveBeenCalledWith(
        "touchmove",
        expect.any(Function),
        { passive: false },
      );

      addEventListenerSpy.mockRestore();
    });
  });

  describe("keyboard event blocking", () => {
    const scrollKeys = [
      "ArrowUp",
      "ArrowDown",
      "PageUp",
      "PageDown",
      "Home",
      "End",
      " ",
    ];

    scrollKeys.forEach((key) => {
      it(`should prevent ${key === " " ? "Space" : key} key on non-modal content`, () => {
        renderHook(() => useScrollLock(true));

        const keyEvent = new KeyboardEvent("keydown", {
          key,
          bubbles: true,
          cancelable: true,
        });
        const preventDefaultSpy = vi.spyOn(keyEvent, "preventDefault");

        document.dispatchEvent(keyEvent);

        expect(preventDefaultSpy).toHaveBeenCalled();
      });
    });

    it("should not prevent non-scroll keys", () => {
      renderHook(() => useScrollLock(true));

      const keyEvent = new KeyboardEvent("keydown", {
        key: "a",
        bubbles: true,
        cancelable: true,
      });
      const preventDefaultSpy = vi.spyOn(keyEvent, "preventDefault");

      document.dispatchEvent(keyEvent);

      expect(preventDefaultSpy).not.toHaveBeenCalled();
    });

    it("should set up keydown event listener when locked", () => {
      const addEventListenerSpy = vi.spyOn(document, "addEventListener");

      renderHook(() => useScrollLock(true));

      expect(addEventListenerSpy).toHaveBeenCalledWith(
        "keydown",
        expect.any(Function),
      );

      addEventListenerSpy.mockRestore();
    });
  });

  describe("MutationObserver", () => {
    it("should set up MutationObserver to watch for modal content", () => {
      const observeSpy = vi.spyOn(MutationObserver.prototype, "observe");

      renderHook(() => useScrollLock(true));

      expect(observeSpy).toHaveBeenCalledWith(document.body, {
        childList: true,
        subtree: true,
        attributes: true,
        attributeFilter: ["data-modal-content"],
      });

      observeSpy.mockRestore();
    });

    it("should disconnect MutationObserver on cleanup", () => {
      const disconnectSpy = vi.spyOn(MutationObserver.prototype, "disconnect");

      const { unmount } = renderHook(() => useScrollLock(true));
      unmount();

      expect(disconnectSpy).toHaveBeenCalled();

      disconnectSpy.mockRestore();
    });
  });

  describe("event listener cleanup", () => {
    it("should remove event listeners on unlock", () => {
      const removeEventListenerSpy = vi.spyOn(document, "removeEventListener");

      const { rerender } = renderHook(
        ({ isLocked }) => useScrollLock(isLocked),
        { initialProps: { isLocked: true } },
      );

      rerender({ isLocked: false });

      expect(removeEventListenerSpy).toHaveBeenCalledWith(
        "wheel",
        expect.any(Function),
      );
      expect(removeEventListenerSpy).toHaveBeenCalledWith(
        "touchmove",
        expect.any(Function),
      );
      expect(removeEventListenerSpy).toHaveBeenCalledWith(
        "keydown",
        expect.any(Function),
      );

      removeEventListenerSpy.mockRestore();
    });
  });

  it("should handle multiple lock/unlock cycles", () => {
    const { rerender } = renderHook(({ isLocked }) => useScrollLock(isLocked), {
      initialProps: { isLocked: false },
    });

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
});
