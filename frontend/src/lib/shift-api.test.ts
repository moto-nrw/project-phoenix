import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
}));

import { ownShiftService, staffShiftService } from "./shift-api";

const backendAssignment = {
  instance_id: 81,
  title: "Frühbetreuung",
  group_name: "Lernzeit",
  room_name: "Sonnenraum",
  date: "2026-07-06",
  start_time: "08:00:00",
  end_time: "09:00:00",
  status: "planned",
  cancelled: false,
  is_primary: true,
  is_substitute: false,
  is_absent: false,
  absence_reason: null,
  cancel_reason: null,
  understaffed_ack: false,
};

const backendShift = {
  id: 42,
  staff_id: 7,
  date: "2026-07-06",
  start_time: "08:00",
  end_time: "16:00",
  break_minutes: 30,
};

const backendOverview = {
  from: "2026-07-06",
  to: "2026-07-10",
  dienstplan_in_use: true,
  staff: [{ id: 7, first_name: "Ada", last_name: "Lovelace" }],
  shifts: [backendShift],
  assignments: [
    {
      instance_id: 81,
      staff_id: 7,
      date: "2026-07-06",
      start_time: "08:00",
      end_time: "09:00",
      activity_title: "Frühbetreuung",
      room_id: 12,
      room_name: "Sonnenraum",
      status: "planned",
      is_absent: false,
      is_substitute: false,
      absence_reason: null,
      coverage_status: "covered",
      coverage_reason: null,
      uncovered_intervals: [],
    },
  ],
};

describe("staffShiftService errors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("preserves status and backend error detail for save failures", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "shift overlaps existing row" }), {
        status: 409,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(
      staffShiftService.createShift({
        staffId: "7",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "16:00",
        breakMinutes: 30,
        shiftTypeId: null,
      }),
    ).rejects.toMatchObject({
      status: 409,
      detail: "shift overlaps existing row",
    });
  });

  it("preserves status for load failures with text bodies", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("database timeout", { status: 500 }),
    );

    await expect(
      staffShiftService.getShifts("2026-07-06", "2026-07-10"),
    ).rejects.toMatchObject({
      status: 500,
      detail: "database timeout",
    });
  });

  it("preserves backend errors when the weekly overview fails", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "overview unavailable" }), {
        status: 503,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(
      staffShiftService.getOverview("2026-07-06", "2026-07-10"),
    ).rejects.toMatchObject({
      status: 503,
      detail: "overview unavailable",
    });
  });

  it("falls back when JSON error bodies are malformed", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("{", {
        status: 400,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(staffShiftService.deleteShift("42")).rejects.toMatchObject({
      status: 400,
      detail: "Schicht konnte nicht gelöscht werden",
    });
  });

  it("falls back when JSON error bodies omit detail", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ message: "wrong shape" }), {
        status: 422,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(
      staffShiftService.updateShift("42", {
        staffId: "7",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "16:00",
        breakMinutes: 30,
        shiftTypeId: null,
      }),
    ).rejects.toMatchObject({
      status: 422,
      detail: "Schicht konnte nicht gespeichert werden",
    });
  });
});

