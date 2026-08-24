import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
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
  fetchBulkArrivalTimes,
} from "./student-arrival-api";

const { mockGetSession } = vi.hoisted(() => ({
  mockGetSession: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  getSession: mockGetSession,
}));

function mockFetchResponse(
  data: unknown,
  options: { ok?: boolean; status?: number; text?: string } = {},
): Response {
  const ok = options.ok ?? true;
  const status = options.status ?? 200;
  return {
    ok,
    status,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(options.text ?? ""),
  } as unknown as Response;
}

describe("student-arrival-api", () => {
  const fetchSpy = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    globalThis.fetch = fetchSpy as unknown as typeof fetch;
    mockGetSession.mockResolvedValue({
      user: { token: "test-token" },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
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

  describe("auth headers", () => {
    it("omits Authorization when session has no token", async () => {
      mockGetSession.mockResolvedValueOnce({ user: {} });
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({
          data: { schedules: [], exceptions: [], notes: [] },
        }),
      );

      await fetchArrivalData("42");

      const headers = (fetchSpy.mock.calls[0]![1] as RequestInit)
        .headers as Record<string, string>;
      expect(headers.Authorization).toBeUndefined();
      expect(headers["Content-Type"]).toBe("application/json");
    });

    it("omits Authorization when session is null", async () => {
      mockGetSession.mockResolvedValueOnce(null);
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({ schedules: [], exceptions: [], notes: [] }),
      );

      await fetchArrivalData("42");

      const headers = (fetchSpy.mock.calls[0]![1] as RequestInit)
        .headers as Record<string, string>;
      expect(headers.Authorization).toBeUndefined();
    });
  });

  describe("fetchArrivalData", () => {
    it("fetches and unwraps arrival data", async () => {
      const payload = {
        data: {
          schedules: [{ id: 1, student_id: 42, weekday: 1 }],
          exceptions: [],
          notes: [],
        },
      };
      fetchSpy.mockResolvedValueOnce(mockFetchResponse(payload));

      const result = await fetchArrivalData("42");

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/42/arrival-schedules",
        expect.objectContaining({
          method: "GET",
          credentials: "include",
        }),
      );
      expect(result.schedules).toHaveLength(1);
      expect(result.schedules[0]!.id).toBe(1);
    });

    it("passes the full displayed week to the arrival endpoint", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({ schedules: [], exceptions: [], notes: [] }),
      );

      await fetchArrivalData("42", "2026-08-17", "2026-08-21");

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/42/arrival-schedules?date=2026-08-17&to=2026-08-21",
        expect.objectContaining({ method: "GET" }),
      );
    });

    it("handles unwrapped (no .data wrapper) payloads", async () => {
      const payload = { schedules: [], exceptions: [], notes: [] };
      fetchSpy.mockResolvedValueOnce(mockFetchResponse(payload));

      const result = await fetchArrivalData("42");

      expect(result).toEqual(payload);
    });

    it("throws with server text on non-ok response", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, {
          ok: false,
          status: 500,
          text: "server exploded",
        }),
      );

      await expect(fetchArrivalData("42")).rejects.toThrow(
        "Request failed (500): server exploded",
      );
    });

    it("throws generic error when response has no text body", async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        status: 404,
        text: () => Promise.reject(new Error("stream error")),
      } as unknown as Response);

      await expect(fetchArrivalData("42")).rejects.toThrow(
        "Request failed (404)",
      );
    });
  });

  describe("fetchBulkArrivalScheduleStatus", () => {
    it("uses one request for all selected children", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({ data: { student_ids: [1, 2] } }),
      );

      await expect(fetchBulkArrivalScheduleStatus(["1", "2"])).resolves.toBe(2);

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/arrival-schedules/status",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ student_ids: [1, 2] }),
        }),
      );
    });
  });

  describe("fetchArrivalSettings", () => {
    it("returns the source of the tenant's care days", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({ data: { care_days_source: "bookings" } }),
      );

      const result = await fetchArrivalSettings();

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/arrival-settings",
        expect.objectContaining({
          method: "GET",
          credentials: "include",
        }),
      );
      expect(result).toEqual({ care_days_source: "bookings" });
    });
  });

  describe("updateArrivalSchedules", () => {
    it("sends PUT with schedules body", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: [{ id: 1 }] }));

      const schedules = [
        { weekday: 1, expected_arrival: "08:00" },
        { weekday: 2, expected_arrival: "08:30", notes: "test" },
      ];
      await updateArrivalSchedules("42", schedules);

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/42/arrival-schedules",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ schedules }),
        }),
      );
    });
  });

  describe("bulkUpsertArrivalSchedules", () => {
    it("posts school class and schedules", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: {} }));

      await bulkUpsertArrivalSchedules(
        { type: "school_class", schoolClass: "3a" },
        [{ weekday: 1, expected_arrival: "08:00" }],
      );

      expect(fetchSpy).toHaveBeenCalledWith(
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
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: {} }));

      await bulkUpsertArrivalSchedules({ type: "group", groupId: "17" }, [
        { weekday: 1, expected_arrival: "08:00" },
      ]);

      expect(fetchSpy).toHaveBeenCalledWith(
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
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: {} }));

      await bulkUpsertArrivalSchedules(
        { type: "students", studentIds: ["9", "4"] },
        [{ weekday: 2, expected_arrival: "09:15" }],
      );

      expect(fetchSpy).toHaveBeenCalledWith(
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

      expect(fetchSpy).not.toHaveBeenCalled();
    });
  });

  describe("arrival exceptions CRUD", () => {
    it("createArrivalException posts exception body", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({ data: { id: 77, student_id: 42 } }),
      );

      const result = await createArrivalException("42", {
        exception_date: "2024-01-15",
        expected_arrival: "09:00",
        reason: "late",
      });

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/42/arrival-exceptions",
        expect.objectContaining({ method: "POST" }),
      );
      expect(result.id).toBe(77);
    });

    it("updateArrivalException puts with exceptionId path", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: { id: 77 } }));

      await updateArrivalException("42", 77, {
        exception_date: "2024-01-15",
        expected_arrival: null,
        clear_expected_arrival: true,
      });

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/42/arrival-exceptions/77",
        expect.objectContaining({ method: "PUT" }),
      );
    });

    it("deleteArrivalException resolves on 204", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, { ok: true, status: 204 }),
      );

      await expect(deleteArrivalException("42", 77)).resolves.toBeUndefined();
    });

    it("deleteArrivalException resolves when response is ok", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({}, { ok: true }));

      await expect(deleteArrivalException("42", 77)).resolves.toBeUndefined();
    });

    it("deleteArrivalException throws on non-ok/non-204", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, {
          ok: false,
          status: 500,
          text: "boom",
        }),
      );

      await expect(deleteArrivalException("42", 77)).rejects.toThrow(
        "Request failed (500): boom",
      );
    });

    it("deleteArrivalException throws generic when text read fails", async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        status: 403,
        text: () => Promise.reject(new Error("nope")),
      } as unknown as Response);

      await expect(deleteArrivalException("42", 77)).rejects.toThrow(
        "Request failed (403)",
      );
    });
  });

  describe("arrival notes CRUD", () => {
    it("createArrivalNote posts note body", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({ data: { id: 5, student_id: 42 } }),
      );

      const result = await createArrivalNote("42", {
        note_date: "2024-01-15",
        content: "note",
      });

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/42/arrival-notes",
        expect.objectContaining({ method: "POST" }),
      );
      expect(result.id).toBe(5);
    });

    it("updateArrivalNote puts with noteId path", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: { id: 5 } }));

      await updateArrivalNote("42", 5, {
        note_date: "2024-01-15",
        content: "updated",
      });

      expect(fetchSpy).toHaveBeenCalledWith(
        "/api/students/42/arrival-notes/5",
        expect.objectContaining({ method: "PUT" }),
      );
    });

    it("deleteArrivalNote resolves on 204", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, { ok: true, status: 204 }),
      );

      await expect(deleteArrivalNote("42", 5)).resolves.toBeUndefined();
    });

    it("deleteArrivalNote throws on non-ok status", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse(null, {
          ok: false,
          status: 404,
          text: "missing",
        }),
      );

      await expect(deleteArrivalNote("42", 5)).rejects.toThrow(
        "Request failed (404): missing",
      );
    });

    it("deleteArrivalNote throws generic when text read fails", async () => {
      fetchSpy.mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: () => Promise.reject(new Error("stream")),
      } as unknown as Response);

      await expect(deleteArrivalNote("42", 5)).rejects.toThrow(
        "Request failed (500)",
      );
    });
  });

  describe("fetchBulkArrivalTimes", () => {
    it("returns empty Map when studentIds is empty", async () => {
      const result = await fetchBulkArrivalTimes([]);

      expect(result.size).toBe(0);
      expect(fetchSpy).not.toHaveBeenCalled();
    });

    it("converts string IDs to integers before posting", async () => {
      fetchSpy.mockResolvedValueOnce(mockFetchResponse({ data: [] }));

      await fetchBulkArrivalTimes(["1", "2", "3"], "2024-01-15");

      const body = (fetchSpy.mock.calls[0]![1] as RequestInit).body as string;
      expect(JSON.parse(body)).toEqual({
        student_ids: [1, 2, 3],
        date: "2024-01-15",
      });
    });

    it("maps snake_case backend fields to camelCase", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse({
          data: [
            {
              student_id: 1,
              date: "2024-01-15",
              weekday_name: "Monday",
              expected_arrival: "08:00",
              is_exception: false,
              day_notes: [{ id: 10, content: "hi" }],
              notes: "normal",
            },
            {
              student_id: 2,
              date: "2024-01-15",
              weekday_name: "Monday",
              is_exception: true,
            },
          ],
        }),
      );

      const result = await fetchBulkArrivalTimes(["1", "2"]);

      expect(result.size).toBe(2);
      const first = result.get("1");
      expect(first).toBeDefined();
      expect(first!.studentId).toBe("1");
      expect(first!.weekdayName).toBe("Monday");
      expect(first!.expectedArrival).toBe("08:00");
      expect(first!.isException).toBe(false);
      expect(first!.dayNotes).toEqual([{ id: "10", content: "hi" }]);

      const second = result.get("2");
      expect(second!.dayNotes).toEqual([]);
      expect(second!.expectedArrival).toBeUndefined();
    });

    it("accepts payloads without the outer data wrapper", async () => {
      fetchSpy.mockResolvedValueOnce(
        mockFetchResponse([
          {
            student_id: 5,
            date: "2024-01-15",
            weekday_name: "Monday",
            is_exception: false,
          },
        ]),
      );

      const result = await fetchBulkArrivalTimes(["5"]);
      expect(result.get("5")!.studentId).toBe("5");
    });
  });
});
