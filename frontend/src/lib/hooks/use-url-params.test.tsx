import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockSearch } = vi.hoisted(() => ({ mockSearch: { value: "" } }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(mockSearch.value),
}));

import { useUrlParams } from "./use-url-params";

const ALLOWED = ["d", "block", "verlauf"] as const;

describe("useUrlParams", () => {
  beforeEach(() => {
    mockSearch.value = "";
    window.history.replaceState(null, "", "/acme/vertretung");
  });

  it("reads a snapshot of only the allowed keys, null when absent", () => {
    mockSearch.value = "d=2026-07-15&utm_source=x";
    const { result } = renderHook(() => useUrlParams(ALLOWED));

    expect(result.current.params).toEqual({
      d: "2026-07-15",
      block: null,
      verlauf: null,
    });
  });

  it("updateParams sets keys via window.history.replaceState", () => {
    const { result } = renderHook(() => useUrlParams(ALLOWED));

    act(() => {
      result.current.updateParams({ d: "2026-07-16" });
    });

    expect(new URLSearchParams(window.location.search).get("d")).toBe(
      "2026-07-16",
    );
  });

  it("updateParams deletes a key when the patch value is null", () => {
    mockSearch.value = "d=2026-07-15&block=42";
    window.history.replaceState(
      null,
      "",
      "/acme/vertretung?d=2026-07-15&block=42",
    );
    const { result } = renderHook(() => useUrlParams(ALLOWED));

    act(() => {
      result.current.updateParams({ block: null });
    });

    const qs = new URLSearchParams(window.location.search);
    expect(qs.get("d")).toBe("2026-07-15");
    expect(qs.has("block")).toBe(false);
  });

  it("updateParams deletes a key when the patch value is an empty string", () => {
    mockSearch.value = "d=2026-07-15&block=42";
    window.history.replaceState(
      null,
      "",
      "/acme/vertretung?d=2026-07-15&block=42",
    );
    const { result } = renderHook(() => useUrlParams(ALLOWED));

    act(() => {
      result.current.updateParams({ block: "" });
    });

    expect(new URLSearchParams(window.location.search).has("block")).toBe(
      false,
    );
  });

  it("rebuilds the URL from the allowlist — unrelated params never survive an update", () => {
    window.history.replaceState(
      null,
      "",
      "/acme/vertretung?d=2026-07-15&utm_source=x&foo=bar",
    );
    const { result } = renderHook(() => useUrlParams(ALLOWED));

    act(() => {
      result.current.updateParams({ block: "42" });
    });

    const keys = [...new URLSearchParams(window.location.search).keys()].sort();
    expect(keys).toEqual(["block", "d"]);
  });

  it("writes no history entry — replaceState, not pushState", () => {
    const pushSpy = vi.spyOn(window.history, "pushState");
    const { result } = renderHook(() => useUrlParams(ALLOWED));

    act(() => {
      result.current.updateParams({ d: "2026-07-16" });
    });

    expect(pushSpy).not.toHaveBeenCalled();
    pushSpy.mockRestore();
  });

  it("drops the query string entirely when the patch clears the only allowed key", () => {
    window.history.replaceState(null, "", "/acme/vertretung?d=2026-07-15");
    const { result } = renderHook(() => useUrlParams(["d"] as const));

    act(() => {
      result.current.updateParams({ d: null });
    });

    expect(window.location.search).toBe("");
    expect(window.location.pathname).toBe("/acme/vertretung");
  });

  describe("syncPopstate", () => {
    it("does not re-read on popstate when the option is off (default)", () => {
      mockSearch.value = "d=2026-07-15";
      const { result } = renderHook(() => useUrlParams(ALLOWED));
      expect(result.current.params.d).toBe("2026-07-15");

      // Change the real URL directly (bypassing the mocked useSearchParams,
      // which stays fixed at mockSearch.value) and fire popstate.
      window.history.replaceState(null, "", "/acme/vertretung?d=2026-07-20");
      act(() => {
        window.dispatchEvent(new PopStateEvent("popstate"));
      });

      // Still reflects the (unmocked-unchanged) next/navigation snapshot.
      expect(result.current.params.d).toBe("2026-07-15");
    });

    it("re-reads window.location.search on popstate when the option is on", () => {
      window.history.replaceState(null, "", "/acme/vertretung?d=2026-07-15");
      const { result } = renderHook(() =>
        useUrlParams(ALLOWED, { syncPopstate: true }),
      );
      expect(result.current.params.d).toBe("2026-07-15");

      window.history.replaceState(null, "", "/acme/vertretung?d=2026-07-20");
      act(() => {
        window.dispatchEvent(new PopStateEvent("popstate"));
      });

      expect(result.current.params.d).toBe("2026-07-20");
    });

    it("removes the popstate listener on unmount", () => {
      const removeSpy = vi.spyOn(window, "removeEventListener");
      const { unmount } = renderHook(() =>
        useUrlParams(ALLOWED, { syncPopstate: true }),
      );
      unmount();
      expect(removeSpy).toHaveBeenCalledWith("popstate", expect.any(Function));
      removeSpy.mockRestore();
    });
  });

  it("is SSR-safe: updateParams no-ops without throwing when window is undefined", () => {
    const { result } = renderHook(() => useUrlParams(ALLOWED));
    const originalWindow = globalThis.window;
    // @ts-expect-error -- simulate an SSR environment for this single call
    delete globalThis.window;
    try {
      expect(() =>
        result.current.updateParams({ d: "2026-07-16" }),
      ).not.toThrow();
    } finally {
      globalThis.window = originalWindow;
    }
  });
});
