import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useScrollLock } from "./useScrollLock";

describe("useScrollLock", () => {
  beforeEach(() => {
    document.body.className = "";
  });

  afterEach(() => {
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
    const { rerender } = renderHook(
      ({ isLocked }) => useScrollLock(isLocked),
      { initialProps: { isLocked: true } },
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

  it("should handle multiple lock/unlock cycles", () => {
    const { rerender } = renderHook(
      ({ isLocked }) => useScrollLock(isLocked),
      { initialProps: { isLocked: false } },
    );

    expect(document.body.classList.contains("modal-open")).toBe(false);

    rerender({ isLocked: true });
    expect(document.body.classList.contains("modal-open")).toBe(true);

    rerender({ isLocked: false });
    expect(document.body.classList.contains("modal-open")).toBe(false);

    rerender({ isLocked: true });
    expect(document.body.classList.contains("modal-open")).toBe(true);
  });

  it("should not affect other body classes", () => {
    document.body.classList.add("existing-class");

    const { unmount } = renderHook(() => useScrollLock(true));

    expect(document.body.classList.contains("modal-open")).toBe(true);
    expect(document.body.classList.contains("existing-class")).toBe(true);

    unmount();

    expect(document.body.classList.contains("modal-open")).toBe(false);
    expect(document.body.classList.contains("existing-class")).toBe(true);
  });
});
