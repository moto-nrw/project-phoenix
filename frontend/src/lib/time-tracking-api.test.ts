import { afterEach, describe, it, expect, vi, beforeEach } from "vitest";
import { getSession } from "next-auth/react";
import { timeTrackingService } from "./time-tracking-api";
import type {
  CreateAbsenceRequest,
  UpdateAbsenceRequest,
  UpdateSessionRequest,
} from "./time-tracking-api";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(),
}));

const mockGetSession = vi.mocked(getSession);

// Helper to create a mock fetch response
function mockFetchResponse(data: unknown, ok = true, status = 200) {
  return vi.fn().mockResolvedValue({
    ok,
    status,
    json: vi.fn().mockResolvedValue(data),
  });
}

// Sample backend responses (snake_case, numeric IDs)
const backendSession = {
  id: 1,
  staff_id: 10,
  date: "2025-06-15T00:00:00Z",
  status: "present",
  check_in_time: "2025-06-15T08:00:00Z",
  check_out_time: null,
  break_minutes: 0,
  notes: "",
  auto_checked_out: false,
  created_by: 10,
  updated_by: null,
  created_at: "2025-06-15T08:00:00Z",
  updated_at: "2025-06-15T08:00:00Z",
};

const backendSessionWithCheckout = {
  ...backendSession,
  check_out_time: "2025-06-15T16:30:00Z",
  break_minutes: 30,
};

const backendBreak = {
  id: 5,
  session_id: 1,
  started_at: "2025-06-15T12:00:00Z",
  ended_at: "2025-06-15T12:30:00Z",
  duration_minutes: 30,
  created_at: "2025-06-15T12:00:00Z",
  updated_at: "2025-06-15T12:30:00Z",
};

const backendHistory = {
  ...backendSessionWithCheckout,
  net_minutes: 480,
  is_overtime: false,
  is_break_compliant: true,
  breaks: [backendBreak],
  edit_count: 0,
};

const backendEdit = {
  id: 3,
  session_id: 1,
  staff_id: 10,
  edited_by: 99,
  field_name: "notes",
  old_value: null,
  new_value: "Updated",
  notes: "Changed notes",
  created_at: "2025-06-15T09:00:00Z",
};

const backendAbsence = {
  id: 7,
  staff_id: 10,
  absence_type: "sick",
  date_start: "2025-06-20T00:00:00Z",
  date_end: "2025-06-21T00:00:00Z",
  half_day: false,
  note: "Flu",
  status: "approved",
  approved_by: 99,
  approved_at: "2025-06-20T08:00:00Z",
  created_by: 10,
  created_at: "2025-06-20T07:00:00Z",
  updated_at: "2025-06-20T08:00:00Z",
  duration_days: 2,
};

