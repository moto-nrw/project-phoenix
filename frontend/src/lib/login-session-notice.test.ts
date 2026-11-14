import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { signOut } = vi.hoisted(() => ({ signOut: vi.fn() }));
vi.mock("next-auth/react", () => ({ signOut }));

import { signOutStaleSession } from "./login-session-notice";

const stale = { error: "RefreshTokenExpired" } as unknown as Session;
const healthy = { user: { id: "1" } } as unknown as Session;

describe("signOutStaleSession", () => {
  const logger = { warn: vi.fn() };

  beforeEach(() => {
    signOut.mockReset();
    logger.warn.mockReset();
  });

  it("leaves a page without a stale session to its other checks at once", () => {
    const started = { current: false };
    for (const [status, session] of [
      ["loading", null],
      ["unauthenticated", null],
      ["authenticated", healthy],
    ] as const) {
      expect(
        signOutStaleSession({ status, session, started, logger }),
      ).toBeNull();
    }
    expect(signOut).not.toHaveBeenCalled();
    expect(started.current).toBe(false);
  });

  it("signs a stale session out once, without leaving the page", async () => {
    signOut.mockResolvedValue(undefined);
    const started = { current: false };
    const input = { status: "authenticated", session: stale, started, logger };

    const cleanup = signOutStaleSession({ ...input, status: "authenticated" });
    expect(signOutStaleSession({ ...input, status: "authenticated" })).toBeNull();
    await cleanup;

    expect(signOut).toHaveBeenCalledTimes(1);
    expect(signOut).toHaveBeenCalledWith({ redirect: false });
    expect(started.current).toBe(true);
    expect(logger.warn).not.toHaveBeenCalled();
  });

  it("reports a failed sign-out and lets a later call try again", async () => {
    const cause = new Error("offline");
    signOut.mockRejectedValueOnce(cause).mockResolvedValueOnce(undefined);
    const started = { current: false };
    const onFailure = vi.fn();

    await signOutStaleSession({
      status: "authenticated",
      session: stale,
      started,
      logger,
      onFailure,
    });

    expect(onFailure).toHaveBeenCalledWith(cause);
    expect(logger.warn).toHaveBeenCalledWith("stale_session_signout_failed", {
      error: "offline",
    });
    expect(started.current).toBe(false);
    await signOutStaleSession({
      status: "authenticated",
      session: stale,
      started,
      logger,
    });
    expect(signOut).toHaveBeenCalledTimes(2);
  });
});
