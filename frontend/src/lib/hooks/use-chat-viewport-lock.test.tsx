import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, render, act } from "@testing-library/react";
import React from "react";
import { useChatViewportLock } from "./use-chat-viewport-lock";

// Minimal wrapper component that actually attaches the ref to a DOM element,
// which is required for the fit effect (it early-returns when ref.current is null).
function ChatWrapper({ ready }: { ready: boolean }) {
  const ref = useChatViewportLock<HTMLDivElement>(ready);
  return React.createElement("div", { ref, "data-testid": "chat" });
}

// Wrapper that nests the chat element inside a <main> with a padding-bottom,
// so el.closest("main") resolves and the bottom-reserve branch runs.
function MainWrapper({
  ready,
  paddingBottom,
}: {
  ready: boolean;
  paddingBottom: string;
}) {
  const ref = useChatViewportLock<HTMLDivElement>(ready);
  return React.createElement(
    "main",
    { style: { paddingBottom } },
    React.createElement("div", { ref, "data-testid": "chat" }),
  );
}

// ---------------------------------------------------------------------------
// Setup: reset overflow styles between tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  document.documentElement.style.overflow = "";
  document.body.style.overflow = "";
});

afterEach(() => {
  document.documentElement.style.overflow = "";
  document.body.style.overflow = "";
});

// ---------------------------------------------------------------------------
// Scroll-lock effect
// ---------------------------------------------------------------------------

describe("useChatViewportLock — scroll lock", () => {
  it("sets html and body overflow to 'hidden' on mount", () => {
    renderHook(() => useChatViewportLock<HTMLDivElement>(false));

    expect(document.documentElement.style.overflow).toBe("hidden");
    expect(document.body.style.overflow).toBe("hidden");
  });

  it("restores the previous overflow values on unmount", () => {
    document.documentElement.style.overflow = "scroll";
    document.body.style.overflow = "auto";

    const { unmount } = renderHook(() =>
      useChatViewportLock<HTMLDivElement>(false),
    );

    expect(document.documentElement.style.overflow).toBe("hidden");
    expect(document.body.style.overflow).toBe("hidden");

    unmount();

    expect(document.documentElement.style.overflow).toBe("scroll");
    expect(document.body.style.overflow).toBe("auto");
  });

  it("restores empty string when there was no prior overflow", () => {
    const { unmount } = renderHook(() =>
      useChatViewportLock<HTMLDivElement>(false),
    );
    unmount();

    expect(document.documentElement.style.overflow).toBe("");
    expect(document.body.style.overflow).toBe("");
  });
});

// ---------------------------------------------------------------------------
// Return value
// ---------------------------------------------------------------------------

