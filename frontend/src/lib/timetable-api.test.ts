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
      kind: "staff",
      resource_id: 3,
      message: "Personal doppelt eingeplant",
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

  it("converts an existing occurrence into a series with one atomic request", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          template_id: 7,
          timeframe_id: 4,
          schedule_ids: [9, 10],
          linked_instance_id: 42,
        },
      }),
    );
    const body = {
      name: "Yoga",
      type: "activity" as const,
      weekdays: [1],
      start_time: "14:00",
      end_time: "15:00",
      room_id: 3,
      category_id: 2,
      calendar_period_id: 5,
      start_date: "2026-05-04",
      instance_notes: "Heute draußen",
    };

    await expect(
      timetableService.convertInstanceToSeries("42", body),
    ).resolves.toEqual({
      templateId: "7",
      timeframeId: "4",
      scheduleIds: ["9", "10"],
      linkedInstanceId: "42",
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/instances/42/convert-to-series",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(body),
      }),
    );
  });

  it("passes series_roster_from through to the update body (#2187)", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: backendTemplate }));

    const body = {
      name: "Yoga",
      type: "activity" as const,
      weekdays: [1],
      start_time: "14:00",
      end_time: "15:00",
      room_id: 3,
      category_id: 2,
      calendar_period_id: 5,
      series_roster_from: "2026-06-01",
    };

    await timetableService.updateTemplate("7", body);

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/templates/7",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(body),
      }),
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

    await expect(timetableService.getTemplate("7", "5")).resolves.toMatchObject(
      {
        id: "7",
        name: "Yoga",
        primaryStaffId: "11",
      },
    );

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
      "/api/timetable/templates/7?period_id=5",
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

  it("loads the offering sources with and without a period filter (#2137)", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            offerings: [
              {
                id: 12,
                name: "Frühbetreuung",
                phase_id: 3,
                phase_name: "Schuljahr 2026/27",
                total_count: 18,
                grade_counts: { "0": 2, "1": 9, "2": 7 },
                sourced_templates: [
                  { id: 7, name: "Frühbetreuung Jg. 1", grade_levels: [1] },
                ],
                legacy_linked_template_id: 7,
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ data: { offerings: [] } }));

    await expect(timetableService.getOfferingSources("5")).resolves.toEqual([
      {
        id: "12",
        name: "Frühbetreuung",
        phaseId: "3",
        phaseName: "Schuljahr 2026/27",
        totalCount: 18,
        gradeCounts: { 0: 2, 1: 9, 2: 7 },
        sourcedTemplates: [
          { id: "7", name: "Frühbetreuung Jg. 1", gradeLevels: [1] },
        ],
        legacyLinkedTemplateId: "7",
      },
    ]);
    await expect(timetableService.getOfferingSources()).resolves.toEqual([]);

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/offering-sources?calendar_period_id=5",
      expect.objectContaining({ method: "GET", credentials: "include" }),
    );
    // No period → no query string at all, not a dangling "?".
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/offering-sources",
      expect.objectContaining({ method: "GET", credentials: "include" }),
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
                kind: "staff",
                resource_id: 3,
                message: "Personal doppelt eingeplant",
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
          kind: "staff",
          resourceId: "3",
          message: "Personal doppelt eingeplant",
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

  it("posts a batched shift-coverage probe with recurrence metadata", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          coverage_warning_count: 17,
          coverage_warnings: [
            {
              staff_id: 11,
              staff_name: "Ada Staff",
              date: "2026-05-06",
              start_time: "12:00",
              end_time: "13:00",
              uncovered_start_time: "12:30",
              uncovered_end_time: "13:00",
              message: "Mittwoch ist nicht vollständig abgedeckt.",
            },
          ],
        },
      }),
    );

    const controller = new AbortController();
    await expect(
      timetableService.checkShiftCoverage(
        {
          dates: ["2026-05-04", "2026-05-06"],
          startTime: "12:00",
          endTime: "13:00",
          staffIds: ["11", "12"],
          replanActivityGroupId: "7",
          calendarPeriodId: "5",
          weekPattern: 2,
        },
        { signal: controller.signal },
      ),
    ).resolves.toEqual({
      coverageWarningCount: 17,
      coverageWarnings: [
        {
          staffId: "11",
          staffName: "Ada Staff",
          date: "2026-05-06",
          startTime: "12:00",
          endTime: "13:00",
          uncoveredStartTime: "12:30",
          uncoveredEndTime: "13:00",
          message: "Mittwoch ist nicht vollständig abgedeckt.",
        },
      ],
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/shift-coverage",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        signal: controller.signal,
        body: JSON.stringify({
          dates: ["2026-05-04", "2026-05-06"],
          start_time: "12:00",
          end_time: "13:00",
          staff_ids: [11, 12],
          replan_activity_group_id: 7,
          calendar_period_id: 5,
          week_pattern: 2,
        }),
      }),
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
      );

    await expect(timetableService.start("42")).resolves.toMatchObject({
      instanceId: "42",
      activeGroupId: "99",
    });
    await expect(timetableService.complete("42", [])).resolves.toMatchObject({
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

  it("loads filtered deviation history, applies deviations, and patches attendance", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            events: [
              {
                id: 91,
                occurrence_date: "2026-05-04",
                start_time: "12:00:00",
                instance_id: 42,
                event_type: "absence",
                occurred_at: "2026-05-04T10:00:00Z",
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            instance_id: 42,
            cancelled: false,
            understaffed_ack: true,
            affected_instances: [],
            warnings: [],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            id: 7,
            instance_id: 42,
            student_id: 21,
            status: "present",
          },
        }),
      );

    await expect(
      timetableService.getDeviationHistory(
        "2026-05-04",
        "2026-05-08",
        "7",
        "12:00",
      ),
    ).resolves.toMatchObject({ events: [{ id: "91", instanceId: "42" }] });
    await expect(
      timetableService.applyDeviations("42", {
        understaffedAck: true,
        understaffedNote: "keine Vertretung",
        absences: [{ staffId: "11", reason: "krank" }],
        substitutions: [{ absentStaffId: "12", substituteStaffId: "13" }],
        presences: ["14"],
      }),
    ).resolves.toMatchObject({ instanceId: "42", understaffedAck: true });
    await expect(
      timetableService.patchAttendance("42", "21", { status: "present" }),
    ).resolves.toMatchObject({ id: "7", studentId: "21", status: "present" });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/deviations/history?date=2026-05-04&date_to=2026-05-08&activity_group_id=7&start_time=12%3A00",
      expect.objectContaining({ method: "GET" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/instances/42/deviations",
      expect.objectContaining({
        body: JSON.stringify({
          understaffed_ack: true,
          understaffed_note: "keine Vertretung",
          absences: [{ staff_id: 11, reason: "krank" }],
          substitutions: [{ absent_staff_id: 12, substitute_staff_id: 13 }],
          presences: [14],
        }),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/timetable/instances/42/students/21",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ status: "present" }),
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

  it("probes edited occurrences in a window (#1875)", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          count: 2,
          occurrences: [
            {
              instance_id: 101,
              date: "2026-05-04",
              start_time: "15:00:00",
              title: "Fußball AG",
              changes: ["room", "title"],
            },
            {
              instance_id: 102,
              date: "2026-05-11",
              start_time: "15:00:00",
              title: "Fußball AG",
              changes: ["staff"],
            },
          ],
        },
      }),
    );

    await expect(
      timetableService.countEditedInWindow("7", "2026-05-04", "2026-05-15"),
    ).resolves.toEqual({
      count: 2,
      occurrences: [
        {
          instanceId: "101",
          date: "2026-05-04",
          startTime: "15:00:00",
          title: "Fußball AG",
          changes: ["room", "title"],
        },
        {
          instanceId: "102",
          date: "2026-05-11",
          startTime: "15:00:00",
          title: "Fußball AG",
          changes: ["staff"],
        },
      ],
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/instances/edited-in-window?activity_group_id=7&from=2026-05-04&to=2026-05-15",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("maps an empty edited-window probe to zero", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ data: { count: 0, occurrences: [] } }),
    );

    await expect(
      timetableService.countEditedInWindow("7", "2026-05-04", "2026-05-10"),
    ).resolves.toEqual({ count: 0, occurrences: [] });
  });

  it("adds include_deletions only on the split path (#1875)", async () => {
    // A fresh Response per call — a single shared one has its body consumed
    // after the first read and throws on the second.
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({ data: { count: 0, occurrences: [] } }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ data: { count: 0, occurrences: [] } }),
      );

    await timetableService.countEditedInWindow("7", "2026-05-04", "2026-05-10");
    expect(fetchMock).toHaveBeenLastCalledWith(
      "/api/timetable/instances/edited-in-window?activity_group_id=7&from=2026-05-04&to=2026-05-10",
      expect.objectContaining({ method: "GET" }),
    );

    await timetableService.countEditedInWindow(
      "7",
      "2026-05-04",
      "2026-05-10",
      true,
    );
    expect(fetchMock).toHaveBeenLastCalledWith(
      "/api/timetable/instances/edited-in-window?activity_group_id=7&from=2026-05-04&to=2026-05-10&include_deletions=true",
      expect.objectContaining({ method: "GET" }),
    );
  });
});

