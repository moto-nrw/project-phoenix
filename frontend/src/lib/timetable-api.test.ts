import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { mockGetSession } = vi.hoisted(() => ({
  mockGetSession: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  getSession: mockGetSession,
}));

vi.mock("./logger", () => ({
  createLogger: () => ({
    info: vi.fn(),
    error: vi.fn(),
    warn: vi.fn(),
  }),
}));

import { timetableService } from "./timetable-api";
import type {
  BackendEnrichedInstance,
  BackendTimetableTemplate,
} from "./timetable-types";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

const backendInstance: BackendEnrichedInstance = {
  id: 42,
  date: "2026-05-04",
  start_time: "12:00",
  end_time: "13:00",
  title: "Mensa",
  status: "planned",
  is_spontaneous: false,
  is_live: false,
  activity_group_id: 7,
  activity_type: "care",
  room_id: 3,
  room_name: "Mensa",
  staff: [
    {
      staff_id: 11,
      is_primary: true,
      is_absent: false,
      is_substitute: false,
    },
  ],
  students: [
    {
      student_id: 21,
      status: "expected",
      note: "kommt spaeter",
    },
  ],
  staff_count: 1,
  absent_staff_count: 0,
  expected_students_count: 1,
  present_students_count: 0,
  required_staff_count: 1,
  assigned_staff_count: 1,
  conflict_warnings: [
    {
      kind: "room",
      resource_id: 3,
      message: "Raum doppelt belegt",
      can_override: true,
    },
  ],
};

const backendTemplate: BackendTimetableTemplate = {
  id: 7,
  name: "Yoga",
  type: "activity",
  category_id: 2,
  category_name: "AG",
  room_id: 3,
  room_name: "Turnhalle",
  is_open: true,
  max_participants: 12,
  target_group_type: "none",
  enrollment_count: 8,
  supervisor_count: 1,
  required_staff_count: 1,
  assigned_staff_count: 1,
  student_ids: [21],
  staff_ids: [11],
  primary_staff_id: 11,
  schedules: [
    {
      id: 9,
      weekday: 1,
      start_time: "14:00",
      end_time: "15:00",
      week_pattern: 0,
      calendar_period_id: 5,
    },
  ],
};

