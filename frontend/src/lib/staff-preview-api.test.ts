import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.fn();
const mockClearSessionCache = vi.fn();

vi.mock("~/lib/session-cache", () => ({
  sessionFetch: (...args: unknown[]) =>
    mockSessionFetch(...args) as Promise<Response>,
  clearSessionCache: () => mockClearSessionCache(),
}));

import {
  performEndStaffPreview,
  performStartStaffPreview,
} from "~/lib/staff-preview-api";

const swrMutate = vi.fn().mockResolvedValue(undefined);

describe("performStartStaffPreview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi
      .fn()
      .mockResolvedValue({ ok: true }) as unknown as typeof fetch;
  });

  it("closes the preview in the audit trail when the session swap fails", async () => {
    mockSessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        access_token: "preview-token",
        expires_in: 900,
        target_account_id: "42",
        target_name: "Anna Beispiel",
      }),
    });
    const update = vi
      .fn()
      .mockRejectedValue(new Error("session update failed"));

    await expect(
      performStartStaffPreview("42", update, swrMutate),
    ).rejects.toThrow("session update failed");

    expect(global.fetch).toHaveBeenCalledWith(
      "/api/auth/staff-preview/end",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ preview_token: "preview-token" }),
      }),
    );
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

    await expect(
      performEndStaffPreview("preview-token", update, swrMutate),
    ).rejects.toThrow("restore failed");

    expect(global.fetch).toHaveBeenCalledWith(
      "/api/auth/staff-preview/end",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ preview_token: "preview-token" }),
      }),
    );
  });

  it("records the end on the normal path", async () => {
    const update = vi.fn().mockResolvedValue(undefined);

    await performEndStaffPreview("preview-token", update, swrMutate);

    expect(update).toHaveBeenCalledWith({ previewEnd: true });
    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(swrMutate).toHaveBeenCalled();
  });

  it("never fails on a rejected audit call", async () => {
    global.fetch = vi
      .fn()
      .mockRejectedValue(new Error("offline")) as unknown as typeof fetch;
    const update = vi.fn().mockResolvedValue(undefined);

    await expect(
      performEndStaffPreview("preview-token", update, swrMutate),
    ).resolves.toBeUndefined();
  });
});
