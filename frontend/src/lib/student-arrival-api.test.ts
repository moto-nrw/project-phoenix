import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "./api-error";

const mockSessionFetch = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
}));

import {
  WEEKDAYS,
  fetchArrivalSettings,
  fetchArrivalData,
  fetchBulkArrivalScheduleStatus,
  updateArrivalSchedules,
  bulkUpsertArrivalSchedules,
  createArrivalException,
  updateArrivalException,
  deleteArrivalException,
  createArrivalNote,
  updateArrivalNote,
  deleteArrivalNote,
  fetchClassArrivalExceptions,
  deleteClassArrivalException,
} from "./student-arrival-api";

function wrapped(data: unknown): Response {
  return Response.json({ success: true, message: "ok", data });
}

function failed(status: number, text = ""): Response {
  return new Response(text, { status });
}

function noContent(): Response {
  return new Response(null, { status: 204 });
}

function requestInit(call = 0): RequestInit {
  return mockSessionFetch.mock.calls[call]![1] as RequestInit;
}

const backendSchedule = {
  id: 1,
  student_id: 42,
  weekday: 1,
  weekday_name: "Montag",
  expected_arrival: "08:00",
  source: "class_schedule",
  source_class: "3a",
  notes: "Tor",
  created_by: 7,
  created_at: "2026-05-01T00:00:00Z",
  updated_at: "2026-05-02T00:00:00Z",
};

const backendException = {
  id: 77,
  student_id: 42,
  exception_date: "2024-01-15",
  expected_arrival: "09:00",
  reason: "late",
  source: "guardian",
  created_by: 3,
  created_at: "2024-01-10T00:00:00Z",
  updated_at: "2024-01-11T00:00:00Z",
};

const backendNote = {
  id: 5,
  student_id: 42,
  note_date: "2024-01-15",
  content: "note",
  created_by: 3,
  created_at: "2024-01-10T00:00:00Z",
  updated_at: "2024-01-11T00:00:00Z",
};