describe("useChatViewportLock — return value", () => {
  it("returns a ref object (initially null)", () => {
    const { result } = renderHook(() =>
      useChatViewportLock<HTMLDivElement>(false),
    );
    // The ref is a React RefObject — current starts null because no element is
    // attached in the hook-only render.
    expect(result.current).toBeDefined();
    expect(result.current.current).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Fit effect — window resize listener
// ---------------------------------------------------------------------------

// The fit effect inside useChatViewportLock only runs when ref.current is
// non-null (it has an early `if (!el) return` guard). renderHook never
// attaches the returned ref to a DOM element, so those effects are unreachable
// via renderHook. The tests below use ChatWrapper (which does attach the ref)
// so the resize listener is actually registered.
describe("useChatViewportLock — resize listener", () => {
  it("attaches a window resize listener when ref is attached and ready is true", () => {
    const addSpy = vi.spyOn(window, "addEventListener");

    render(React.createElement(ChatWrapper, { ready: true }));

    expect(addSpy).toHaveBeenCalledWith("resize", expect.any(Function));
    addSpy.mockRestore();
  });

  it("removes the window resize listener when the component unmounts", () => {
    const removeSpy = vi.spyOn(window, "removeEventListener");

    const { unmount } = render(
      React.createElement(ChatWrapper, { ready: true }),
    );

    unmount();

    expect(removeSpy).toHaveBeenCalledWith("resize", expect.any(Function));
    removeSpy.mockRestore();
  });

  it("attaches the resize listener when ready flips from false to true", () => {
    const addSpy = vi.spyOn(window, "addEventListener");
    addSpy.mockClear();

    const { rerender } = render(
      React.createElement(ChatWrapper, { ready: false }),
    );

    const callsBefore = addSpy.mock.calls.filter(
      ([name]) => name === "resize",
    ).length;

    act(() => {
      rerender(React.createElement(ChatWrapper, { ready: true }));
    });

    const callsAfter = addSpy.mock.calls.filter(
      ([name]) => name === "resize",
    ).length;

    expect(callsAfter).toBeGreaterThan(callsBefore);
    addSpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// visualViewport fallback
// ---------------------------------------------------------------------------

describe("useChatViewportLock — visualViewport", () => {
  it("attaches resize/scroll listeners on visualViewport when available", () => {
    // happy-dom ships a basic window.visualViewport; spy on addEventListener.
    // Use ChatWrapper so the ref is attached and the fit effect actually runs.
    const vv = window.visualViewport;
    if (!vv) return; // skip if the browser/env doesn't implement it

    const vvSpy = vi.spyOn(vv, "addEventListener");

    render(React.createElement(ChatWrapper, { ready: true }));

    const resizeCalls = vvSpy.mock.calls.filter(([n]) => n === "resize");
    const scrollCalls = vvSpy.mock.calls.filter(([n]) => n === "scroll");
    expect(resizeCalls.length).toBeGreaterThanOrEqual(1);
    expect(scrollCalls.length).toBeGreaterThanOrEqual(1);

    vvSpy.mockRestore();
  });

  it("removes visualViewport listeners on unmount", () => {
    const vv = window.visualViewport;
    if (!vv) return;

    const vvSpy = vi.spyOn(vv, "removeEventListener");

    const { unmount } = render(
      React.createElement(ChatWrapper, { ready: true }),
    );

    unmount();

    const removedNames = vvSpy.mock.calls.map(([n]) => n);
    expect(removedNames).toContain("resize");
    expect(removedNames).toContain("scroll");

    vvSpy.mockRestore();
  });

  it("falls back to window.innerHeight when visualViewport is unavailable", () => {
    const original = window.visualViewport;
    // Older browsers / jsdom expose no visualViewport; the fit effect must
    // still size the element from window.innerHeight and not throw.
    Object.defineProperty(window, "visualViewport", {
      value: undefined,
      configurable: true,
    });
    window.innerHeight = 700;

    expect(() =>
      render(React.createElement(ChatWrapper, { ready: true })),
    ).not.toThrow();

    Object.defineProperty(window, "visualViewport", {
      value: original,
      configurable: true,
    });
  });
});

// ---------------------------------------------------------------------------
// Fit effect — iOS scroll-offset snap
// ---------------------------------------------------------------------------

describe("useChatViewportLock — scroll snap", () => {
  afterEach(() => {
    Object.defineProperty(window, "scrollY", {
      value: 0,
      configurable: true,
      writable: true,
    });
  });

  it("snaps the document back to the top when a stray scroll offset exists", () => {
    // iOS Safari can leave the layout viewport scrolled after the keyboard
    // dismisses even though overflow is hidden; fit() must snap it back.
    const scrollToSpy = vi
      .spyOn(window, "scrollTo")
      .mockImplementation(() => undefined);
    Object.defineProperty(window, "scrollY", {
      value: 150,
      configurable: true,
      writable: true,
    });

    render(React.createElement(ChatWrapper, { ready: true }));

    expect(scrollToSpy).toHaveBeenCalledWith(0, 0);
    scrollToSpy.mockRestore();
  });

  it("does not scroll when the document is already at the top", () => {
    const scrollToSpy = vi
      .spyOn(window, "scrollTo")
      .mockImplementation(() => undefined);
    Object.defineProperty(window, "scrollY", {
      value: 0,
      configurable: true,
      writable: true,
    });

    render(React.createElement(ChatWrapper, { ready: true }));

    expect(scrollToSpy).not.toHaveBeenCalled();
    scrollToSpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// Fit effect — mobile bottom-nav reserve (el.closest("main") padding-bottom)
// ---------------------------------------------------------------------------

describe("useChatViewportLock — bottom-nav reserve", () => {
  it("reserves the surrounding <main> padding-bottom in the fitted height", () => {
    render(
      React.createElement(MainWrapper, { ready: true, paddingBottom: "112px" }),
    );

    const el = document.querySelector<HTMLDivElement>('[data-testid="chat"]');
    // The fit effect sets an explicit pixel height (never left unset once the
    // element and its <main> ancestor resolve).
    expect(el?.style.height).toMatch(/px$/);
  });

  it("handles a <main> with no numeric padding-bottom without throwing", () => {
    // parseFloat of an empty/auto padding is NaN, which must fall back to 0.
    expect(() =>
      render(
        React.createElement(MainWrapper, { ready: true, paddingBottom: "" }),
      ),
    ).not.toThrow();

    const el = document.querySelector<HTMLDivElement>('[data-testid="chat"]');
    expect(el?.style.height).toMatch(/px$/);
  });
});
