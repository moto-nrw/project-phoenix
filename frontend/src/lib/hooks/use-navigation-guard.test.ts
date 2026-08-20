import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, cleanup } from "@testing-library/react";

import { useNavigationGuard } from "./use-navigation-guard";
import { attemptNavigation } from "./navigation-guard-store";

const { mockPush, mockReplace } = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockReplace: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
}));

function mountAnchor(href: string, attrs: Record<string, string> = {}) {
  const anchor = document.createElement("a");
  anchor.setAttribute("href", href);
  for (const [k, v] of Object.entries(attrs)) anchor.setAttribute(k, v);
  document.body.appendChild(anchor);
  return anchor;
}

function clickAnchor(anchor: HTMLAnchorElement) {
  const event = new MouseEvent("click", { bubbles: true, cancelable: true });
  anchor.dispatchEvent(event);
  return event;
}

describe("useNavigationGuard", () => {
  beforeEach(() => {
    mockPush.mockReset();
    mockReplace.mockReset();
    window.history.replaceState(null, "", "/");
    vi.spyOn(window.history, "go").mockImplementation(() => undefined);
  });

  afterEach(() => {
    cleanup();
    document.body.innerHTML = "";
    vi.restoreAllMocks();
    window.history.replaceState(null, "", "/");
  });

  it("intercepts an in-app link click and stores the destination", () => {
    const { result } = renderHook(() => useNavigationGuard(true));
    const anchor = mountAnchor("/database");

    let event!: MouseEvent;
    act(() => {
      event = clickAnchor(anchor);
    });

    expect(event.defaultPrevented).toBe(true);
    expect(result.current.pendingHref).toBe("/database");
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("warns on hard unload (tab close / reload) while blocking", () => {
    renderHook(() => useNavigationGuard(true));

    const event = new Event("beforeunload", { cancelable: true });
    act(() => {
      window.dispatchEvent(event);
    });

    // preventDefault + a returnValue is what triggers the browser's native
    // "unsaved changes" confirmation dialog.
    expect(event.defaultPrevented).toBe(true);
  });

  it("does not warn on unload when not blocking", () => {
    renderHook(() => useNavigationGuard(false));

    const event = new Event("beforeunload", { cancelable: true });
    act(() => {
      window.dispatchEvent(event);
    });

    expect(event.defaultPrevented).toBe(false);
  });

  it("navigates on confirm and clears the pending destination", () => {
    const { result } = renderHook(() => useNavigationGuard(true));
    const anchor = mountAnchor("/database");

    act(() => {
      clickAnchor(anchor);
    });
    act(() => {
      result.current.confirmNavigation();
    });

    // router.replace, not push: it collapses the same-URL sentinel into the
    // target so Back from the destination lands on this page, not a duplicate.
    expect(mockReplace).toHaveBeenCalledWith("/database");
    expect(mockPush).not.toHaveBeenCalled();
    expect(result.current.pendingHref).toBeNull();
  });

  it("stays on the page on cancel without navigating", () => {
    const { result } = renderHook(() => useNavigationGuard(true));
    const anchor = mountAnchor("/database");

    act(() => {
      clickAnchor(anchor);
    });
    act(() => {
      result.current.cancelNavigation();
    });

    expect(result.current.pendingHref).toBeNull();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("does not intercept when not blocking", () => {
    const { result } = renderHook(() => useNavigationGuard(false));
    const anchor = mountAnchor("/database");

    let event!: MouseEvent;
    act(() => {
      event = clickAnchor(anchor);
    });

    expect(event.defaultPrevented).toBe(false);
    expect(result.current.pendingHref).toBeNull();
  });

  it("ignores modified clicks (new tab / window)", () => {
    const { result } = renderHook(() => useNavigationGuard(true));
    const anchor = mountAnchor("/database");

    act(() => {
      anchor.dispatchEvent(
        new MouseEvent("click", {
          bubbles: true,
          cancelable: true,
          metaKey: true,
        }),
      );
    });

    expect(result.current.pendingHref).toBeNull();
  });

  it("ignores download links and external targets", () => {
    const { result } = renderHook(() => useNavigationGuard(true));
    const download = mountAnchor("/file.pdf", { download: "" });
    const blank = mountAnchor("/elsewhere", { target: "_blank" });

    act(() => {
      clickAnchor(download);
      clickAnchor(blank);
    });

    expect(result.current.pendingHref).toBeNull();
  });

  it("ignores clicks that navigate to the current URL", () => {
    const { result } = renderHook(() => useNavigationGuard(true));
    const anchor = mountAnchor(
      window.location.pathname + window.location.search,
    );

    let event!: MouseEvent;
    act(() => {
      event = clickAnchor(anchor);
    });

    expect(event.defaultPrevented).toBe(false);
    expect(result.current.pendingHref).toBeNull();
  });

  describe("browser Back/Forward (popstate)", () => {
    function dispatchPopState() {
      window.dispatchEvent(new PopStateEvent("popstate"));
    }

    it("arms a history sentinel when blocking begins", () => {
      const pushSpy = vi.spyOn(window.history, "pushState");
      renderHook(() => useNavigationGuard(true));

      expect(pushSpy).toHaveBeenCalledTimes(1);
      pushSpy.mockRestore();
    });

    it("does not arm or listen when not blocking", () => {
      const pushSpy = vi.spyOn(window.history, "pushState");
      const { result } = renderHook(() => useNavigationGuard(false));

      act(() => {
        dispatchPopState();
      });

      expect(pushSpy).not.toHaveBeenCalled();
      expect(result.current.pendingHref).toBeNull();
      pushSpy.mockRestore();
    });

    it("intercepts a Back press and re-arms the sentinel", () => {
      const { result } = renderHook(() => useNavigationGuard(true));
      const pushSpy = vi.spyOn(window.history, "pushState");

      act(() => {
        dispatchPopState();
      });

      // The modal opens and the trap is re-armed so a second Back press is
      // still caught while the modal is open.
      expect(result.current.pendingHref).not.toBeNull();
      expect(pushSpy).toHaveBeenCalledTimes(1);
      pushSpy.mockRestore();
    });

    it("replays the history traversal on confirm", () => {
      const goSpy = vi.spyOn(window.history, "go").mockImplementation(() => {});
      const { result } = renderHook(() => useNavigationGuard(true));

      act(() => {
        dispatchPopState();
      });
      act(() => {
        result.current.confirmNavigation();
      });

      // Back/Forward has no target URL, so we traverse history rather than
      // router.push: sentinel → this page → the page the user wanted.
      expect(goSpy).toHaveBeenCalledWith(-2);
      expect(mockPush).not.toHaveBeenCalled();
      expect(result.current.pendingHref).toBeNull();
    });

    it("stays on the page on cancel without traversing history", () => {
      const goSpy = vi.spyOn(window.history, "go").mockImplementation(() => {});
      const { result } = renderHook(() => useNavigationGuard(true));

      act(() => {
        dispatchPopState();
      });
      act(() => {
        result.current.cancelNavigation();
      });

      expect(goSpy).not.toHaveBeenCalled();
      expect(result.current.pendingHref).toBeNull();
    });

    it("collapses the sentinel when blocking is disarmed in place", () => {
      const goSpy = vi.spyOn(window.history, "go").mockImplementation(() => {});
      const { rerender } = renderHook(
        ({ block }) => useNavigationGuard(block),
        { initialProps: { block: true } },
      );

      // Save/Discard clears the dirty state without leaving the page.
      act(() => {
        rerender({ block: false });
      });

      // The same-URL sentinel pushed on arm must be popped, otherwise the next
      // Back press lands on a leftover duplicate of the current URL.
      expect(goSpy).toHaveBeenCalledWith(-1);
    });

    it("does not accumulate sentinels across edit/save/edit cycles", () => {
      const goSpy = vi.spyOn(window.history, "go").mockImplementation(() => {});
      const pushSpy = vi.spyOn(window.history, "pushState");
      const { rerender } = renderHook(
        ({ block }) => useNavigationGuard(block),
        { initialProps: { block: true } },
      );

      act(() => {
        rerender({ block: false }); // save: pop the sentinel
      });
      act(() => {
        rerender({ block: true }); // edit again: arm a fresh sentinel
      });

      // One sentinel pushed per arm (initial + re-arm), one popped on disarm —
      // never a growing stack.
      expect(pushSpy).toHaveBeenCalledTimes(2);
      expect(goSpy).toHaveBeenCalledWith(-1);
      pushSpy.mockRestore();
    });

    it("does not pop a sentinel after a link-click confirm navigates away", () => {
      const goSpy = vi.spyOn(window.history, "go").mockImplementation(() => {});
      const { result, rerender } = renderHook(
        ({ block }) => useNavigationGuard(block),
        { initialProps: { block: true } },
      );
      const anchor = mountAnchor("/database");

      act(() => {
        clickAnchor(anchor);
      });
      act(() => {
        result.current.confirmNavigation();
      });
      // The destination page unmounts the guard; cleanup must not pop, because
      // router.replace already consumed the sentinel (the target now sits where
      // the sentinel was), so there is no leftover entry for cleanup to own.
      act(() => {
        rerender({ block: false });
      });

      expect(mockReplace).toHaveBeenCalledWith("/database");
      expect(goSpy).not.toHaveBeenCalled();
    });

    it("ignores the popstate it triggers itself on confirm", () => {
      vi.spyOn(window.history, "go").mockImplementation(() => {});
      const { result } = renderHook(() => useNavigationGuard(true));

      act(() => {
        dispatchPopState();
      });
      act(() => {
        result.current.confirmNavigation();
      });
      // The go(-2) above lands on the target page and fires one popstate; it
      // must not re-open the guard.
      act(() => {
        dispatchPopState();
      });

      expect(result.current.pendingHref).toBeNull();
      vi.restoreAllMocks();
    });
  });

  describe("programmatic navigation (useTenantRouter)", () => {
    it("intercepts a router.push and defers it without navigating", () => {
      const { result } = renderHook(() => useNavigationGuard(true));
      const proceed = vi.fn();

      let intercepted!: boolean;
      act(() => {
        intercepted = attemptNavigation(proceed, "/database");
      });

      // The registry reports the navigation was intercepted; the deferred
      // router call must not run until the user confirms.
      expect(intercepted).toBe(true);
      expect(result.current.pendingHref).toBe("/database");
      expect(proceed).not.toHaveBeenCalled();
    });

    it("runs the deferred navigation on confirm after collapsing the sentinel", () => {
      const goSpy = vi.spyOn(window.history, "go").mockImplementation(() => {});
      const { result } = renderHook(() => useNavigationGuard(true));
      const proceed = vi.fn();

      act(() => {
        attemptNavigation(proceed, "/database");
      });
      act(() => {
        result.current.confirmNavigation();
      });

      // Sentinel is popped (go(-1)); the deferred push runs from the bypassed
      // popstate the traversal fires, landing on a clean history stack.
      expect(goSpy).toHaveBeenCalledWith(-1);
      expect(result.current.pendingHref).toBeNull();
      expect(proceed).not.toHaveBeenCalled();

      act(() => {
        window.dispatchEvent(new PopStateEvent("popstate"));
      });
      expect(proceed).toHaveBeenCalledTimes(1);
    });

    it("does not run the deferred navigation on cancel", () => {
      const { result } = renderHook(() => useNavigationGuard(true));
      const proceed = vi.fn();

      act(() => {
        attemptNavigation(proceed, "/database");
      });
      act(() => {
        result.current.cancelNavigation();
      });

      expect(result.current.pendingHref).toBeNull();
      expect(proceed).not.toHaveBeenCalled();
    });

    it("does not intercept programmatic navigation when not blocking", () => {
      renderHook(() => useNavigationGuard(false));
      const proceed = vi.fn();

      const intercepted = attemptNavigation(proceed, "/database");

      expect(intercepted).toBe(false);
      expect(proceed).not.toHaveBeenCalled();
    });
  });
});
