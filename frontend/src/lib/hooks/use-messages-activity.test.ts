import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useMessagesActivity } from "./use-messages-activity";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function dispatchActivity(
  detail: {
    threadId?: string | null;
    studentId?: string | null;
    source?: string | null;
  } = {},
  eventName = "messages-activity",
) {
  window.dispatchEvent(new CustomEvent(eventName, { detail }));
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  // Ensure tab is visible by default
  Object.defineProperty(document, "hidden", {
    configurable: true,
    get: () => false,
  });
});

afterEach(() => {
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// Basic event handling
// ---------------------------------------------------------------------------

describe("useMessagesActivity — basic dispatch", () => {
  it("calls onMatch when a messages-activity event fires with no id filters", () => {
    const onMatch = vi.fn();
    renderHook(() => useMessagesActivity({ onMatch, marksRead: false }));

    act(() => {
      dispatchActivity({});
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("calls onMatch when event detail is empty (null ids match everything)", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, threadId: "t1", marksRead: false }),
    );

    act(() => {
      // detail has no threadId — null/missing ALWAYS matches
      dispatchActivity({});
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("does NOT call onMatch when a DIFFERENT threadId is specified in the event", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, threadId: "t1", marksRead: false }),
    );

    act(() => {
      dispatchActivity({ threadId: "t2" }); // different thread
    });

    expect(onMatch).not.toHaveBeenCalled();
  });

  it("calls onMatch when the event threadId MATCHES the consumer threadId", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, threadId: "t1", marksRead: false }),
    );

    act(() => {
      dispatchActivity({ threadId: "t1" });
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("skips when a DIFFERENT studentId is in the event", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, studentId: "s1", marksRead: false }),
    );

    act(() => {
      dispatchActivity({ studentId: "s2" });
    });

    expect(onMatch).not.toHaveBeenCalled();
  });

  it("calls onMatch when the event studentId matches the consumer studentId", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, studentId: "s1", marksRead: false }),
    );

    act(() => {
      dispatchActivity({ studentId: "s1" });
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("removes the event listener on unmount", () => {
    const onMatch = vi.fn();
    const { unmount } = renderHook(() =>
      useMessagesActivity({ onMatch, marksRead: false }),
    );

    unmount();

    act(() => {
      dispatchActivity({});
    });

    expect(onMatch).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Custom eventName
// ---------------------------------------------------------------------------

describe("useMessagesActivity — custom eventName", () => {
  it("listens on the supplied custom event name", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({
        onMatch,
        eventName: "parent-conversation-refresh",
        marksRead: false,
      }),
    );

    act(() => {
      window.dispatchEvent(
        new CustomEvent("parent-conversation-refresh", { detail: {} }),
      );
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("does NOT fire on the default event name when a custom one is given", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({
        onMatch,
        eventName: "parent-conversation-refresh",
        marksRead: false,
      }),
    );

    act(() => {
      dispatchActivity({}); // fires "messages-activity"
    });

    expect(onMatch).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Debounce
// ---------------------------------------------------------------------------

describe("useMessagesActivity — debounce", () => {
  it("delays the callback by debounceMs", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, debounceMs: 300, marksRead: false }),
    );

    act(() => {
      dispatchActivity({});
    });

    expect(onMatch).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("collapses a burst of events into one call", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, debounceMs: 200, marksRead: false }),
    );

    act(() => {
      dispatchActivity({});
      dispatchActivity({});
      dispatchActivity({});
    });

    act(() => {
      vi.advanceTimersByTime(200);
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });
});

// ---------------------------------------------------------------------------
// Hidden-tab guard (marksRead: true)
// ---------------------------------------------------------------------------

describe("useMessagesActivity — hidden-tab guard", () => {
  it("defers the callback when the tab is hidden and marksRead is true", () => {
    const onMatch = vi.fn();

    Object.defineProperty(document, "hidden", {
      configurable: true,
      get: () => true,
    });

    renderHook(() => useMessagesActivity({ onMatch, marksRead: true }));

    act(() => {
      dispatchActivity({});
    });

    expect(onMatch).not.toHaveBeenCalled();
  });

  it("flushes the deferred callback when the tab becomes visible (visibilitychange)", () => {
    const onMatch = vi.fn();

    Object.defineProperty(document, "hidden", {
      configurable: true,
      get: () => true,
    });

    renderHook(() => useMessagesActivity({ onMatch, marksRead: true }));

    act(() => {
      dispatchActivity({});
    });

    expect(onMatch).not.toHaveBeenCalled();

    // Tab becomes visible
    Object.defineProperty(document, "hidden", {
      configurable: true,
      get: () => false,
    });

    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("fires immediately when tab is visible and marksRead is true", () => {
    const onMatch = vi.fn();
    renderHook(() => useMessagesActivity({ onMatch, marksRead: true }));

    act(() => {
      dispatchActivity({});
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("fires immediately even when tab is hidden when marksRead is false", () => {
    const onMatch = vi.fn();

    Object.defineProperty(document, "hidden", {
      configurable: true,
      get: () => true,
    });

    renderHook(() => useMessagesActivity({ onMatch, marksRead: false }));

    act(() => {
      dispatchActivity({});
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });
});

// ---------------------------------------------------------------------------
// refetchOnFocus
// ---------------------------------------------------------------------------

describe("useMessagesActivity — refetchOnFocus", () => {
  it("calls onMatch on window focus when refetchOnFocus is true", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, refetchOnFocus: true, marksRead: false }),
    );

    act(() => {
      window.dispatchEvent(new Event("focus"));
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("does NOT call onMatch on window focus when refetchOnFocus is false (default)", () => {
    const onMatch = vi.fn();
    renderHook(() => useMessagesActivity({ onMatch, marksRead: false }));

    act(() => {
      window.dispatchEvent(new Event("focus"));
    });

    expect(onMatch).not.toHaveBeenCalled();
  });

  it("removes the focus listener on unmount", () => {
    const onMatch = vi.fn();
    const { unmount } = renderHook(() =>
      useMessagesActivity({ onMatch, refetchOnFocus: true, marksRead: false }),
    );

    unmount();

    act(() => {
      window.dispatchEvent(new Event("focus"));
    });

    expect(onMatch).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// ignoreOwnSource — the read-advancing consumers' own-write filter
// ---------------------------------------------------------------------------

describe("useMessagesActivity — ignoreOwnSource", () => {
  it("skips an event this account caused itself", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, marksRead: false, ignoreOwnSource: "7" }),
    );

    act(() => {
      dispatchActivity({ source: "7" });
    });

    expect(onMatch).not.toHaveBeenCalled();
  });

  it("still fires for an event another account caused", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, marksRead: false, ignoreOwnSource: "7" }),
    );

    act(() => {
      dispatchActivity({ source: "42" });
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("fires for a consumer that does NOT set ignoreOwnSource, even for its own source", () => {
    // The inbox is refetch-only: suppressing its own writes would leave its
    // preview, ordering and unread pill stale after the user's own send.
    const onMatch = vi.fn();
    renderHook(() => useMessagesActivity({ onMatch, marksRead: false }));

    act(() => {
      dispatchActivity({ source: "7" });
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });

  it("fires when the event carries no source at all", () => {
    const onMatch = vi.fn();
    renderHook(() =>
      useMessagesActivity({ onMatch, marksRead: false, ignoreOwnSource: "7" }),
    );

    act(() => {
      dispatchActivity({});
    });

    expect(onMatch).toHaveBeenCalledTimes(1);
  });
});
