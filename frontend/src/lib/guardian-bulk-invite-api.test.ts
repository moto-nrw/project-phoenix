import { afterEach, describe, expect, it, vi } from "vitest";
import { bulkInviteGuardians } from "./guardian-bulk-invite-api";

function mockFetch(status: number, body: unknown) {
  const fetchMock = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("bulkInviteGuardians", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("sends numeric student IDs and maps the counts to camelCase", async () => {
    const fetchMock = mockFetch(200, {
      status: "success",
      data: {
        dry_run: true,
        invited: 3,
        linked_existing_account: 1,
        resent: 2,
        skipped_active: 4,
        skipped_open: 5,
        skipped_restricted: 6,
        problems: [
          {
            guardian_profile_id: "7",
            guardian_name: "Katharina Brenner",
            student_names: ["Mia Brenner"],
            reason: "missing_email",
          },
        ],
      },
    });

    const result = await bulkInviteGuardians(["4", "9"], {
      dryRun: true,
      resendOpen: true,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/guardians/bulk-invite",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          student_ids: [4, 9],
          dry_run: true,
          resend_open: true,
        }),
      }),
    );
    expect(result).toEqual({
      dryRun: true,
      invited: 3,
      linkedExistingAccount: 1,
      resent: 2,
      skippedActive: 4,
      skippedOpen: 5,
      skippedRestricted: 6,
      problems: [
        {
          guardianProfileId: "7",
          guardianName: "Katharina Brenner",
          studentNames: ["Mia Brenner"],
          reason: "missing_email",
        },
      ],
    });
  });

  it("throws on a failed request and on an error envelope", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => undefined);
    mockFetch(403, {});
    await expect(
      bulkInviteGuardians(["4"], { dryRun: false, resendOpen: false }),
    ).rejects.toThrow("403");

    mockFetch(200, { status: "error", error: "nope" });
    await expect(
      bulkInviteGuardians(["4"], { dryRun: false, resendOpen: false }),
    ).rejects.toThrow("nope");
  });
});
