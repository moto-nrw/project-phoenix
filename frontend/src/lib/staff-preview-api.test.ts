import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.fn();
const mockClearSessionCache = vi.fn();
const mockSignOut = vi.fn().mockResolvedValue(undefined);

vi.mock("~/lib/session-cache", () => ({
  sessionFetch: (...args: unknown[]) =>
    mockSessionFetch(...args) as Promise<Response>,
  clearSessionCache: () => mockClearSessionCache(),
}));

vi.mock("next-auth/react", () => ({
  signOut: (...args: unknown[]) => mockSignOut(...args) as Promise<undefined>,
}));

const previewSession = (targetAccountId: string | null) => ({
  user: {
    isPreview: targetAccountId !== null,
    previewTargetAccountId: targetAccountId ?? undefined,
  },
});

import {
  performEndStaffPreview,
  performStartStaffPreview,
} from "~/lib/staff-preview-api";

const swrMutate = vi.fn().mockResolvedValue(undefined);

describe("performStartStaffPreview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  it("reads only allowlisted metadata and clears caches after the server swap", async () => {
    mockSessionFetch.mockResolvedValue(
      Response.json({
        expires_in: 900,
        target_account_id: "42",
        target_name: "Anna Beispiel",
      }),
    );
    await expect(performStartStaffPreview("42", swrMutate)).resolves.toEqual({
      expiresIn: 900,
      targetAccountId: "42",
      targetName: "Anna Beispiel",
    });
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/auth/staff-preview",
      expect.objectContaining({ body: JSON.stringify({ account_id: "42" }) }),
    );
    expect(swrMutate).toHaveBeenCalledWith(expect.any(Function), undefined, {
      revalidate: false,
    });
    expect(mockClearSessionCache).toHaveBeenCalledOnce();
  });
  it("leaves caches alone when the BFF rejects the start or its session swap", async () => {
    mockSessionFetch.mockResolvedValue(
      Response.json({ code: "general.server" }, { status: 500 }),
    );
    await expect(
      performStartStaffPreview("42", swrMutate),
    ).rejects.toMatchObject({ status: 500 });
    expect(mockSessionFetch).toHaveBeenCalledOnce();
    expect(swrMutate).not.toHaveBeenCalled();
    expect(mockClearSessionCache).not.toHaveBeenCalled();
  });
});

describe("performEndStaffPreview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi
      .fn()
      .mockResolvedValue({ ok: true }) as unknown as typeof fetch;
  });

  it("records the end even when restoring the admin session fails", async () => {
    const update = vi.fn().mockRejectedValue(new Error("restore failed"));

    await expect(performEndStaffPreview(update, swrMutate)).rejects.toThrow(
      "restore failed",
    );

    expect(global.fetch).toHaveBeenCalledWith(
      "/api/auth/staff-preview/end",
      expect.objectContaining({
        method: "POST",
        // The route reads the preview token from the session cookie (#5005).
        body: JSON.stringify({}),
      }),
    );
  });

  it("records the end on the normal path", async () => {
    const update = vi.fn().mockResolvedValue(undefined);

    await performEndStaffPreview(update, swrMutate);

    expect(update).toHaveBeenCalledWith({ previewEnd: true });
    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(swrMutate).toHaveBeenCalled();
  });

  it.each(["success", "failure"] as const)(
    "settles the audit call before restoring the session can navigate (%s)",
    async (outcome) => {
      let finishAudit!: () => void;
      const audit = new Promise<Response>((resolve, reject) => {
        finishAudit = () => {
          if (outcome === "success")
            resolve(new Response(null, { status: 204 }));
          else reject(new Error("offline"));
        };
      });
      vi.mocked(global.fetch).mockReturnValue(audit);
      // Updating the session can immediately make PersonalStorageGuard leave
      // the page. No audit request may still be pending at that point.
      const update = vi.fn().mockResolvedValue(previewSession(null));

      const ending = performEndStaffPreview(update, swrMutate);
      await Promise.resolve();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/auth/staff-preview/end",
        expect.objectContaining({
          body: JSON.stringify({}),
          // Bounded: a stalled audit call cannot hold the restore forever.
          signal: expect.any(AbortSignal) as AbortSignal,
        }),
      );
      expect(update).not.toHaveBeenCalled();
      finishAudit();
      await ending;

      expect(update).toHaveBeenCalledWith({ previewEnd: true });
      expect(swrMutate).toHaveBeenCalled();
    },
  );

  it("signs out when the session stayed in the preview", async () => {
    // Restore silently did nothing: the trail says ended, the cookie would
    // still render as the target — that state must not survive.
    const update = vi.fn().mockResolvedValue(previewSession("42"));

    await expect(performEndStaffPreview(update, swrMutate)).rejects.toThrow(
      "preview session was not restored",
    );

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    expect(swrMutate).not.toHaveBeenCalled();
  });

  it("signs out when restoring the admin session fails", async () => {
    const update = vi.fn().mockRejectedValue(new Error("restore failed"));

    await expect(performEndStaffPreview(update, swrMutate)).rejects.toThrow(
      "restore failed",
    );

    expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
  });

  it("never fails on a rejected audit call", async () => {
    global.fetch = vi
      .fn()
      .mockRejectedValue(new Error("offline")) as unknown as typeof fetch;
    const update = vi.fn().mockResolvedValue(undefined);

    await expect(
      performEndStaffPreview(update, swrMutate),
    ).resolves.toBeUndefined();
  });
});