describe("staff pool + move (#1884)", () => {
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

  it("loads and maps the staff pool", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        status: "success",
        data: {
          instance_id: 42,
          title: "Mensa",
          date: "2026-05-04",
          start_time: "12:30",
          end_time: "13:30",
          dienstplan_in_use: true,
          entries: [],
        },
      }),
    );

    const pool = await timetableService.getStaffPool("42");
    expect(pool.instanceId).toBe("42");
    expect(pool.dienstplanInUse).toBe(true);
    expect(fetchMock).toHaveBeenLastCalledWith(
      "/api/timetable/instances/42/staff-pool",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("posts a move with numeric ids and maps the result", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        status: "success",
        data: {
          target_instance_id: 42,
          source_instance_id: 41,
          action: "moved",
          time_conflicts: [],
          coverage_warnings: [],
        },
      }),
    );

    const result = await timetableService.moveStaff("42", {
      staffId: "7",
      sourceInstanceId: "41",
    });
    expect(result.action).toBe("moved");
    expect(result.sourceInstanceId).toBe("41");
    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(url).toBe("/api/timetable/instances/42/move-staff");
    expect(JSON.parse(String(init.body))).toEqual({
      staff_id: 7,
      source_instance_id: 41,
    });
  });

  it("omits source_instance_id for a pool assign", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        status: "success",
        data: {
          target_instance_id: 42,
          action: "assigned",
          time_conflicts: null,
          coverage_warnings: null,
        },
      }),
    );

    const result = await timetableService.moveStaff("42", { staffId: "9" });
    expect(result.action).toBe("assigned");
    const [, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toEqual({ staff_id: 9 });
  });
});
