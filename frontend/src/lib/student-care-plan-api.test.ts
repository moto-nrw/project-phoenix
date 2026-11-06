import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./session-cache", async () => {
  const { transportFetch } =
    await vi.importActual<typeof import("./api-error")>("./api-error");
  return { sessionFetch: transportFetch };
});

import {
  correctAttendance,
  fetchAttendanceCorrections,
  fetchStudentCarePlanDay,
  fetchStudentCarePlanWeek,
} from "./student-care-plan-api";

/** The route wrapper's answer, or a failure without an envelope. */
function mockResponse(ok: boolean, data?: unknown): Response {
  return ok
    ? Response.json({ success: true, data })
    : new Response("", { status: 500 });
}

const backendInstance = {
  id: 7,
  title: "Mittagessen",
  start_time: "12:00",
  end_time: "13:00",
  room_id: 3,
  status: "planned",
  active_group_id: 9,
  attendance: {
    status: "present",
    substatus: "late",
    note: "kam später",
    checked_in_at: "2026-07-21T12:05:00+02:00",
    checked_out_at: null,
    is_unplanned: false,
  },
};

const backendDay = {
  student_id: 42,
  date: "2026-07-21",
  weekday: 2,
  // arrival: schedule with no reason; pickup: exception with reason + null time.
  arrival: { expected_time: "08:00", source: "schedule", reason: null },
  instances: [
    backendInstance,
    { ...backendInstance, id: 8, active_group_id: null },
  ],
  pickup: {
    expected_time: null,
    source: "exception",
    reason: "früher abgeholt",
  },
};

describe("student-care-plan-api", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe("fetchStudentCarePlanDay", () => {
    it("requests the day endpoint and maps the response to camelCase", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(mockResponse(true, backendDay));

      const day = await fetchStudentCarePlanDay("42", "2026-07-21");

      expect(fetch).toHaveBeenCalledWith(
        "/api/timetable/student/42/day?date=2026-07-21",
        expect.objectContaining({ method: "GET" }),
      );
      expect(day.studentId).toBe("42");
      expect(day.date).toBe("2026-07-21");
      expect(day.weekday).toBe(2);
      // arrival: schedule, empty reason dropped.
      expect(day.arrival).toEqual({
        expectedTime: "08:00",
        source: "schedule",
      });
      // pickup: exception carries a reason and a null time (absence signal).
      expect(day.pickup).toEqual({
        expectedTime: null,
        source: "exception",
        reason: "früher abgeholt",
      });
      // instances: int64 ids -> strings, attendance snake -> camel.
      expect(day.instances).toHaveLength(2);
      expect(day.instances[0]).toMatchObject({
        id: "7",
        roomId: "3",
        activeGroupId: "9",
        attendance: {
          status: "present",
          substatus: "late",
          note: "kam später",
          checkedInAt: "2026-07-21T12:05:00+02:00",
          checkedOutAt: null,
          isUnplanned: false,
        },
      });
      // second instance has a null active_group_id -> null.
      expect(day.instances[1]?.activeGroupId).toBeNull();
    });

    it("treats null instances as an empty list", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, { ...backendDay, instances: null }),
      );
      const day = await fetchStudentCarePlanDay("1", "2026-07-21");
      expect(day.instances).toEqual([]);
    });

    it("defaults an unknown/missing slot source to 'none'", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, {
          ...backendDay,
          arrival: { expected_time: null, source: undefined },
        }),
      );
      const day = await fetchStudentCarePlanDay("1", "2026-07-21");
      expect(day.arrival.source).toBe("none");
    });

    it("throws when the response is not ok", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(mockResponse(false));
      await expect(
        fetchStudentCarePlanDay("1", "2026-07-21"),
      ).rejects.toMatchObject({ status: 500, code: "general.server" });
    });
  });

  describe("fetchStudentCarePlanWeek", () => {
    it("requests the week endpoint and maps each day", async () => {
      const backendWeek = {
        student_id: 42,
        from: "2026-07-20",
        to: "2026-07-24",
        days: [backendDay],
      };
      vi.mocked(fetch).mockResolvedValueOnce(mockResponse(true, backendWeek));

      const week = await fetchStudentCarePlanWeek(
        "42",
        "2026-07-20",
        "2026-07-24",
      );

      expect(fetch).toHaveBeenCalledWith(
        "/api/timetable/student/42/week?from=2026-07-20&to=2026-07-24",
        expect.objectContaining({ method: "GET" }),
      );
      expect(week.studentId).toBe("42");
      expect(week.from).toBe("2026-07-20");
      expect(week.to).toBe("2026-07-24");
      expect(week.days).toHaveLength(1);
      expect(week.days[0]?.studentId).toBe("42");
    });

    it("treats null days as an empty list", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, { student_id: 1, from: "a", to: "b", days: null }),
      );
      const week = await fetchStudentCarePlanWeek("1", "a", "b");
      expect(week.days).toEqual([]);
    });

    it("throws when the response is not ok", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(mockResponse(false));
      await expect(
        fetchStudentCarePlanWeek("1", "a", "b"),
      ).rejects.toMatchObject({ status: 500, code: "general.server" });
    });
  });

  describe("attendance corrections", () => {
    const correction = {
      field_name: "status",
      old_value: "expected",
      new_value: "present",
      reason: "vergessen",
      corrected_at: "2026-07-21T15:00:00+02:00",
    };

    it("reads the correction trail of a child in a block", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, { corrections: [correction] }),
      );

      await expect(fetchAttendanceCorrections("7", "42")).resolves.toEqual([
        correction,
      ]);
      expect(fetch).toHaveBeenCalledWith(
        "/api/timetable/instances/7/students/42/corrections",
        expect.objectContaining({ method: "GET" }),
      );
    });

    it("posts a correction with its reason", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(mockResponse(true, {}));

      await correctAttendance("7", "42", {
        reason: "vergessen",
        status: "present",
      });

      expect(fetch).toHaveBeenCalledWith(
        "/api/timetable/instances/7/students/42/correction",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ reason: "vergessen", status: "present" }),
        }),
      );
    });
  });
});
