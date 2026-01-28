import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  fetchStudentPickupData,
  updateStudentPickupSchedules,
  createStudentPickupException,
  updateStudentPickupException,
  deleteStudentPickupException,
  fetchBulkPickupTimes,
  type BulkPickupTimeResponse,
} from "./pickup-schedule-api";
import type {
  BackendPickupData,
  BulkPickupScheduleFormData,
  PickupExceptionFormData,
  BackendPickupException,
  BackendPickupSchedule,
} from "./pickup-schedule-helpers";

// Mock data
const mockBackendSchedule: BackendPickupSchedule = {
  id: 1,
  student_id: 123,
  weekday: 1,
  weekday_name: "Montag",
  pickup_time: "15:30",
  notes: "Test notes",
  created_by: 1,
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
};

const mockBackendException: BackendPickupException = {
  id: 2,
  student_id: 123,
  exception_date: "2024-01-20",
  pickup_time: "16:00",
  reason: "Arzttermin",
  created_by: 1,
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T10:00:00Z",
};

const mockBackendPickupData: BackendPickupData = {
  schedules: [mockBackendSchedule],
  exceptions: [mockBackendException],
};

// Helper to create mock fetch response
function createMockResponse(
  ok: boolean,
  status: number,
  data?: unknown,
  jsonError?: boolean,
): Response {
  return {
    ok,
    status,
    statusText: ok ? "OK" : "Error",
    json: () =>
      jsonError
        ? Promise.reject(new Error("JSON parse error"))
        : Promise.resolve(data),
  } as Response;
}

