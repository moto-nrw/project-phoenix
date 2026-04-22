import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { SchoolCheckinResponse } from "./student-api";

// The student-api module pulls in `getCachedSession` via session-cache and
// performs fetch through authFetch in api-helpers. We mock both at the
// module boundary so the tests stay purely about the mapping + error path.

const { mockGetCachedSession, mockAuthFetch } = vi.hoisted(() => ({
  mockGetCachedSession: vi.fn(),
  mockAuthFetch: vi.fn(),
}));

vi.mock("./session-cache", () => ({
  getCachedSession: mockGetCachedSession,
  sessionFetch: vi.fn(),
}));

vi.mock("./api-helpers", async () => {
  const actual = await vi.importActual<object>("./api-helpers");
  return {
    ...actual,
    authFetch: mockAuthFetch,
    isBrowserContext: vi.fn(() => true),
  };
});

import { schoolCheckinStudent } from "./student-api";

describe("schoolCheckinStudent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetCachedSession.mockResolvedValue({ user: { token: "tok-1" } });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("maps a successful check-in response from snake_case to camelCase", async () => {
    const backendPayload = {
      data: {
        student_id: 42,
        status: "checked_in" as const,
        check_in_time: "2026-04-22T08:00:00Z",
        check_out_time: undefined,
        yard_since: undefined,
        location: "Anwesend" as const,
        changed: true,
      },
    };
    mockAuthFetch.mockResolvedValueOnce(backendPayload);

    const result: SchoolCheckinResponse = await schoolCheckinStudent(
      "42",
      "in",
    );

    expect(result).toEqual({
      studentId: 42,
      status: "checked_in",
      checkInTime: "2026-04-22T08:00:00Z",
      checkOutTime: undefined,
      yardSince: undefined,
      location: "Anwesend",
      changed: true,
    });

    // Verify the proxy was called with the expected URL, method, body, and token.
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/api/students/42/school-checkin",
      expect.objectContaining({
        method: "POST",
        body: { action: "in" },
        token: "tok-1",
      }),
    );
  });

  it("handles an unwrapped response (no `data` envelope)", async () => {
    // extractApiData accepts either shape; the raw object is the fallback path.
    mockAuthFetch.mockResolvedValueOnce({
      student_id: 42,
      status: "checked_out" as const,
      location: "Abwesend" as const,
      changed: true,
      check_out_time: "2026-04-22T14:00:00Z",
    });

    const result = await schoolCheckinStudent("42", "out");
    expect(result.status).toBe("checked_out");
    expect(result.location).toBe("Abwesend");
    expect(result.checkOutTime).toBe("2026-04-22T14:00:00Z");
    expect(result.changed).toBe(true);
  });

  it("propagates errors from the underlying fetch", async () => {
    mockAuthFetch.mockRejectedValueOnce(new Error("503 Service Unavailable"));

    await expect(schoolCheckinStudent("42", "in")).rejects.toThrow(
      "503 Service Unavailable",
    );
  });

  it("sends the correct action on 'out' calls", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      data: {
        student_id: 1,
        status: "checked_out",
        location: "Abwesend",
        changed: false,
      },
    });

    await schoolCheckinStudent("1", "out");

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/api/students/1/school-checkin",
      expect.objectContaining({ body: { action: "out" } }),
    );
  });

  it("passes an undefined token when no session exists", async () => {
    // Logged-out caller shouldn't crash — authFetch handles the missing
    // token downstream (401 from the proxy). We just make sure we don't
    // blow up trying to read `.token` off a null session.
    mockGetCachedSession.mockResolvedValueOnce(null);
    mockAuthFetch.mockResolvedValueOnce({
      data: {
        student_id: 1,
        status: "not_checked_in",
        location: "Abwesend",
        changed: false,
      },
    });

    await schoolCheckinStudent("1", "in");

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/api/students/1/school-checkin",
      expect.objectContaining({ token: undefined }),
    );
  });
});
