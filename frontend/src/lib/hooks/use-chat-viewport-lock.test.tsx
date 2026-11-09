import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, render, act } from "@testing-library/react";
import React from "react";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import {
  MIN_MESSAGE_LIST_PX,
  useChatViewportLock,
} from "./use-chat-viewport-lock";

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

// ---------------------------------------------------------------------------
// Fit effect — soft keyboard on small phones (#3664)
// ---------------------------------------------------------------------------

describe("useChatViewportLock — soft keyboard", () => {
  const originalViewport = window.visualViewport;
  const originalInnerHeight = window.innerHeight;

  // Layout viewport height, visual viewport height (shrinks when the soft
  // keyboard opens) and the chat's top edge.
  const mockLayout = (
    innerHeight: number,
    visualHeight: number,
    top: number,
    scale = 1,
  ) => {
    window.innerHeight = innerHeight;
    const viewport = new EventTarget();
    Object.defineProperty(viewport, "height", { value: visualHeight });
    Object.defineProperty(viewport, "scale", { value: scale });
    Object.defineProperty(window, "visualViewport", {
      value: viewport,
      configurable: true,
    });
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(
      new DOMRect(0, top, 360, 0),
    );
  };

  const fittedChat = () => {
    render(
      React.createElement(MainWrapper, { ready: true, paddingBottom: "112px" }),
    );
    return document.querySelector<HTMLDivElement>('[data-testid="chat"]');
  };

  const keyboardFlag = () =>
    document.documentElement.hasAttribute("data-chat-keyboard");

  afterEach(() => {
    vi.restoreAllMocks();
    window.innerHeight = originalInnerHeight;
    Object.defineProperty(window, "visualViewport", {
      value: originalViewport,
      configurable: true,
    });
  });

  it("keeps the bottom-nav reserve while the keyboard is closed", () => {
    mockLayout(640, 640, 80);

    // 640 - 80 top - 112 reserved for the bottom nav.
    expect(fittedChat()?.style.height).toBe("448px");
  });

  it("ends at the keyboard instead of subtracting the hidden nav again", () => {
    // Small Android phone with the keyboard open: the layout viewport keeps its
    // height, only the visual viewport shrinks. The bottom nav sits behind the
    // keyboard, so the chat may use everything down to the keyboard edge.
    mockLayout(640, 300, 80);

    expect(fittedChat()?.style.height).toBe("220px");
  });

  it("never forces the minimum height past the visible area", () => {
    mockLayout(640, 250, 80);

    const chat = fittedChat();
    // Only 170px are visible below the chat's top. A 240px floor pushed the
    // composer under the keyboard, where the locked page cannot scroll to it.
    expect(chat?.style.height).toBe("170px");
    // The consumers' CSS min-height (min-h-[20rem]) must not undo the fit.
    expect(chat?.style.minHeight).toBe("0px");
  });

  it("still applies the minimum height when it fits on screen", () => {
    mockLayout(400, 400, 80);

    // 400 - 80 - 112 = 208 is below the floor; 240px still fit into the
    // 320px visible below the chat's top edge.
    expect(fittedChat()?.style.height).toBe("240px");
  });

  it("flags the open keyboard on <html> so pages can hide their intro", () => {
    mockLayout(640, 300, 80);
    fittedChat();
    expect(keyboardFlag()).toBe(true);
  });

  it("does not flag the keyboard while it is closed", () => {
    mockLayout(640, 640, 80);
    fittedChat();
    expect(keyboardFlag()).toBe(false);
  });

  it("does not mistake a pinch zoom for an open keyboard", () => {
    // Zoomed in 2x: the visual viewport shows half the layout in CSS pixels.
    mockLayout(640, 320, 80, 2);
    fittedChat();
    expect(keyboardFlag()).toBe(false);
  });

  it("clears the keyboard flag when the chat unmounts", () => {
    mockLayout(640, 300, 80);
    const { unmount } = render(
      React.createElement(MainWrapper, { ready: true, paddingBottom: "112px" }),
    );
    expect(keyboardFlag()).toBe(true);

    unmount();

    expect(keyboardFlag()).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Fit effect — readable message list, compact mode (small phones)
// ---------------------------------------------------------------------------

// Chat with a header, the marked message list and the shell's <main>, as the
// three chats render it.
function ChatWithList({ ready = true }: { ready?: boolean }) {
  const ref = useChatViewportLock<HTMLDivElement>(ready);
  return React.createElement(
    "main",
    { style: { paddingBottom: "112px", paddingTop: "16px" } },
    React.createElement(
      "div",
      { ref, "data-testid": "chat" },
      React.createElement("div", {
        "data-chat-scroll": "",
        "data-testid": "list",
      }),
    ),
  );
}

describe("useChatViewportLock — message list and compact mode", () => {
  const originalViewport = window.visualViewport;
  const originalInnerHeight = window.innerHeight;

  // `content` is what header, composer and the list at its minimum need
  // (scrollHeight); clientHeight follows the height the hook set.
  const mockLayout = (innerHeight: number, top: number, content: number) => {
    window.innerHeight = innerHeight;
    Object.defineProperty(window, "visualViewport", {
      value: undefined,
      configurable: true,
    });
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(
      new DOMRect(0, top, 360, 0),
    );
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockImplementation(
      function (this: HTMLElement) {
        return this.dataset.testid === "chat" ? content : 0;
      },
    );
    vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockImplementation(
      function (this: HTMLElement) {
        return parseFloat(this.style.height) || 0;
      },
    );
  };

  const chat = () =>
    document.querySelector<HTMLDivElement>('[data-testid="chat"]');
  const list = () =>
    document.querySelector<HTMLDivElement>('[data-testid="list"]');
  const compactFlag = () =>
    document.documentElement.hasAttribute("data-chat-compact");

  afterEach(() => {
    vi.restoreAllMocks();
    window.innerHeight = originalInnerHeight;
    Object.defineProperty(window, "visualViewport", {
      value: originalViewport,
      configurable: true,
    });
  });

  it("keeps the message list at a readable minimum height", () => {
    mockLayout(900, 80, 400);
    render(React.createElement(ChatWithList));

    expect(list()?.style.minHeight).toBe(`${MIN_MESSAGE_LIST_PX}px`);
  });

  it("keeps the page locked while the chat fits", () => {
    // 900 - 80 top - 112 nav = 708 px, the chat needs 400.
    mockLayout(900, 80, 400);
    render(React.createElement(ChatWithList));

    expect(chat()?.style.height).toBe("708px");
    expect(document.documentElement.style.overflow).toBe("hidden");
    expect(document.body.style.overflow).toBe("hidden");
  });

  it("lets the page scroll instead of clipping the chat when it does not fit", () => {
    // 360 × 640 phone with group messages above the chat: 640 - 525 - 112
    // leaves 3 px, the chat needs 380 (header, composer, 160 px list). The
    // locked layout shrank the list to 24 px and clipped the composer.
    mockLayout(640, 525, 380);
    render(React.createElement(ChatWithList));

    expect(document.documentElement.style.overflow).toBe("");
    expect(document.body.style.overflow).toBe("");
    // Scrolled down, the chat fills the screen from the content top (16 px
    // main padding) to the bottom nav: 640 - 112 - 16.
    expect(chat()?.style.height).toBe("512px");
  });

  it("never makes the chat shorter than its content when the page scrolls", () => {
    mockLayout(640, 525, 600);
    render(React.createElement(ChatWithList));

    expect(chat()?.style.height).toBe("600px");
  });

  it("keeps the person's own scroll position while the page scrolls", () => {
    const scrollToSpy = vi
      .spyOn(window, "scrollTo")
      .mockImplementation(() => undefined);
    mockLayout(640, 525, 380);
    render(React.createElement(ChatWithList));
    Object.defineProperty(window, "scrollY", {
      value: 200,
      configurable: true,
      writable: true,
    });

    act(() => {
      window.dispatchEvent(new Event("resize"));
    });

    expect(scrollToSpy).not.toHaveBeenCalled();
    Object.defineProperty(window, "scrollY", {
      value: 0,
      configurable: true,
      writable: true,
    });
  });

  it("restores the page overflow on unmount after releasing the lock", () => {
    document.documentElement.style.overflow = "auto";
    mockLayout(640, 525, 380);
    const { unmount } = render(React.createElement(ChatWithList));

    expect(document.documentElement.style.overflow).toBe("auto");
    unmount();
    expect(document.documentElement.style.overflow).toBe("auto");
    expect(document.body.style.overflow).toBe("");
  });

  it("flags a compact chat on small screens", () => {
    // 640 - 112 nav = 528 px usable height.
    mockLayout(640, 80, 300);
    render(React.createElement(ChatWithList));

    expect(compactFlag()).toBe(true);
  });

  it("does not flag a compact chat on tall screens", () => {
    // 844 - 112 = 732 px usable height (iPhone 14, Android 390 × 844).
    mockLayout(844, 80, 300);
    render(React.createElement(ChatWithList));

    expect(compactFlag()).toBe(false);
  });

  it("clears the compact flag when the chat unmounts", () => {
    mockLayout(640, 80, 300);
    const { unmount } = render(React.createElement(ChatWithList));
    unmount();

    expect(compactFlag()).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Every chat marks its message list
// ---------------------------------------------------------------------------

describe("useChatViewportLock — consumers", () => {
  it("is only used by chats that mark their message list", () => {
    // Without `data-chat-scroll` the hook cannot keep the list readable, and
    // a small phone shows the 24 px list from the parents' chat again.
    const srcDir = path.resolve(__dirname, "../..");
    const consumers: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) walk(full);
        else if (
          /\.tsx$/.test(entry.name) &&
          !/\.test\.tsx$/.test(entry.name) &&
          readFileSync(full, "utf8").includes("useChatViewportLock<")
        )
          consumers.push(full);
      }
    };
    walk(srcDir);

    expect(consumers.length).toBeGreaterThanOrEqual(3);
    const unmarked = consumers.filter(
      (file) => !readFileSync(file, "utf8").includes('data-chat-scroll=""'),
    );
    expect(unmarked).toEqual([]);
  });
});