describe("pickup-schedule-api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("fetchStudentPickupData", () => {
    it("fetches and maps pickup data successfully", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: mockBackendPickupData,
        }),
      );

      const result = await fetchStudentPickupData("123");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/students/123/pickup-schedules",
      );
      expect(result.schedules).toHaveLength(1);
      expect(result.schedules[0]).toEqual({
        id: "1",
        studentId: "123",
        weekday: 1,
        weekdayName: "Montag",
        pickupTime: "15:30",
        notes: "Test notes",
        createdBy: "1",
        createdAt: "2024-01-15T10:00:00Z",
        updatedAt: "2024-01-15T10:00:00Z",
      });
      expect(result.exceptions).toHaveLength(1);
      expect(result.exceptions[0]).toEqual({
        id: "2",
        studentId: "123",
        exceptionDate: "2024-01-20",
        pickupTime: "16:00",
        reason: "Arzttermin",
        createdBy: "1",
        createdAt: "2024-01-15T10:00:00Z",
        updatedAt: "2024-01-15T10:00:00Z",
      });
    });

    it("returns empty arrays when data is undefined", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: undefined,
        }),
      );

      const result = await fetchStudentPickupData("123");

      expect(result.schedules).toEqual([]);
      expect(result.exceptions).toEqual([]);
    });

    it("throws translated error on non-ok response with JSON error", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(false, 404, { error: "student not found" }),
        );

      await expect(fetchStudentPickupData("999")).rejects.toThrow(
        "Schüler/in nicht gefunden",
      );
    });

    it("throws translated error when JSON parse fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(false, 500, undefined, true));

      await expect(fetchStudentPickupData("123")).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("throws error when response status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "error",
          error: "Database error",
        }),
      );

      await expect(fetchStudentPickupData("123")).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("translates unauthorized error", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(false, 401, { error: "unauthorized" }),
        );

      await expect(fetchStudentPickupData("123")).rejects.toThrow(
        "Keine Berechtigung",
      );
    });

    it("translates forbidden error", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(false, 403, { error: "forbidden" }),
        );

      await expect(fetchStudentPickupData("123")).rejects.toThrow(
        "Zugriff verweigert",
      );
    });
  });

  describe("updateStudentPickupSchedules", () => {
    const formData: BulkPickupScheduleFormData = {
      schedules: [
        {
          weekday: 1,
          pickupTime: "15:30",
          notes: "Updated notes",
        },
      ],
    };

    it("updates pickup schedules and returns mapped data", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: mockBackendPickupData,
        }),
      );

      const result = await updateStudentPickupSchedules("123", formData);

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/students/123/pickup-schedules",
        {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            schedules: [
              {
                weekday: 1,
                pickup_time: "15:30",
                notes: "Updated notes",
              },
            ],
          }),
        },
      );
      expect(result.schedules).toHaveLength(1);
      expect(result.exceptions).toHaveLength(1);
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(false, 400, { error: "invalid weekday" }),
        );

      await expect(
        updateStudentPickupSchedules("123", formData),
      ).rejects.toThrow("Ungültiger Wochentag");
    });

    it("throws translated error when JSON parse fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(false, 500, undefined, true));

      await expect(
        updateStudentPickupSchedules("123", formData),
      ).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("throws error when response status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "error",
          error: "pickup_time is required",
        }),
      );

      await expect(
        updateStudentPickupSchedules("123", formData),
      ).rejects.toThrow("Abholzeit ist erforderlich");
    });

    it("throws error when data is missing", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: null,
        }),
      );

      await expect(
        updateStudentPickupSchedules("123", formData),
      ).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("translates invalid pickup_time format error", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(false, 400, {
          error: "invalid pickup_time format",
        }),
      );

      await expect(
        updateStudentPickupSchedules("123", formData),
      ).rejects.toThrow("Ungültiges Zeitformat (erwartet HH:MM)");
    });
  });

  describe("createStudentPickupException", () => {
    const exceptionData: PickupExceptionFormData = {
      exceptionDate: "2024-01-25",
      pickupTime: "16:30",
      reason: "Zahnarzt",
    };

    it("creates pickup exception and returns mapped result", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 201, {
          status: "success",
          data: mockBackendException,
        }),
      );

      const result = await createStudentPickupException("123", exceptionData);

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/students/123/pickup-exceptions",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            exception_date: "2024-01-25",
            pickup_time: "16:30",
            reason: "Zahnarzt",
          }),
        },
      );
      expect(result.id).toBe("2");
      expect(result.exceptionDate).toBe("2024-01-20");
      expect(result.reason).toBe("Arzttermin");
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(false, 400, {
          error: "exception_date is required",
        }),
      );

      await expect(
        createStudentPickupException("123", exceptionData),
      ).rejects.toThrow("Datum ist erforderlich");
    });

    it("throws translated error when JSON parse fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(false, 500, undefined, true));

      await expect(
        createStudentPickupException("123", exceptionData),
      ).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("throws error when response status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "error",
          error: "exception already exists",
        }),
      );

      await expect(
        createStudentPickupException("123", exceptionData),
      ).rejects.toThrow("Für dieses Datum existiert bereits eine Ausnahme");
    });

    it("throws error when data is missing", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: null,
        }),
      );

      await expect(
        createStudentPickupException("123", exceptionData),
      ).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("translates invalid exception_date format error", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(false, 400, {
          error: "invalid exception_date format",
        }),
      );

      await expect(
        createStudentPickupException("123", exceptionData),
      ).rejects.toThrow("Ungültiges Datumsformat (erwartet JJJJ-MM-TT)");
    });

    it("translates reason is required error", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(false, 400, { error: "reason is required" }),
        );

      await expect(
        createStudentPickupException("123", exceptionData),
      ).rejects.toThrow("Grund ist erforderlich");
    });
  });

  describe("updateStudentPickupException", () => {
    const exceptionData: PickupExceptionFormData = {
      exceptionDate: "2024-01-25",
      pickupTime: "17:00",
      reason: "Updated reason",
    };

    it("updates pickup exception and returns mapped result", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: mockBackendException,
        }),
      );

      const result = await updateStudentPickupException(
        "123",
        "456",
        exceptionData,
      );

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/students/123/pickup-exceptions/456",
        {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            exception_date: "2024-01-25",
            pickup_time: "17:00",
            reason: "Updated reason",
          }),
        },
      );
      expect(result.id).toBe("2");
      expect(result.reason).toBe("Arzttermin");
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(false, 404, { error: "student not found" }),
        );

      await expect(
        updateStudentPickupException("999", "456", exceptionData),
      ).rejects.toThrow("Schüler/in nicht gefunden");
    });

    it("throws translated error when JSON parse fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(false, 500, undefined, true));

      await expect(
        updateStudentPickupException("123", "456", exceptionData),
      ).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("throws error when response status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "error",
          error: "full access required",
        }),
      );

      await expect(
        updateStudentPickupException("123", "456", exceptionData),
      ).rejects.toThrow("Vollzugriff erforderlich");
    });

    it("throws error when data is missing", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: null,
        }),
      );

      await expect(
        updateStudentPickupException("123", "456", exceptionData),
      ).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });
  });

  describe("deleteStudentPickupException", () => {
    it("deletes exception with 204 No Content response", async () => {
      global.fetch = vi.fn().mockResolvedValue(createMockResponse(true, 204));

      await expect(
        deleteStudentPickupException("123", "456"),
      ).resolves.toBeUndefined();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/students/123/pickup-exceptions/456",
        {
          method: "DELETE",
        },
      );
    });

    it("deletes exception with JSON success response", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
        }),
      );

      await expect(
        deleteStudentPickupException("123", "456"),
      ).resolves.toBeUndefined();
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(false, 404, { error: "student not found" }),
        );

      await expect(deleteStudentPickupException("999", "456")).rejects.toThrow(
        "Schüler/in nicht gefunden",
      );
    });

    it("throws translated error when JSON parse fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(false, 500, undefined, true));

      await expect(deleteStudentPickupException("123", "456")).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("throws error when response status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "error",
          error: "unauthorized",
        }),
      );

      await expect(deleteStudentPickupException("123", "456")).rejects.toThrow(
        "Keine Berechtigung",
      );
    });
  });

  describe("fetchBulkPickupTimes", () => {
    const mockBulkResponse: BulkPickupTimeResponse[] = [
      {
        student_id: 123,
        date: "2024-01-22",
        weekday_name: "Montag",
        pickup_time: "15:30",
        is_exception: false,
        notes: "Regular schedule",
      },
      {
        student_id: 456,
        date: "2024-01-22",
        weekday_name: "Montag",
        pickup_time: "16:00",
        is_exception: true,
        reason: "Arzttermin",
      },
    ];

    it("fetches bulk pickup times and returns Map", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: mockBulkResponse,
        }),
      );

      const result = await fetchBulkPickupTimes(["123", "456"], "2024-01-22");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/students/pickup-times/bulk",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            student_ids: [123, 456],
            date: "2024-01-22",
          }),
        },
      );

      expect(result).toBeInstanceOf(Map);
      expect(result.size).toBe(2);

      const student123 = result.get("123");
      expect(student123).toEqual({
        studentId: "123",
        date: "2024-01-22",
        weekdayName: "Montag",
        pickupTime: "15:30",
        isException: false,
        notes: "Regular schedule",
        reason: undefined,
      });

      const student456 = result.get("456");
      expect(student456).toEqual({
        studentId: "456",
        date: "2024-01-22",
        weekdayName: "Montag",
        pickupTime: "16:00",
        isException: true,
        reason: "Arzttermin",
        notes: undefined,
      });
    });

    it("returns empty Map when studentIds array is empty", async () => {
      const result = await fetchBulkPickupTimes([]);

      expect(result).toBeInstanceOf(Map);
      expect(result.size).toBe(0);
      expect(global.fetch).not.toHaveBeenCalled();
    });

    it("fetches without date parameter (defaults to today)", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: [],
        }),
      );

      await fetchBulkPickupTimes(["123"]);

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/students/pickup-times/bulk",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            student_ids: [123],
            date: undefined,
          }),
        },
      );
    });

    it("throws translated error on non-ok response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(false, 401, { error: "unauthorized" }),
        );

      await expect(fetchBulkPickupTimes(["123"])).rejects.toThrow(
        "Keine Berechtigung",
      );
    });

    it("throws translated error when JSON parse fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(false, 500, undefined, true));

      await expect(fetchBulkPickupTimes(["123"])).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("throws error when response status is error", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "error",
          error: "Database error",
        }),
      );

      await expect(fetchBulkPickupTimes(["123"])).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("throws error when data is missing", async () => {
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: null,
        }),
      );

      await expect(fetchBulkPickupTimes(["123"])).rejects.toThrow(
        "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
      );
    });

    it("handles response with optional fields undefined", async () => {
      const minimalResponse: BulkPickupTimeResponse[] = [
        {
          student_id: 789,
          date: "2024-01-22",
          weekday_name: "Montag",
          is_exception: false,
        },
      ];

      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse(true, 200, {
          status: "success",
          data: minimalResponse,
        }),
      );

      const result = await fetchBulkPickupTimes(["789"]);

      const student789 = result.get("789");
      expect(student789).toEqual({
        studentId: "789",
        date: "2024-01-22",
        weekdayName: "Montag",
        pickupTime: undefined,
        isException: false,
        reason: undefined,
        notes: undefined,
      });
    });
  });
});