describe("TimeTrackingService", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetSession.mockResolvedValue({
      user: { token: "test-jwt-token" },
      expires: "2025-12-31",
    } as never);
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  describe("checkIn", () => {
    it("sends POST with status and returns mapped session", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "Checked in",
        data: backendSession,
      });

      const result = await timeTrackingService.checkIn("present");

      expect(global.fetch).toHaveBeenCalledWith("/api/time-tracking/check-in", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer test-jwt-token",
        },
        body: JSON.stringify({ status: "present" }),
      });
      expect(result.id).toBe("1");
      expect(result.staffId).toBe("10");
      expect(result.status).toBe("present");
    });

    it("sends home_office status", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "Checked in",
        data: backendSession,
      });

      await timeTrackingService.checkIn("home_office");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/check-in",
        expect.objectContaining({
          body: JSON.stringify({ status: "home_office" }),
        }),
      );
    });

    it("throws on error response", async () => {
      global.fetch = mockFetchResponse(
        { error: "Already checked in" },
        false,
        409,
      );

      await expect(timeTrackingService.checkIn("present")).rejects.toThrow(
        "Already checked in",
      );
    });

    it("uses fallback error message when API error has no error field", async () => {
      global.fetch = mockFetchResponse({}, false, 500);

      await expect(timeTrackingService.checkIn("present")).rejects.toThrow(
        "Failed to check in",
      );
    });

    it("uses message field when error field is absent", async () => {
      global.fetch = mockFetchResponse({ message: "Server error" }, false, 500);

      await expect(timeTrackingService.checkIn("present")).rejects.toThrow(
        "Server error",
      );
    });
  });

  describe("checkOut", () => {
    it("sends POST and returns mapped session", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "Checked out",
        data: backendSessionWithCheckout,
      });

      const result = await timeTrackingService.checkOut();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/check-out",
        expect.objectContaining({ method: "POST" }),
      );
      expect(result.checkOutTime).toBe("2025-06-15T16:30:00Z");
      expect(result.breakMinutes).toBe(30);
    });

    it("throws on error", async () => {
      global.fetch = mockFetchResponse({ error: "Not checked in" }, false, 400);

      await expect(timeTrackingService.checkOut()).rejects.toThrow(
        "Not checked in",
      );
    });
  });

  describe("getCurrentSession", () => {
    it("returns mapped session when active", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: backendSession,
      });

      const result = await timeTrackingService.getCurrentSession();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/current",
        expect.objectContaining({ method: "GET" }),
      );
      expect(result).not.toBeNull();
      expect(result!.id).toBe("1");
    });

    it("returns null when no active session", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: null,
      });

      const result = await timeTrackingService.getCurrentSession();
      expect(result).toBeNull();
    });
  });

  describe("getHistory", () => {
    it("sends date range and returns mapped history", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: [backendHistory],
      });

      const result = await timeTrackingService.getHistory(
        "2025-06-01",
        "2025-06-30",
      );

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/history?from=2025-06-01&to=2025-06-30",
        expect.objectContaining({ method: "GET" }),
      );
      expect(result).toHaveLength(1);
      expect(result[0]!.netMinutes).toBe(480);
      expect(result[0]!.isOvertime).toBe(false);
      expect(result[0]!.isBreakCompliant).toBe(true);
      expect(result[0]!.breaks).toHaveLength(1);
      expect(result[0]!.breaks[0]!.durationMinutes).toBe(30);
    });
  });

  describe("updateSession", () => {
    it("sends PUT with updates and returns mapped session", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "Updated",
        data: { ...backendSession, notes: "Updated notes" },
      });

      const updates: UpdateSessionRequest = { notes: "Updated notes" };
      const result = await timeTrackingService.updateSession("1", updates);

      expect(global.fetch).toHaveBeenCalledWith("/api/time-tracking/1", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer test-jwt-token",
        },
        body: JSON.stringify(updates),
      });
      expect(result.notes).toBe("Updated notes");
    });
  });

  describe("startBreak", () => {
    it("sends POST and returns mapped break", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "Break started",
        data: { ...backendBreak, ended_at: null, duration_minutes: 0 },
      });

      const result = await timeTrackingService.startBreak();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/break/start",
        expect.objectContaining({ method: "POST" }),
      );
      expect(result.id).toBe("5");
      expect(result.sessionId).toBe("1");
      expect(result.endedAt).toBeNull();
    });
  });

  describe("endBreak", () => {
    it("sends POST and returns mapped session", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "Break ended",
        data: { ...backendSession, break_minutes: 30 },
      });

      const result = await timeTrackingService.endBreak();

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/break/end",
        expect.objectContaining({ method: "POST" }),
      );
      expect(result.breakMinutes).toBe(30);
    });
  });

  describe("getSessionBreaks", () => {
    it("returns mapped breaks for a session", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: [backendBreak],
      });

      const result = await timeTrackingService.getSessionBreaks("1");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/breaks/1",
        expect.objectContaining({ method: "GET" }),
      );
      expect(result).toHaveLength(1);
      expect(result[0]!.startedAt).toBe("2025-06-15T12:00:00Z");
      expect(result[0]!.endedAt).toBe("2025-06-15T12:30:00Z");
    });
  });

  describe("getSessionEdits", () => {
    it("returns mapped edits for a session", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: [backendEdit],
      });

      const result = await timeTrackingService.getSessionEdits("1");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/1/edits",
        expect.objectContaining({ method: "GET" }),
      );
      expect(result).toHaveLength(1);
      expect(result[0]!.fieldName).toBe("notes");
      expect(result[0]!.editedBy).toBe("99");
    });

    it("handles null data gracefully", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: null,
      });

      const result = await timeTrackingService.getSessionEdits("1");
      expect(result).toEqual([]);
    });
  });

  describe("getAbsences", () => {
    it("sends date range and returns mapped absences", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: [backendAbsence],
      });

      const result = await timeTrackingService.getAbsences(
        "2025-06-01",
        "2025-06-30",
      );

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/absences?from=2025-06-01&to=2025-06-30",
        expect.objectContaining({ method: "GET" }),
      );
      expect(result).toHaveLength(1);
      expect(result[0]!.absenceType).toBe("sick");
      expect(result[0]!.durationDays).toBe(2);
      expect(result[0]!.dateStart).toBe("2025-06-20");
    });

    it("handles null data gracefully", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: null,
      });

      const result = await timeTrackingService.getAbsences(
        "2025-06-01",
        "2025-06-30",
      );
      expect(result).toEqual([]);
    });
  });

  describe("createAbsence", () => {
    it("sends POST with absence data and returns mapped absence", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "Created",
        data: backendAbsence,
      });

      const req: CreateAbsenceRequest = {
        absence_type: "sick",
        date_start: "2025-06-20",
        date_end: "2025-06-21",
        note: "Flu",
      };
      const result = await timeTrackingService.createAbsence(req);

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/absences",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify(req),
        }),
      );
      expect(result.id).toBe("7");
      expect(result.note).toBe("Flu");
    });
  });

  describe("updateAbsence", () => {
    it("sends PUT with updates and returns mapped absence", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "Updated",
        data: { ...backendAbsence, note: "Severe flu" },
      });

      const req: UpdateAbsenceRequest = { note: "Severe flu" };
      const result = await timeTrackingService.updateAbsence("7", req);

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/absences/7",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify(req),
        }),
      );
      expect(result.note).toBe("Severe flu");
    });
  });

  describe("deleteAbsence", () => {
    it("sends DELETE request", async () => {
      global.fetch = mockFetchResponse(null, true, 204);

      await timeTrackingService.deleteAbsence("7");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/time-tracking/absences/7",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("throws on error", async () => {
      global.fetch = mockFetchResponse({ error: "Not found" }, false, 404);

      await expect(timeTrackingService.deleteAbsence("999")).rejects.toThrow(
        "Not found",
      );
    });
  });

  describe("authentication", () => {
    it("sends request without auth header when no session", async () => {
      mockGetSession.mockResolvedValue(null);
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: backendSession,
      });

      await timeTrackingService.getCurrentSession();

      const callArgs = (global.fetch as ReturnType<typeof vi.fn>).mock
        .calls[0] as [string, RequestInit];
      const headers = callArgs[1].headers as Record<string, string>;
      expect(headers.Authorization).toBeUndefined();
    });

    it("sends request without auth header when session has no token", async () => {
      mockGetSession.mockResolvedValue({
        user: {},
        expires: "2025-12-31",
      } as never);
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: backendSession,
      });

      await timeTrackingService.getCurrentSession();

      const callArgs = (global.fetch as ReturnType<typeof vi.fn>).mock
        .calls[0] as [string, RequestInit];
      const headers = callArgs[1].headers as Record<string, string>;
      expect(headers.Authorization).toBeUndefined();
    });
  });

  describe("request headers", () => {
    it("includes Content-Type for requests with body", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: backendSession,
      });

      await timeTrackingService.checkIn("present");

      const callArgs = (global.fetch as ReturnType<typeof vi.fn>).mock
        .calls[0] as [string, RequestInit];
      const headers = callArgs[1].headers as Record<string, string>;
      expect(headers["Content-Type"]).toBe("application/json");
    });

    it("omits Content-Type for GET requests", async () => {
      global.fetch = mockFetchResponse({
        success: true,
        message: "",
        data: backendSession,
      });

      await timeTrackingService.getCurrentSession();

      const callArgs = (global.fetch as ReturnType<typeof vi.fn>).mock
        .calls[0] as [string, RequestInit];
      const headers = callArgs[1].headers as Record<string, string>;
      expect(headers["Content-Type"]).toBeUndefined();
    });
  });
});