describe("student-arrival-api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("WEEKDAYS", () => {
    it("exposes the 5 schooldays Monday-Friday", () => {
      expect(WEEKDAYS).toHaveLength(5);
      expect(WEEKDAYS[0]).toEqual({
        value: 1,
        label: "Montag",
        short: "Mo",
      });
      expect(WEEKDAYS.at(-1)).toEqual({
        value: 5,
        label: "Freitag",
        short: "Fr",
      });
    });
  });

  describe("fetchArrivalData", () => {
    it("fetches through the session and maps the route's data to string IDs and camelCase", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        wrapped({
          schedules: [backendSchedule],
          exceptions: [backendException],
          notes: [backendNote],
        }),
      );

      const result = await fetchArrivalData("42");

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/arrival-schedules",
        expect.objectContaining({ method: "GET" }),
      );
      // The BFF reads the session cookie; the browser sends no token (#5005).
      expect(requestInit().headers).toBeUndefined();
      expect(result).toEqual({
        schedules: [
          {
            id: "1",
            studentId: "42",
            weekday: 1,
            weekdayName: "Montag",
            expectedArrival: "08:00",
            source: "class_schedule",
            sourceClass: "3a",
            notes: "Tor",
            createdBy: "7",
            createdAt: "2026-05-01T00:00:00Z",
            updatedAt: "2026-05-02T00:00:00Z",
          },
        ],
        exceptions: [
          {
            id: "77",
            studentId: "42",
            exceptionDate: "2024-01-15",
            expectedArrival: "09:00",
            reason: "late",
            source: "guardian",
            createdBy: "3",
            createdAt: "2024-01-10T00:00:00Z",
            updatedAt: "2024-01-11T00:00:00Z",
          },
        ],
        notes: [
          {
            id: "5",
            studentId: "42",
            noteDate: "2024-01-15",
            content: "note",
            createdBy: "3",
            createdAt: "2024-01-10T00:00:00Z",
            updatedAt: "2024-01-11T00:00:00Z",
          },
        ],
      });
    });

    it("passes the full displayed week to the arrival endpoint", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        wrapped({ schedules: [], exceptions: [], notes: [] }),
      );

      await fetchArrivalData("42", "2026-08-17", "2026-08-21");

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/arrival-schedules?date=2026-08-17&to=2026-08-21",
        expect.objectContaining({ method: "GET" }),
      );
    });

    it("treats missing lists as empty", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        wrapped({ schedules: null, exceptions: null, notes: null }),
      );

      await expect(fetchArrivalData("42")).resolves.toEqual({
        schedules: [],
        exceptions: [],
        notes: [],
      });
    });

    it("throws an ApiError with the status on a non-ok response", async () => {
      mockSessionFetch.mockResolvedValueOnce(failed(500, "server exploded"));

      const error = await fetchArrivalData("42").catch((err: unknown) => err);

      expect(error).toBeInstanceOf(ApiError);
      expect(error).toMatchObject({
        status: 500,
        message: "GET /api/students/42/arrival-schedules failed (500)",
      });
    });

    it("keeps code, status and request ID of the envelope as an ApiError", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        failed(
          409,
          JSON.stringify({
            status: "error",
            error: "class not found",
            code: "classday.arrival_exception_class_not_found",
            instance: "req-arrival",
          }),
        ),
      );

      const error = await fetchArrivalData("42").catch((err: unknown) => err);

      expect(error).toBeInstanceOf(ApiError);
      expect(error).toMatchObject({
        status: 409,
        code: "classday.arrival_exception_class_not_found",
        requestId: "req-arrival",
      });
    });

    it("passes a transport failure on unchanged", async () => {
      const unavailable = new ApiError("Network unavailable", 503, {
        code: "general.unavailable",
      });
      mockSessionFetch.mockRejectedValueOnce(unavailable);

      await expect(fetchArrivalData("42")).rejects.toBe(unavailable);
    });
  });

  describe("fetchBulkArrivalScheduleStatus", () => {
    it("uses one request for all selected children", async () => {
      mockSessionFetch.mockResolvedValueOnce(wrapped({ student_ids: [1, 2] }));

      await expect(fetchBulkArrivalScheduleStatus(["1", "2"])).resolves.toBe(2);

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/arrival-schedules/status",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ student_ids: [1, 2] }),
        }),
      );
    });
  });

  describe("fetchArrivalSettings", () => {
    it("returns the source of the tenant's care days and school periods", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        wrapped({
          care_days_source: "bookings",
          school_periods: [
            { period: 1, end_time: "08:45" },
            { period: 2, end_time: "09:35" },
          ],
          default_arrival_time: "12:30",
          default_pickup_time: "16:00",
        }),
      );

      const result = await fetchArrivalSettings();

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/arrival-settings",
        expect.objectContaining({ method: "GET" }),
      );
      expect(result).toEqual({
        careDaysSource: "bookings",
        schoolPeriods: [
          { period: 1, endTime: "08:45" },
          { period: 2, endTime: "09:35" },
        ],
        defaultArrivalTime: "12:30",
        defaultPickupTime: "16:00",
      });
    });
  });

  describe("updateArrivalSchedules", () => {
    it("sends PUT with schedules body and maps the saved arrival data", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        wrapped({ schedules: [backendSchedule], exceptions: [], notes: [] }),
      );

      const schedules = [
        { weekday: 1, expected_arrival: "08:00" },
        { weekday: 2, expected_arrival: "08:30", notes: "test" },
      ];
      const result = await updateArrivalSchedules("42", schedules);

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/arrival-schedules",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ schedules }),
        }),
      );
      expect(result.schedules[0]).toMatchObject({ id: "1", studentId: "42" });
      expect(result.exceptions).toEqual([]);
      expect(result.notes).toEqual([]);
    });
  });

  describe("bulkUpsertArrivalSchedules", () => {
    it("posts school class and schedules", async () => {
      mockSessionFetch.mockResolvedValueOnce(wrapped({}));

      await bulkUpsertArrivalSchedules(
        { type: "school_class", schoolClass: "3a" },
        [{ weekday: 1, expected_arrival: "08:00" }],
      );

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/arrival-schedules/bulk",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            schedules: [{ weekday: 1, expected_arrival: "08:00" }],
            school_class: "3a",
          }),
        }),
      );
    });

    it("posts group ID and schedules", async () => {
      mockSessionFetch.mockResolvedValueOnce(wrapped({}));

      await bulkUpsertArrivalSchedules({ type: "group", groupId: "17" }, [
        { weekday: 1, expected_arrival: "08:00" },
      ]);

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/arrival-schedules/bulk",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            schedules: [{ weekday: 1, expected_arrival: "08:00" }],
            group_id: 17,
          }),
        }),
      );
    });

    it("posts selected student IDs and schedules", async () => {
      mockSessionFetch.mockResolvedValueOnce(wrapped({}));

      await bulkUpsertArrivalSchedules(
        { type: "students", studentIds: ["9", "4"] },
        [{ weekday: 2, expected_arrival: "09:15" }],
      );

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/arrival-schedules/bulk",
        expect.objectContaining({
          body: JSON.stringify({
            schedules: [{ weekday: 2, expected_arrival: "09:15" }],
            student_ids: [9, 4],
          }),
        }),
      );
    });

    it("rejects an invalid group ID before sending a request", async () => {
      await expect(
        bulkUpsertArrivalSchedules({ type: "group", groupId: "not-an-id" }, [
          { weekday: 1, expected_arrival: "08:00" },
        ]),
      ).rejects.toThrow("Ungültige Gruppen-ID");

      expect(mockSessionFetch).not.toHaveBeenCalled();
    });
  });

  describe("arrival exceptions CRUD", () => {
    it("createArrivalException posts exception body and maps the answer", async () => {
      mockSessionFetch.mockResolvedValueOnce(wrapped(backendException));

      const input = {
        exception_date: "2024-01-15",
        expected_arrival: "09:00",
        reason: "late",
      };
      const result = await createArrivalException("42", input);

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/arrival-exceptions",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify(input),
        }),
      );
      expect(result).toMatchObject({
        id: "77",
        studentId: "42",
        exceptionDate: "2024-01-15",
      });
    });

    it("updateArrivalException puts with exceptionId path", async () => {
      mockSessionFetch.mockResolvedValueOnce(wrapped(backendException));

      await updateArrivalException("42", "77", {
        exception_date: "2024-01-15",
        expected_arrival: null,
        clear_expected_arrival: true,
      });

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/arrival-exceptions/77",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({
            exception_date: "2024-01-15",
            expected_arrival: null,
            clear_expected_arrival: true,
          }),
        }),
      );
    });

    it("deleteArrivalException resolves on 204", async () => {
      mockSessionFetch.mockResolvedValueOnce(noContent());

      await expect(deleteArrivalException("42", "77")).resolves.toBeUndefined();
      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/arrival-exceptions/77",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("deleteArrivalException resolves when response is ok", async () => {
      mockSessionFetch.mockResolvedValueOnce(wrapped({}));

      await expect(deleteArrivalException("42", "77")).resolves.toBeUndefined();
    });

    it("deleteArrivalException throws on non-ok/non-204", async () => {
      mockSessionFetch.mockResolvedValueOnce(failed(500, "boom"));

      await expect(deleteArrivalException("42", "77")).rejects.toMatchObject({
        status: 500,
        message: "DELETE /api/students/42/arrival-exceptions/77 failed (500)",
      });
    });

    it("deleteArrivalException keeps the status when the body cannot be read", async () => {
      mockSessionFetch.mockResolvedValueOnce({
        ok: false,
        status: 403,
        text: () => Promise.reject(new Error("nope")),
      } as unknown as Response);

      const error = await deleteArrivalException("42", "77").catch(
        (err: unknown) => err,
      );

      expect(error).toBeInstanceOf(ApiError);
      expect(error).toMatchObject({ status: 403 });
    });
  });

  describe("arrival notes CRUD", () => {
    it("createArrivalNote posts note body and maps the answer", async () => {
      mockSessionFetch.mockResolvedValueOnce(wrapped(backendNote));

      const result = await createArrivalNote("42", {
        note_date: "2024-01-15",
        content: "note",
      });

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/arrival-notes",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ note_date: "2024-01-15", content: "note" }),
        }),
      );
      expect(result).toMatchObject({
        id: "5",
        studentId: "42",
        noteDate: "2024-01-15",
      });
    });

    it("updateArrivalNote puts with noteId path", async () => {
      mockSessionFetch.mockResolvedValueOnce(wrapped(backendNote));

      await updateArrivalNote("42", "5", {
        note_date: "2024-01-15",
        content: "updated",
      });

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/arrival-notes/5",
        expect.objectContaining({ method: "PUT" }),
      );
    });

    it("deleteArrivalNote resolves on 204", async () => {
      mockSessionFetch.mockResolvedValueOnce(noContent());

      await expect(deleteArrivalNote("42", "5")).resolves.toBeUndefined();
      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/42/arrival-notes/5",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("deleteArrivalNote throws on non-ok status", async () => {
      mockSessionFetch.mockResolvedValueOnce(failed(404, "missing"));

      await expect(deleteArrivalNote("42", "5")).rejects.toMatchObject({
        status: 404,
        message: "DELETE /api/students/42/arrival-notes/5 failed (404)",
      });
    });

    it("deleteArrivalNote keeps the status when the body cannot be read", async () => {
      mockSessionFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: () => Promise.reject(new Error("stream")),
      } as unknown as Response);

      const error = await deleteArrivalNote("42", "5").catch(
        (err: unknown) => err,
      );

      expect(error).toBeInstanceOf(ApiError);
      expect(error).toMatchObject({ status: 500 });
    });
  });

  describe("class arrival exceptions", () => {
    it("encodes the class into the path and returns the route's data", async () => {
      const list = { school_class: "3/a", can_edit: true, exceptions: [] };
      mockSessionFetch.mockResolvedValueOnce(wrapped(list));

      await expect(fetchClassArrivalExceptions("3/a")).resolves.toEqual(list);
      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/class-arrival-exceptions/3%2Fa",
        expect.objectContaining({ method: "GET" }),
      );
    });

    it("deletes one date and resolves on 204", async () => {
      mockSessionFetch.mockResolvedValueOnce(noContent());

      await expect(
        deleteClassArrivalException("3a", "2026-09-10"),
      ).resolves.toBeUndefined();
      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/students/class-arrival-exceptions/3a/2026-09-10",
        expect.objectContaining({ method: "DELETE" }),
      );
    });
  });
});