describe("staffShiftService requests", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps nullable list responses to an empty list", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: null }, { status: 200 }),
    );

    await expect(
      staffShiftService.getShifts("2026-07-06", "2026-07-10"),
    ).resolves.toEqual([]);
  });

  it("loads own shifts through the time-tracking endpoint", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: [backendShift] }, { status: 200 }),
    );

    await expect(
      ownShiftService.getOwnShifts("2026-07-06", "2026-07-10"),
    ).resolves.toEqual([
      {
        id: "42",
        staffId: "7",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "16:00",
        breakMinutes: 30,
        shiftTypeId: null,
        shiftTypeName: null,
        shiftTypeColor: null,
        notes: "",
        seriesId: null,
        detached: false,
        cancelled: false,
        changeReason: null,
        originShiftId: null,
      },
    ]);
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/time-tracking/shifts?from=2026-07-06&to=2026-07-10",
    );
  });

  it("loads and maps the weekly shift-assignment overview", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: backendOverview }, { status: 200 }),
    );

    await expect(
      staffShiftService.getOverview("2026-07-06", "2026-07-10"),
    ).resolves.toMatchObject({
      from: "2026-07-06",
      to: "2026-07-10",
      dienstplanInUse: true,
      staff: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
      shifts: [{ id: "42", staffId: "7" }],
      assignments: [
        {
          instanceId: "81",
          staffId: "7",
          roomId: "12",
          coverageStatus: "covered",
        },
      ],
    });
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/staff/shifts/overview?from=2026-07-06&to=2026-07-10",
    );
  });

  it("serializes create and update payloads for the backend", async () => {
    const payload = {
      staffId: "7",
      date: "2026-07-06",
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
      shiftTypeId: null,
    };
    mockSessionFetch
      .mockResolvedValueOnce(Response.json({ data: backendShift }))
      .mockResolvedValueOnce(Response.json({ data: backendShift }));

    await staffShiftService.createShift(payload);
    await staffShiftService.updateShift("42", payload);

    expect(mockSessionFetch).toHaveBeenNthCalledWith(1, "/api/staff/shifts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        staff_id: 7,
        date: "2026-07-06",
        start_time: "08:00",
        end_time: "16:00",
        break_minutes: 30,
        shift_type_id: null,
      }),
    });
    expect(mockSessionFetch).toHaveBeenNthCalledWith(
      2,
      "/api/staff/shifts/42",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          staff_id: 7,
          date: "2026-07-06",
          start_time: "08:00",
          end_time: "16:00",
          break_minutes: 30,
          shift_type_id: null,
        }),
      },
    );
  });

  it("serializes a linked shift type id to a number", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: { ...backendShift, shift_type_id: 5 } }),
    );

    await staffShiftService.createShift({
      staffId: "7",
      date: "2026-07-06",
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
      shiftTypeId: "5",
    });

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shifts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        staff_id: 7,
        date: "2026-07-06",
        start_time: "08:00",
        end_time: "16:00",
        break_minutes: 30,
        shift_type_id: 5,
      }),
    });
  });

  it("serializes replacement origin IDs as strings without bigint rounding", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({
        data: { ...backendShift, origin_shift_id: "9223372036854775807" },
      }),
    );

    await staffShiftService.createShift({
      staffId: "7",
      date: "2026-07-06",
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
      shiftTypeId: null,
      originShiftId: "9223372036854775807",
    });

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shifts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        staff_id: 7,
        date: "2026-07-06",
        start_time: "08:00",
        end_time: "16:00",
        break_minutes: 30,
        shift_type_id: null,
        origin_shift_id: "9223372036854775807",
      }),
    });
  });

  it("serializes an atomic move with explicit source and target owners", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: { ...backendShift, staff_id: 8 } }),
    );

    await staffShiftService.moveShift("42", {
      sourceStaffId: "7",
      targetStaffId: "8",
      date: "2026-07-07",
      startTime: "09:00",
      endTime: "15:00",
      breakMinutes: 20,
      shiftTypeId: "5",
    });

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shifts/42/move", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        source_staff_id: 7,
        target_staff_id: 8,
        date: "2026-07-07",
        start_time: "09:00",
        end_time: "15:00",
        break_minutes: 20,
        shift_type_id: 5,
      }),
    });
  });

  it("serializes notes and a nullable change reason when supplied", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: backendShift }, { status: 201 }),
    );

    await staffShiftService.createShift({
      staffId: "7",
      date: "2026-07-06",
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
      shiftTypeId: null,
      notes: "Nur Hintereingang",
      changeReason: null,
    });

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shifts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        staff_id: 7,
        date: "2026-07-06",
        start_time: "08:00",
        end_time: "16:00",
        break_minutes: 30,
        shift_type_id: null,
        notes: "Nur Hintereingang",
        change_reason: null,
      }),
    });
  });

  it("returns cleanly after successful deletes", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(staffShiftService.deleteShift("42")).resolves.toBeUndefined();
    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shifts/42", {
      method: "DELETE",
    });
  });

  it("loads one staff member's shifts through the staff_id filter (#1844)", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: [backendShift] }, { status: 200 }),
    );

    await expect(
      staffShiftService.getShiftsForStaff("7", "2026-07-06", "2026-07-10"),
    ).resolves.toEqual([
      {
        id: "42",
        staffId: "7",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "16:00",
        breakMinutes: 30,
        shiftTypeId: null,
        shiftTypeName: null,
        shiftTypeColor: null,
        notes: "",
        seriesId: null,
        detached: false,
        cancelled: false,
        changeReason: null,
        originShiftId: null,
      },
    ]);
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/staff/shifts?staff_id=7&from=2026-07-06&to=2026-07-10",
    );
  });
});

describe("ownShiftService assignments (#1844)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("loads and maps the own Betreuungsplan assignments", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: [backendAssignment] }, { status: 200 }),
    );

    await expect(
      ownShiftService.getOwnAssignments("2026-07-06", "2026-07-10"),
    ).resolves.toEqual([
      {
        instanceId: "81",
        title: "Frühbetreuung",
        groupName: "Lernzeit",
        roomName: "Sonnenraum",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "09:00",
        status: "planned",
        cancelled: false,
        isPrimary: true,
        isSubstitute: false,
        isAbsent: false,
        absenceReason: null,
        cancelReason: null,
        understaffedAck: false,
      },
    ]);
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/time-tracking/assignments?from=2026-07-06&to=2026-07-10",
    );
  });

  it("maps a nullable assignment list to an empty list", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: null }, { status: 200 }),
    );

    await expect(
      ownShiftService.getOwnAssignments("2026-07-06", "2026-07-10"),
    ).resolves.toEqual([]);
  });

  it("preserves the backend error detail when the assignments load fails", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "assignments unavailable" }), {
        status: 500,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(
      ownShiftService.getOwnAssignments("2026-07-06", "2026-07-10"),
    ).rejects.toMatchObject({
      status: 500,
      detail: "assignments unavailable",
    });
  });

  it("falls back to a default message when the assignments error body is empty", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response("", { status: 502 }));

    await expect(
      ownShiftService.getOwnAssignments("2026-07-06", "2026-07-10"),
    ).rejects.toMatchObject({
      status: 502,
      detail: "Einsätze konnten nicht geladen werden",
    });
  });
});
