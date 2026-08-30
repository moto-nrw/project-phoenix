import { beforeEach, describe, expect, it, vi } from "vitest";

const sessionFetch = vi.hoisted(() => vi.fn());
vi.mock("./session-cache", () => ({ sessionFetch }));

import { groupTransferService } from "./group-transfer-api";

describe("groupTransferService", () => {
  beforeEach(() => vi.clearAllMocks());

  it("assigns a typed group handover", async () => {
    sessionFetch.mockResolvedValue({ ok: true });

    await groupTransferService.transferGroup("12", "34");

    expect(sessionFetch).toHaveBeenCalledWith("/api/substitutions", {
      method: "POST",
      body: JSON.stringify({
        type: "group_handover",
        group_handover: { group_id: 12, target_staff_id: 34 },
      }),
    });
  });

  it("maps the narrow overview projection", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        targets: [],
        group_handovers: [
          {
            id: 5,
            group: { id: 12 },
            target: { id: 34, full_name: "Toni Test" },
            period: { end_date: "2026-08-29" },
          },
        ],
      }),
    });

    await expect(
      groupTransferService.getActiveTransfersForGroup("12"),
    ).resolves.toEqual([
      {
        substitutionId: "5",
        groupId: "12",
        targetStaffId: "34",
        targetName: "Toni Test",
        validUntil: "2026-08-29",
      },
    ]);
  });

  it("rejects malformed target data instead of showing nobody", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ targets: null, group_handovers: [] }),
    });

    await expect(groupTransferService.getAllAvailableStaff()).rejects.toThrow(
      "Ungültige Antwort",
    );
  });

  it("rejects malformed group-leader candidate data", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ data: null }),
    });

    await expect(groupTransferService.getStaffByRole("user")).rejects.toThrow(
      "Ungültige Antwort",
    );
  });

  it("does not hide overview failures as an empty list", async () => {
    sessionFetch.mockResolvedValue({
      ok: false,
      status: 403,
      json: async () => ({ error: "Keine Berechtigung" }),
    });

    await expect(
      groupTransferService.getActiveTransfersForGroup("12"),
    ).rejects.toThrow("Keine Berechtigung");
  });

  it("ends a typed group handover", async () => {
    sessionFetch.mockResolvedValue({ ok: true });

    await groupTransferService.cancelTransferBySubstitutionId("5");

    expect(sessionFetch).toHaveBeenCalledWith("/api/substitutions/end", {
      method: "POST",
      body: JSON.stringify({ type: "group_handover", id: 5 }),
    });
  });
});
