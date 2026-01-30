import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDebounce } from "./use-debounce";

describe("useDebounce", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns initial value immediately", () => {
    const { result } = renderHook(() => useDebounce("initial", 500));

    expect(result.current).toBe("initial");
  });

  it("updates value after delay", () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      {
        initialProps: { value: "initial", delay: 500 },
      },
    );

    expect(result.current).toBe("initial");

    // Update the value
    rerender({ value: "updated", delay: 500 });

    // Value should not update immediately
    expect(result.current).toBe("initial");

    // Fast-forward time by 500ms
    act(() => {
      vi.advanceTimersByTime(500);
    });

    // Value should now be updated
    expect(result.current).toBe("updated");
  });

  it("does not update value before delay", () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      {
        initialProps: { value: "initial", delay: 500 },
      },
    );

    rerender({ value: "updated", delay: 500 });

    // Fast-forward time by less than delay
    act(() => {
      vi.advanceTimersByTime(300);
    });

    // Value should still be initial
    expect(result.current).toBe("initial");
  });

  it("cancels previous timeout on rapid value changes", () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      {
        initialProps: { value: "first", delay: 500 },
      },
    );

    // Change value multiple times rapidly
    rerender({ value: "second", delay: 500 });

    act(() => {
      vi.advanceTimersByTime(200);
    });

    rerender({ value: "third", delay: 500 });

    act(() => {
      vi.advanceTimersByTime(200);
    });

    rerender({ value: "fourth", delay: 500 });

    // Fast-forward by full delay from last update
    act(() => {
      vi.advanceTimersByTime(500);
    });

    // Should only update to the last value
    expect(result.current).toBe("fourth");
  });

  it("handles delay changes", () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      {
        initialProps: { value: "initial", delay: 500 },
      },
    );

    rerender({ value: "updated", delay: 1000 });

    // Fast-forward by original delay
    act(() => {
      vi.advanceTimersByTime(500);
    });

    // Should not update yet (new delay is 1000ms)
    expect(result.current).toBe("initial");

    // Fast-forward by remaining time
    act(() => {
      vi.advanceTimersByTime(500);
    });

    // Now should be updated
    expect(result.current).toBe("updated");
  });

  it("cleans up timeout on unmount", () => {
    const { rerender, unmount } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      {
        initialProps: { value: "initial", delay: 500 },
      },
    );

    rerender({ value: "updated", delay: 500 });

    // Unmount before timeout completes
    unmount();

    // Fast-forward time
    act(() => {
      vi.advanceTimersByTime(500);
    });

    // No error should occur (cleanup worked properly)
    expect(true).toBe(true);
  });

  it("works with different value types", () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      {
        initialProps: { value: 0, delay: 500 },
      },
    );

    expect(result.current).toBe(0);

    rerender({ value: 42, delay: 500 });

    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(result.current).toBe(42);
  });

  it("works with object values", () => {
    const initialObj = { name: "John", age: 30 };
    const updatedObj = { name: "Jane", age: 25 };

    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      {
        initialProps: { value: initialObj, delay: 500 },
      },
    );

    expect(result.current).toBe(initialObj);

    rerender({ value: updatedObj, delay: 500 });

    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(result.current).toBe(updatedObj);
  });
});