describe("timetableService", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    mockGetSession.mockResolvedValue({
      user: { token: "jwt" },
      expires: "2099-01-01",
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("requires a session before loading the week", async () => {
    mockGetSession.mockResolvedValueOnce(null);

    await expect(
      timetableService.getWeek("2026-05-04", "2026-05-08"),
    ).rejects.toMatchObject({
      httpStatus: 401,
      code: "UNAUTHENTICATED",
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("loads and maps weekly instances", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        status: "success",
        data: {
          from: "2026-05-04",
          to: "2026-05-08",
          instances: [backendInstance],
        },
      }),
    );

    const result = await timetableService.getWeek("2026-05-04", "2026-05-08");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/instances?from=2026-05-04&to=2026-05-08",
      expect.objectContaining({
        method: "GET",
        credentials: "include",
      }),
    );
    expect(result.instances[0]).toMatchObject({
      id: "42",
      activityGroupId: "7",
      roomId: "3",
      studentIds: ["21"],
      conflictWarnings: [
        expect.objectContaining({ resourceId: "3", canOverride: true }),
      ],
    });
  });

  it("creates, updates and deletes instances with the expected wire shape", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: backendInstance }))
      .mockResolvedValueOnce(jsonResponse({ data: backendInstance }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    const body = {
      date: "2026-05-04",
      title: "Mensa",
      start_time: "12:00",
      end_time: "13:00",
      room_id: 3,
      staff_ids: [11],
      student_ids: [21],
    };

    await expect(timetableService.create(body)).resolves.toMatchObject({
      id: "42",
      title: "Mensa",
    });
    await expect(timetableService.update("42", body)).resolves.toMatchObject({
      id: "42",
    });
    await expect(
      timetableService.deleteCancelled("42"),
    ).resolves.toBeUndefined();

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/instances",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(body),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/instances/42",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(body),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/timetable/instances/42",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("handles template CRUD and period filtering", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            template_id: 7,
            timeframe_id: 4,
            schedule_ids: [9, 10],
            instances_created: 2,
            materialized_from: "2026-05-04",
            materialized_to: "2026-05-08",
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ data: { templates: [backendTemplate] } }),
      )
      .mockResolvedValueOnce(jsonResponse({ data: backendTemplate }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    const createBody = {
      name: "Yoga",
      type: "activity" as const,
      weekdays: [1],
      start_time: "14:00",
      end_time: "15:00",
      room_id: 3,
      category_id: 2,
      calendar_period_id: 5,
    };

    await expect(timetableService.createTemplate(createBody)).resolves.toEqual({
      templateId: "7",
      timeframeId: "4",
      scheduleIds: ["9", "10"],
      instancesCreated: 2,
      materializedFrom: "2026-05-04",
      materializedTo: "2026-05-08",
    });
    await expect(timetableService.getTemplates("5")).resolves.toMatchObject({
      templates: [{ id: "7", schedules: [{ calendarPeriodId: "5" }] }],
    });
    await expect(
      timetableService.updateTemplate("7", createBody),
    ).resolves.toMatchObject({ id: "7", primaryStaffId: "11" });
    await expect(
      timetableService.archiveTemplate("7"),
    ).resolves.toBeUndefined();

    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/templates?period_id=5",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("loads a single template, splits a template, and ends it from an effective date", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: backendTemplate }))
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            old_template_id: 7,
            new_template_id: 8,
            schedule_ids: [11, 12],
            deleted_instances: 4,
            instances_created: 6,
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            template_id: 7,
            effective_date: "2026-06-01",
            deleted_instances: 4,
          },
        }),
      );

    await expect(timetableService.getTemplate("7")).resolves.toMatchObject({
      id: "7",
      name: "Yoga",
      primaryStaffId: "11",
    });

    const splitBody = {
      name: "Yoga",
      type: "activity" as const,
      weekdays: [1],
      start_time: "15:00",
      end_time: "16:00",
      room_id: 3,
      category_id: 2,
      effective_date: "2026-06-01",
      materialize_from: "2026-06-01",
      materialize_to: "2026-06-07",
    };

    await expect(
      timetableService.splitTemplate("7", splitBody),
    ).resolves.toEqual({
      oldTemplateId: "7",
      newTemplateId: "8",
      scheduleIds: ["11", "12"],
      deletedInstances: 4,
      instancesCreated: 6,
    });
    await expect(
      timetableService.endTemplate("7", {
        effective_date: "2026-06-01",
      }),
    ).resolves.toEqual({
      templateId: "7",
      effectiveDate: "2026-06-01",
      deletedInstances: 4,
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/templates/7",
      expect.objectContaining({ method: "GET", credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/templates/7/split",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(splitBody),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/timetable/templates/7/end",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ effective_date: "2026-06-01" }),
      }),
    );
  });

  it("checks conflicts with only the provided params in the query string", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            date: "2026-05-04",
            start_time: "12:00",
            end_time: "13:00",
            warnings: [
              {
                kind: "room",
                resource_id: 3,
                message: "Raum doppelt belegt",
                conflicting_instance_id: 42,
                conflicting_title: "Mensa",
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            date: "2026-05-04",
            start_time: "12:00",
            end_time: "13:00",
            warnings: [],
          },
        }),
      );

    await expect(
      timetableService.checkConflicts({
        date: "2026-05-04",
        startTime: "12:00",
        endTime: "13:00",
        roomId: "3",
        staffIds: ["11", "12"],
        studentIds: ["21"],
        excludeInstanceId: "42",
      }),
    ).resolves.toEqual({
      date: "2026-05-04",
      startTime: "12:00",
      endTime: "13:00",
      warnings: [
        {
          kind: "room",
          resourceId: "3",
          message: "Raum doppelt belegt",
          conflictingInstanceId: "42",
          conflictingTitle: "Mensa",
        },
      ],
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/conflict-check?date=2026-05-04&start_time=12%3A00&end_time=13%3A00&room_id=3&staff_ids=11%2C12&student_ids=21&exclude_instance_id=42",
      expect.objectContaining({ method: "GET", credentials: "include" }),
    );

    // Optional params (and empty arrays) are omitted entirely.
    await expect(
      timetableService.checkConflicts({
        date: "2026-05-04",
        startTime: "12:00",
        endTime: "13:00",
        staffIds: [],
      }),
    ).resolves.toMatchObject({ warnings: [] });
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/conflict-check?date=2026-05-04&start_time=12%3A00&end_time=13%3A00",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("calls lifecycle, materialize, re-plan and quality endpoints", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            instance_id: 42,
            status: "active",
            active_group_id: 99,
            started_at: "2026-05-04T12:00:00Z",
            warnings: [],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            instance_id: 42,
            status: "completed",
            completed_at: "2026-05-04T13:00:00Z",
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: { from: "2026-05-04", to: "2026-05-08", instances_created: 3 },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            from: "2026-05-04",
            to: "2026-05-08",
            deleted_instances: 1,
            candidates_skipped_existing: 0,
            instances_created: 3,
            instance_students_created: 6,
            instance_staff_created: 3,
            warnings: [],
            duration_ms: 12,
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            from: "2026-05-04",
            to: "2026-05-08",
            gaps: [
              {
                instance_id: 42,
                date: "2026-05-04",
                title: "Mensa",
                start_time: "12:00",
                end_time: "13:00",
                room_id: 3,
                status: "planned",
                assigned_staff_count: 1,
                absent_staff_count: 1,
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            from: "2026-05-04",
            to: "2026-05-08",
            conflicts: [
              {
                kind: "modified_instance_time_mismatch",
                date: "2026-05-04",
                activity_group_id: 7,
                instance_id: 42,
                activity_title: "Mensa",
                student_id: 21,
                arrival_source: "schedule",
                original_start_time: "12:00",
                modified_start_time: "12:30",
              },
            ],
          },
        }),
      );

    await expect(timetableService.start("42")).resolves.toMatchObject({
      instanceId: "42",
      activeGroupId: "99",
    });
    await expect(timetableService.complete("42")).resolves.toMatchObject({
      instanceId: "42",
      status: "completed",
    });
    await expect(
      timetableService.materialize("2026-05-04", "2026-05-08"),
    ).resolves.toMatchObject({ instancesCreated: 3 });
    await expect(
      timetableService.replanWeek("2026-05-04", "2026-05-08"),
    ).resolves.toMatchObject({ deletedInstances: 1 });
    await expect(
      timetableService.getGaps("2026-05-04", "2026-05-08"),
    ).resolves.toMatchObject({ gaps: [{ instanceId: "42" }] });
    await expect(
      timetableService.getExceptionConflicts("2026-05-04", "2026-05-08"),
    ).resolves.toMatchObject({
      conflicts: [{ instanceId: "42", studentId: "21" }],
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/instances/42/start",
      expect.objectContaining({ method: "POST", body: "{}" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      4,
      "/api/timetable/instances/re-plan-week",
      expect.objectContaining({
        body: JSON.stringify({
          from_date: "2026-05-04",
          to_date: "2026-05-08",
        }),
      }),
    );
  });

  it("re-plans a week scoped to a single template", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          from: "2026-05-04",
          to: "2026-05-08",
          deleted_instances: 2,
          candidates_skipped_existing: 0,
          instances_created: 3,
          instance_students_created: 6,
          instance_staff_created: 3,
          duration_ms: 5,
        },
      }),
    );

    await expect(
      timetableService.replanWeek("2026-05-04", "2026-05-08", "7"),
    ).resolves.toMatchObject({ deletedInstances: 2, instancesCreated: 3 });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/instances/re-plan-week",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          from_date: "2026-05-04",
          to_date: "2026-05-08",
          activity_group_id: 7,
        }),
      }),
    );
  });

  it("substitutes staff and patches attendance", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            absent_staff_id: 11,
            substitute_staff_id: 12,
            date: "2026-05-04",
            affected_instances: [
              {
                instance_id: 42,
                title: "Mensa",
                start_time: "12:00",
                action: "updated",
              },
            ],
            warnings: [],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            id: 100,
            instance_id: 42,
            student_id: 21,
            status: "absent",
            substatus: "sick",
            note: "krank",
          },
        }),
      );

    await expect(
      timetableService.substitute("11", "12", "2026-05-04"),
    ).resolves.toMatchObject({
      absentStaffId: "11",
      substituteStaffId: "12",
      affectedInstances: [{ instanceId: "42" }],
    });
    await expect(
      timetableService.patchAttendance("42", "21", {
        status: "absent",
        substatus: "sick",
        note: "krank",
      }),
    ).resolves.toMatchObject({
      id: "100",
      instanceId: "42",
      studentId: "21",
    });
  });

  it("uses backend error payloads and generic messages for failed responses", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        { error: "Zeitraum ungueltig", code: "INVALID_RANGE" },
        { status: 400 },
      ),
    );

    await expect(timetableService.materialize("x", "y")).rejects.toMatchObject({
      message: "Zeitraum ungueltig",
      httpStatus: 400,
      code: "INVALID_RANGE",
    });

    fetchMock.mockResolvedValueOnce(
      new Response("nope", {
        status: 502,
        headers: { "Content-Type": "text/plain" },
      }),
    );
    await expect(timetableService.cancel("42")).rejects.toMatchObject({
      message: "Anfrage fehlgeschlagen (HTTP 502)",
      httpStatus: 502,
    });
  });

  it("passes an optional cancel reason through the lifecycle body (#1840)", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({ data: { instance_id: 42, status: "cancelled" } }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ data: { instance_id: 42, status: "cancelled" } }),
      );

    await timetableService.cancel("42", "Ausflug");
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/instances/42/cancel",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ reason: "Ausflug" }),
      }),
    );

    await timetableService.cancel("42");
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/instances/42/cancel",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({}),
      }),
    );
  });

  it("sends an optional reason with a substitute (#1840)", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          absent_staff_id: 11,
          substitute_staff_id: 12,
          date: "2026-05-04",
          affected_instances: [],
          warnings: [],
        },
      }),
    );

    await timetableService.substitute("11", "12", "2026-05-04", "krank");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/substitute",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          absent_staff_id: 11,
          substitute_staff_id: 12,
          date: "2026-05-04",
          reason: "krank",
        }),
      }),
    );
  });

  it("marks a staff member absent for the day without a substitute (#1840)", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          absent_staff_id: 11,
          substitute_staff_id: 0,
          date: "2026-05-04",
          affected_instances: [
            {
              instance_id: 42,
              title: "Mensa",
              start_time: "12:00",
              action: "marked_absent",
            },
          ],
          warnings: [],
        },
      }),
    );

    await expect(
      timetableService.markAbsent("11", "2026-05-04", "krank"),
    ).resolves.toMatchObject({
      absentStaffId: "11",
      affectedInstances: [{ instanceId: "42", action: "marked_absent" }],
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/substitute",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          absent_staff_id: 11,
          date: "2026-05-04",
          reason: "krank",
        }),
      }),
    );
  });

  it("flips the understaffed acknowledgement (#1840)", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            instance_id: 42,
            status: "planned",
            understaffed_ack: true,
            understaffed_note: "keine Vertretung",
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            instance_id: 42,
            status: "planned",
            understaffed_ack: false,
            understaffed_note: null,
          },
        }),
      );

    await expect(
      timetableService.acknowledgeUnderstaffed("42", true, "keine Vertretung"),
    ).resolves.toMatchObject({
      instanceId: "42",
      understaffedAck: true,
      understaffedNote: "keine Vertretung",
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/instances/42/acknowledge-understaffed",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ ack: true, note: "keine Vertretung" }),
      }),
    );

    await timetableService.acknowledgeUnderstaffed("42", false);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/instances/42/acknowledge-understaffed",
      expect.objectContaining({
        body: JSON.stringify({ ack: false }),
      }),
    );
  });

  it("applies a cancel-only deviation, ignoring the other edits (#1840)", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          instance_id: 42,
          cancelled: true,
          understaffed_ack: false,
          affected_instances: [],
          warnings: [],
        },
      }),
    );

    await expect(
      timetableService.applyDeviations("42", {
        cancel: true,
        cancelReason: "Ausflug",
        understaffedAck: true,
        absences: [{ staffId: "11" }],
      }),
    ).resolves.toMatchObject({ instanceId: "42", cancelled: true });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/instances/42/deviations",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ cancel: true, cancel_reason: "Ausflug" }),
      }),
    );
  });

  it("serializes a full deviation save into one wire body (#1840)", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          instance_id: 42,
          cancelled: false,
          understaffed_ack: true,
          affected_instances: [
            {
              instance_id: 42,
              title: "Mensa",
              start_time: "12:00",
              action: "substituted",
            },
          ],
          warnings: [],
        },
      }),
    );

    await expect(
      timetableService.applyDeviations("42", {
        understaffedAck: true,
        understaffedNote: "Rest offen",
        absences: [{ staffId: "11", reason: "krank" }, { staffId: "13" }],
        substitutions: [
          {
            absentStaffId: "11",
            substituteStaffId: "12",
            reason: "springt ein",
          },
        ],
        presences: ["14", "15"],
      }),
    ).resolves.toMatchObject({
      instanceId: "42",
      understaffedAck: true,
      affectedInstances: [{ instanceId: "42", action: "substituted" }],
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/instances/42/deviations",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          understaffed_ack: true,
          understaffed_note: "Rest offen",
          absences: [{ staff_id: 11, reason: "krank" }, { staff_id: 13 }],
          substitutions: [
            {
              absent_staff_id: 11,
              substitute_staff_id: 12,
              reason: "springt ein",
            },
          ],
          presences: [14, 15],
        }),
      }),
    );
  });

  it("omits the understaffed note when clearing the acknowledgement (#1840)", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          instance_id: 42,
          cancelled: false,
          understaffed_ack: false,
          affected_instances: [],
          warnings: [],
        },
      }),
    );

    await timetableService.applyDeviations("42", {
      understaffedAck: false,
      understaffedNote: "sollte nicht gesendet werden",
      absences: [],
      substitutions: [],
      presences: [],
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/instances/42/deviations",
      expect.objectContaining({
        body: JSON.stringify({ understaffed_ack: false }),
      }),
    );
  });

  it("marks absent without a reason and omits it from the body (#1840)", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          absent_staff_id: 11,
          substitute_staff_id: 0,
          date: "2026-05-04",
          affected_instances: [],
          warnings: [],
        },
      }),
    );

    await timetableService.markAbsent("11", "2026-05-04");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/substitute",
      expect.objectContaining({
        body: JSON.stringify({ absent_staff_id: 11, date: "2026-05-04" }),
      }),
    );
  });

  it("cancels via deviations without a reason and with substitutions lacking one (#1840)", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            instance_id: 42,
            cancelled: true,
            understaffed_ack: false,
            affected_instances: [],
            warnings: [],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            instance_id: 42,
            cancelled: false,
            understaffed_ack: false,
            affected_instances: [],
            warnings: [],
          },
        }),
      );

    // cancel without a reason → cancel_reason is omitted.
    await timetableService.applyDeviations("42", { cancel: true });
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/instances/42/deviations",
      expect.objectContaining({ body: JSON.stringify({ cancel: true }) }),
    );

    // No understaffedAck field, absence and substitution without a reason.
    await timetableService.applyDeviations("42", {
      absences: [{ staffId: "11" }],
      substitutions: [{ absentStaffId: "11", substituteStaffId: "12" }],
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/instances/42/deviations",
      expect.objectContaining({
        body: JSON.stringify({
          absences: [{ staff_id: 11 }],
          substitutions: [{ absent_staff_id: 11, substitute_staff_id: 12 }],
        }),
      }),
    );
  });
});
