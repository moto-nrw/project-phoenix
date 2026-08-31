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

  it("loads only narrow schedule substitutions and staff targets", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          group_handovers: [],
          targets: [],
          schedule_appointments: [
            {
              id: 42,
              type: "schedule_substitution",
              date: "2026-08-31",
              start_time: "12:00",
              end_time: "13:00",
              title: "Mensa",
              status: "planned",
              staff: [
                {
                  assignment_id: 9,
                  staff: { id: 7, full_name: "Ada Alt" },
                  is_absent: true,
                  is_substitute: false,
                  can_end: false,
                },
              ],
            },
          ],
          schedule_targets: [{ id: 8, full_name: "Nora Neu" }],
        },
      }),
    });

    const result = await substitutionService.fetchScheduleOverview(
      "2026-08-31",
      "2026-09-04",
    );

    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/substitutions?from=2026-08-31&to=2026-09-04",
      { credentials: "include" },
    );
    expect(result.staff).toEqual([{ id: "8", name: "Nora Neu" }]);
    expect(result.appointments[0]).toMatchObject({
      id: "42",
      staff: [{ assignmentId: "9", id: "7", isAbsent: true }],
    });
  });

  it("rejects a missing schedule overview envelope clearly", async () => {
    sessionFetch.mockResolvedValue({ ok: true, json: async () => ({}) });

    await expect(
      substitutionService.fetchScheduleOverview("2026-08-31", "2026-09-04"),
    ).rejects.toThrow("Ungültige Antwort für Vertretungen.");
  });

  it("assigns an appointment-scoped schedule substitution", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          instance_id: 42,
          cancelled: false,
          understaffed_ack: false,
          affected_instances: [],
          warnings: [],
        },
      }),
    });

    await substitutionService.applyScheduleSubstitution("42", {
      substitutions: [
        {
          absentStaffId: "7",
          substituteStaffId: "8",
          instanceIds: ["42", "43"],
        },
      ],
    });

    expect(JSON.parse(sessionFetch.mock.calls[0]?.[1]?.body as string)).toEqual(
      {
        type: "schedule_substitution",
        schedule_substitution: {
          instance_id: 42,
          substitutions: [
            {
              absent_staff_id: 7,
              substitute_staff_id: 8,
              instance_ids: [42, 43],
            },
          ],
        },
      },
    );
  });

  it("assigns day-wide substitutions through the same module", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: { days: [], total_affected: 0 },
      }),
    });

    await substitutionService.applyBulkSubstitution({
      absentStaffId: "7",
      substituteStaffId: "8",
      dates: ["2026-08-31", "2026-09-01"],
    });

    expect(JSON.parse(sessionFetch.mock.calls[0]?.[1]?.body as string)).toEqual(
      {
        type: "schedule_substitution",
        schedule_substitution: {
          whole_days: {
            absent_staff_id: 7,
            substitute_staff_id: 8,
            dates: ["2026-08-31", "2026-09-01"],
          },
        },
      },
    );
  });

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

  it("loads the central overview through one module request", async () => {
    sessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          groups: [{ id: "12", name: "Robins Gruppe" }],
          group_handovers: [handover],
          targets: [{ id: "34", full_name: "Toni Test" }],
          running_supervisions: [
            {
              id: "41",
              type: "additional_supervision",
              name: "Freispiel",
              room_name: "Atelier",
              supervisors: [{ id: "11", full_name: "Alex Alt" }],
              available_targets: [{ id: "73", full_name: "Nora Neu" }],
              is_current_user_supervising: true,
              can_assign: true,
            },
          ],
        },
      }),
    });

    const result = await substitutionService.fetchOverview();

    expect(sessionFetch).toHaveBeenCalledWith("/api/substitutions", {
      credentials: "include",
    });
    expect(result.groups).toEqual([{ id: "12", name: "Robins Gruppe" }]);
    expect(result.groupHandovers[0]?.id).toBe("5");
    expect(result.targets).toEqual([{ id: "34", fullName: "Toni Test" }]);
    expect(result.runningSupervisions[0]).toMatchObject({
      id: "41",
      roomName: "Atelier",
      canAssign: true,
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
          group_id: "12",
          target_staff_id: "34",
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
    const json = vi.fn(async () => ({
      data: {
        id: "91",
        type: "additional_supervision",
        active_group_id: "41",
        target: { id: "73", full_name: "Toni Test" },
      },
    }));
    sessionFetch.mockResolvedValue({
      ok: true,
      json,
    });

    const result = await substitutionService.addSupervisor(
      "9007199254740993",
      "9007199254740995",
    );

    expect(sessionFetch).toHaveBeenCalledWith("/api/substitutions", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        type: "additional_supervision",
        additional_supervision: {
          active_group_id: "9007199254740993",
          target_staff_id: "9007199254740995",
        },
      }),
    });
    expect(result).toEqual({ id: "91", targetName: "Toni Test" });
    expect(json).toHaveBeenCalledTimes(1);
  });

  it("rejects a missing proxy envelope", async () => {
    sessionFetch.mockResolvedValue({ ok: true, json: async () => ({}) });

    await expect(substitutionService.fetchSubstitutions()).rejects.toThrow(
      "Ungültige Antwort",
    );
  });

  it("ends a typed group handover", async () => {
    sessionFetch.mockResolvedValue({ ok: true });

    await substitutionService.deleteSubstitution("9007199254740993");

    expect(sessionFetch).toHaveBeenCalledWith("/api/substitutions/end", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        type: "group_handover",
        id: "9007199254740993",
      }),
    });
  });
});
