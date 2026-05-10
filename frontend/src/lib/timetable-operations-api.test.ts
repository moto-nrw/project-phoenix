import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { timetableOperationsApi } from "./timetable-operations-api";

describe("timetableOperationsApi", () => {
  let originalFetch: typeof fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("fetches planned instances with an encoded date query", async () => {
    const mockFetch = vi.mocked(globalThis.fetch);
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          instances: [
            {
              id: 130,
              title: "Lernzeit",
              date: "2026-05-10",
              start_time: "14:00",
              end_time: "15:00",
              room_id: 230,
              status: "planned",
              is_overdue: false,
              minutes_until_start: 10,
              expected_students_count: 12,
              present_students_count: 0,
              assigned_staff_ids: [330],
            },
          ],
        },
      }),
    );

    const result = await timetableOperationsApi.plannedNow("2026-05-10");

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/timetable/operations/planned-now?date=2026-05-10",
      {
        credentials: "include",
        headers: { Accept: "application/json" },
      },
    );
    expect(result[0]?.id).toBe("130");
    expect(result[0]?.assignedStaffIds).toEqual(["330"]);
  });

  it("posts start and maps the response", async () => {
    const mockFetch = vi.mocked(globalThis.fetch);
    mockFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          instance_id: 131,
          active_group_id: 231,
          status: "active",
        },
      }),
    );

    const result = await timetableOperationsApi.start("131");

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/timetable/operations/instances/131/start",
      {
        method: "POST",
        credentials: "include",
        headers: { Accept: "application/json" },
      },
    );
    expect(result).toEqual({
      instanceId: "131",
      activeGroupId: "231",
      status: "active",
    });
  });

  it("fetches rosters by instance and active group", async () => {
    const mockFetch = vi.mocked(globalThis.fetch);
    mockFetch
      .mockResolvedValueOnce(jsonResponse({ data: rosterPayload(132) }))
      .mockResolvedValueOnce(jsonResponse({ data: rosterPayload(133) }));

    const byInstance = await timetableOperationsApi.roster("132");
    const byGroup = await timetableOperationsApi.rosterByActiveGroup("233");

    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/operations/instances/132/roster",
      {
        credentials: "include",
        headers: { Accept: "application/json" },
      },
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/operations/active-groups/233/roster",
      {
        credentials: "include",
        headers: { Accept: "application/json" },
      },
    );
    expect(byInstance.instance.id).toBe("132");
    expect(byGroup.instance.id).toBe("133");
  });

  it("posts check-in and check-out operations", async () => {
    const mockFetch = vi.mocked(globalThis.fetch);
    mockFetch
      .mockResolvedValueOnce(jsonResponse({ data: rosterPayload(134) }))
      .mockResolvedValueOnce(jsonResponse({ data: rosterPayload(134) }));

    await timetableOperationsApi.checkIn("134", "430");
    await timetableOperationsApi.checkOut("134", "430");

    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/operations/instances/134/students/430/check-in",
      {
        method: "POST",
        credentials: "include",
        headers: { Accept: "application/json" },
      },
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/operations/instances/134/students/430/check-out",
      {
        method: "POST",
        credentials: "include",
        headers: { Accept: "application/json" },
      },
    );
  });

  it("patches attendance and completes an instance", async () => {
    const mockFetch = vi.mocked(globalThis.fetch);
    mockFetch
      .mockResolvedValueOnce(jsonResponse({ data: { ok: true } }))
      .mockResolvedValueOnce(jsonResponse({ data: { ok: true } }));

    await timetableOperationsApi.patchAttendance("135", "431", {
      status: "absent",
      substatus: "sick",
      note: "abgemeldet",
    });
    await timetableOperationsApi.complete("135");

    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/operations/instances/135/students/431/attendance",
      {
        method: "PATCH",
        credentials: "include",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          status: "absent",
          substatus: "sick",
          note: "abgemeldet",
        }),
      },
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/operations/instances/135/complete",
      {
        method: "POST",
        credentials: "include",
        headers: { Accept: "application/json" },
      },
    );
  });

  it("uses backend error messages and falls back when JSON parsing fails", async () => {
    const mockFetch = vi.mocked(globalThis.fetch);
    mockFetch
      .mockResolvedValueOnce(
        jsonResponse({ error: "Nicht erlaubt" }, false, 403),
      )
      .mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: () => Promise.reject(new Error("not json")),
      } as Response);

    await expect(timetableOperationsApi.complete("136")).rejects.toThrow(
      "Nicht erlaubt",
    );
    await expect(timetableOperationsApi.complete("136")).rejects.toThrow(
      "Anfrage fehlgeschlagen (HTTP 500)",
    );
  });
});

function jsonResponse(body: unknown, ok = true, status = 200): Response {
  return {
    ok,
    status,
    json: () => Promise.resolve(body),
  } as Response;
}

function rosterPayload(instanceId: number) {
  return {
    instance: {
      id: instanceId,
      title: "Lernzeit",
      status: "active",
      active_group_id: 240,
      room_id: 241,
      room_name: "Raum A",
    },
    rows: [
      {
        student_id: 440,
        student_name: "Mila Muster",
        school_class: "3a",
        group_name: "OGS Blau",
        planned: true,
        is_unplanned: false,
        currently_present: true,
        visit_id: 540,
        status: "present",
      },
    ],
  };
}
