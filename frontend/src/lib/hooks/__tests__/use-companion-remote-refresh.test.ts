import { describe, it, expect, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useCompanionRemoteRefresh } from "~/lib/hooks/use-companion-remote-refresh";
import { notifyStudentCompanionsChanged } from "~/lib/student-companion-api";

/**
 * The suppression of a view's OWN write is the part that decides whether a user
 * keeps their work, so it is tested against the real ordering the backend
 * produces: the response is streamed from the handler, the commit and its
 * after-commit broadcast follow, so the SSE echo lands AFTER the save promise
 * has already resolved.
 */
describe("useCompanionRemoteRefresh", () => {
  it("refetches on the SSE echo of its own save without flagging it stale", async () => {
    const onRefresh = vi.fn();
    const { result } = renderHook(() =>
      useCompanionRemoteRefresh({
        active: true,
        // The draft still differs from the loaded snapshot until the caller's
        // reload lands — exactly the window in which a mis-attributed echo
        // would block the save the user just completed.
        hasUnsavedCompanionEdits: true,
        onRefresh,
      }),
    );

    await act(async () => {
      // The save itself announces on the in-tab bus while it is still running.
      await result.current.withOwnWrite(() => {
        notifyStudentCompanionsChanged();
        return Promise.resolve("saved");
      }, true);
    });

    // The backend's own event, delivered over SSE after the commit.
    act(() => {
      notifyStudentCompanionsChanged();
    });

    // No conflict warning for the user's own change — but the echo IS the first
    // signal that runs after the commit, while the caller's own reload starts on
    // a response the backend streams before it. So it refetches: the announcement
    // during the save is suppressed outright, the post-commit one reloads.
    expect(result.current.companionsStale).toBe(false);
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it("reloads on its own echo even while the draft still differs", async () => {
    const onRefresh = vi.fn();
    const { result } = renderHook(() =>
      useCompanionRemoteRefresh({
        active: true,
        // The caller's post-save reload has not landed yet, so the draft is
        // still measured against the pre-save baseline. Skipping the refetch
        // here is what would leave the form recording the PRE-commit list as
        // its new snapshot — the reload it started itself may well have read
        // the links before the outer transaction committed.
        hasUnsavedCompanionEdits: true,
        onRefresh,
      }),
    );

    await act(async () => {
      await result.current.withOwnWrite(() => Promise.resolve("saved"), true);
    });

    act(() => {
      notifyStudentCompanionsChanged();
    });

    expect(onRefresh).toHaveBeenCalledTimes(1);
    expect(result.current.companionsStale).toBe(false);
  });

  it("still flags a remote write that follows its own echo", async () => {
    const onRefresh = vi.fn();
    const { result } = renderHook(() =>
      useCompanionRemoteRefresh({
        active: true,
        hasUnsavedCompanionEdits: true,
        onRefresh,
      }),
    );

    await act(async () => {
      await result.current.withOwnWrite(() => Promise.resolve("saved"), true);
    });

    act(() => {
      notifyStudentCompanionsChanged(); // own echo, consumed
    });
    act(() => {
      notifyStudentCompanionsChanged(); // somebody else
    });

    expect(result.current.companionsStale).toBe(true);
  });

  it("does not swallow a remote write after a save that cannot announce", async () => {
    const onRefresh = vi.fn();
    const { result } = renderHook(() =>
      useCompanionRemoteRefresh({
        active: true,
        hasUnsavedCompanionEdits: true,
        onRefresh,
      }),
    );

    // A successful save that touched neither the links nor the departure
    // plan: the backend answers it without a broadcast, so the next
    // announcement can only be somebody else's change.
    await act(async () => {
      await result.current.withOwnWrite(() => Promise.resolve("saved"), false);
    });

    act(() => {
      notifyStudentCompanionsChanged();
    });

    expect(result.current.companionsStale).toBe(true);
  });

  it("does not swallow a remote write after a failed save", async () => {
    const onRefresh = vi.fn();
    const { result } = renderHook(() =>
      useCompanionRemoteRefresh({
        active: true,
        hasUnsavedCompanionEdits: true,
        onRefresh,
      }),
    );

    // A refused save produces no echo — the backend broadcasts only after a
    // commit. The announcement that follows is a genuine remote change and
    // must flag the draft stale instead of being consumed as an echo.
    await act(async () => {
      await expect(
        result.current.withOwnWrite(
          () => Promise.reject(new Error("save failed")),
          true,
        ),
      ).rejects.toThrow("save failed");
    });

    act(() => {
      notifyStudentCompanionsChanged();
    });

    expect(result.current.companionsStale).toBe(true);
  });

  it("refetches instead of flagging while nothing is edited", () => {
    const onRefresh = vi.fn();
    const { result } = renderHook(() =>
      useCompanionRemoteRefresh({
        active: true,
        hasUnsavedCompanionEdits: false,
        onRefresh,
      }),
    );

    act(() => {
      notifyStudentCompanionsChanged();
    });

    expect(onRefresh).toHaveBeenCalledTimes(1);
    expect(result.current.companionsStale).toBe(false);
  });
});
