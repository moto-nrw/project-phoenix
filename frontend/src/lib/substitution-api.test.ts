import { beforeEach, describe, expect, it, vi } from "vitest";

const sessionFetch = vi.hoisted(() => vi.fn());
vi.mock("./session-cache", () => ({ sessionFetch }));

import { substitutionService } from "./substitution-api";

const handover = {
  id: 5,
  type: "group_handover" as const,
  group: { id: 12, name: "Robins Gruppe" },
  target: {
    id: 34,
    full_name: "Toni Test",
  },
  period: { start_date: "2026-08-29", end_date: "2026-08-30" },
  can_end: true,
};

describe("substitutionService", () => {
  beforeEach(() => vi.clearAllMocks());

  it("maps the proxy-wrapped overview response", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: { group_handovers: [handover], targets: [] },
      }),
    });

    const result = await substitutionService.fetchSubstitutions();

    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({
      id: "5",
      type: "group_handover",
      groupId: "12",
      groupName: "Robins Gruppe",
      substituteStaffId: "34",
      substituteStaffName: "Toni Test",
      canEnd: true,
    });
  });

  it("rejects a malformed overview instead of showing an empty plan", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: { group_handovers: null, targets: [] },
      }),
    });

    await expect(substitutionService.fetchSubstitutions()).rejects.toThrow(
      "Ungültige Antwort",
    );
  });

  it("assigns and maps a typed group handover", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ data: handover }),
    });

    const result = await substitutionService.createSubstitution(
      "12",
      "34",
      "2026-08-29",
      "2026-08-30",
    );

    expect(sessionFetch).toHaveBeenCalledWith("/api/substitutions", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        type: "group_handover",
        group_handover: {
          group_id: 12,
          target_staff_id: 34,
          start_date: "2026-08-29",
          end_date: "2026-08-30",
        },
      }),
    });
    expect(result.id).toBe("5");
  });

  it("rejects a missing proxy envelope", async () => {
    sessionFetch.mockResolvedValue({ ok: true, json: async () => ({}) });

    await expect(substitutionService.fetchSubstitutions()).rejects.toThrow(
      "Ungültige Antwort",
    );
  });

  it("ends a typed group handover", async () => {
    sessionFetch.mockResolvedValue({ ok: true });

    await substitutionService.deleteSubstitution("5");

    expect(sessionFetch).toHaveBeenCalledWith("/api/substitutions/end", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({ type: "group_handover", id: 5 }),
    });
  });
});
