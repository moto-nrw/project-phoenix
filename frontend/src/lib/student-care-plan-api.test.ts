import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  fetchStudentCarePlanDay,
  fetchStudentCarePlanWeek,
} from "./student-care-plan-api";

function mockResponse(ok: boolean, body: unknown): Response {
  return { ok, json: async () => body } as unknown as Response;
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
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, { status: "success", data: backendDay }),
      );

      const day = await fetchStudentCarePlanDay("42", "2026-07-21");

      expect(fetch).toHaveBeenCalledWith(
        "/api/timetable/student/42/day?date=2026-07-21",
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
        mockResponse(true, {
          status: "success",
          data: { ...backendDay, instances: null },
        }),
      );
      const day = await fetchStudentCarePlanDay("1", "2026-07-21");
      expect(day.instances).toEqual([]);
    });

    it("defaults an unknown/missing slot source to 'none'", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, {
          status: "success",
          data: {
            ...backendDay,
            arrival: { expected_time: null, source: undefined },
          },
        }),
      );
      const day = await fetchStudentCarePlanDay("1", "2026-07-21");
      expect(day.arrival.source).toBe("none");
    });

    it("throws when the response is not ok", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(mockResponse(false, {}));
      await expect(fetchStudentCarePlanDay("1", "2026-07-21")).rejects.toThrow(
        "Betreuungsplan konnte nicht geladen werden",
      );
    });

    it("throws the backend error message on an error envelope", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, { status: "error", error: "forbidden" }),
      );
      await expect(fetchStudentCarePlanDay("1", "2026-07-21")).rejects.toThrow(
        "forbidden",
      );
    });

    it("throws the fallback when the envelope carries no data", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, { status: "success" }),
      );
      await expect(fetchStudentCarePlanDay("1", "2026-07-21")).rejects.toThrow(
        "Betreuungsplan konnte nicht geladen werden",
      );
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
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, { status: "success", data: backendWeek }),
      );

      const week = await fetchStudentCarePlanWeek(
        "42",
        "2026-07-20",
        "2026-07-24",
      );

      expect(fetch).toHaveBeenCalledWith(
        "/api/timetable/student/42/week?from=2026-07-20&to=2026-07-24",
      );
      expect(week.studentId).toBe("42");
      expect(week.from).toBe("2026-07-20");
      expect(week.to).toBe("2026-07-24");
      expect(week.days).toHaveLength(1);
      expect(week.days[0]?.studentId).toBe("42");
    });

    it("treats null days as an empty list", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        mockResponse(true, {
          status: "success",
          data: { student_id: 1, from: "a", to: "b", days: null },
        }),
      );
      const week = await fetchStudentCarePlanWeek("1", "a", "b");
      expect(week.days).toEqual([]);
    });

    it("throws when the response is not ok", async () => {
      vi.mocked(fetch).mockResolvedValueOnce(mockResponse(false, {}));
      await expect(fetchStudentCarePlanWeek("1", "a", "b")).rejects.toThrow(
        "Betreuungsplan konnte nicht geladen werden",
      );
    });
  });
});
