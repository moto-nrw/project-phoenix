import { beforeEach, describe, expect, it, vi } from "vitest";

const sessionFetch = vi.hoisted(() => vi.fn());
vi.mock("./session-cache", () => ({ sessionFetch }));

import { substitutionService } from "./substitution-api";

const handover = {
  id: "5",
  type: "group_handover" as const,
  group: { id: "12", name: "Robins Gruppe" },
  target: {
    id: "34",
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

  it("loads one running supervision with its available targets", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          group_handovers: [],
          targets: [],
          running_supervisions: [
            {
              id: "41",
              type: "additional_supervision",
              name: "Freispiel",
              room_name: "Atelier",
              supervisors: [{ id: "11", full_name: "Alex Alt" }],
              available_targets: [{ id: "73", full_name: "Toni Test" }],
              is_current_user_supervising: true,
              can_assign: true,
            },
          ],
        },
      }),
    });

    const result = await substitutionService.fetchRunningSupervision("41");

    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/substitutions?active_group_id=41",
      { credentials: "include" },
    );
    expect(result).toEqual({
      id: "41",
      name: "Freispiel",
      roomName: "Atelier",
      supervisors: [{ id: "11", fullName: "Alex Alt" }],
      availableTargets: [{ id: "73", fullName: "Toni Test" }],
      isCurrentUserSupervising: true,
      canAssign: true,
    });
  });

  it("adds a supervisor with only the two allowed ids", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          id: "91",
          type: "additional_supervision",
          active_group_id: "41",
          target: { id: "73", full_name: "Toni Test" },
        },
      }),
    });

    const result = await substitutionService.addSupervisor("41", "73");

    expect(sessionFetch).toHaveBeenCalledWith("/api/substitutions", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        type: "additional_supervision",
        additional_supervision: {
          active_group_id: 41,
          target_staff_id: 73,
        },
      }),
    });
    expect(result).toEqual({ id: "91", targetName: "Toni Test" });
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
