import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";

import { useScrollToTop } from "./use-scroll-to-top";

describe("useScrollToTop", () => {
  let originalScrollTo: typeof window.scrollTo;
  let scrollTo: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    originalScrollTo = window.scrollTo;
    scrollTo = vi.fn();
    window.scrollTo = scrollTo as unknown as typeof window.scrollTo;
  });

  afterEach(() => {
    window.scrollTo = originalScrollTo;
  });

  it("scrolls to the top on mount", () => {
    renderHook(() => useScrollToTop("student-1"));
    expect(scrollTo).toHaveBeenCalledTimes(1);
    expect(scrollTo).toHaveBeenCalledWith(0, 0);
  });

  it("scrolls again when the key changes (e.g. switching students in place)", () => {
    const { rerender } = renderHook(({ k }) => useScrollToTop(k), {
      initialProps: { k: "student-1" },
    });
    expect(scrollTo).toHaveBeenCalledTimes(1);

    rerender({ k: "student-2" });
    expect(scrollTo).toHaveBeenCalledTimes(2);
  });

  it("does not scroll again on re-render when the key is unchanged", () => {
    const { rerender } = renderHook(({ k }) => useScrollToTop(k), {
      initialProps: { k: "student-1" },
    });
    expect(scrollTo).toHaveBeenCalledTimes(1);

    // Re-render with the same key — the keyed effect must not re-fire, so a
    // normal re-render (state update, parent re-render) never yanks the user
    // back to the top mid-scroll.
    rerender({ k: "student-1" });
    expect(scrollTo).toHaveBeenCalledTimes(1);
  });

  it("still scrolls to the top on mount when no key is provided", () => {
    renderHook(() => useScrollToTop());
    expect(scrollTo).toHaveBeenCalledTimes(1);
    expect(scrollTo).toHaveBeenCalledWith(0, 0);
  });
});
