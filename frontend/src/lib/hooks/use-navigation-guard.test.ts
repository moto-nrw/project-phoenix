import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

import { useNavigationGuard } from "./use-navigation-guard";

const { mockPush } = vi.hoisted(() => ({ mockPush: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
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
  });

  afterEach(() => {
    document.body.innerHTML = "";
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

  it("navigates on confirm and clears the pending destination", () => {
    const { result } = renderHook(() => useNavigationGuard(true));
    const anchor = mountAnchor("/database");

    act(() => {
      clickAnchor(anchor);
    });
    act(() => {
      result.current.confirmNavigation();
    });

    expect(mockPush).toHaveBeenCalledWith("/database");
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
      goSpy.mockRestore();
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
      goSpy.mockRestore();
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
      goSpy.mockRestore();
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
      goSpy.mockRestore();
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
      // router.push already left a new entry on top of the sentinel.
      act(() => {
        rerender({ block: false });
      });

      expect(mockPush).toHaveBeenCalledWith("/database");
      expect(goSpy).not.toHaveBeenCalled();
      goSpy.mockRestore();
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
});
