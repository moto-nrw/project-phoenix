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

import {
  schoolCheckinStudent,
  schoolCheckinStudentsBatch,
} from "./student-api";

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
      // Backend int64 -> frontend string at the boundary (CLAUDE.md §4).
      studentId: "42",
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

describe("schoolCheckinStudentsBatch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetCachedSession.mockResolvedValue({ user: { token: "tok-1" } });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("maps a batch response and keeps IDs as strings on the wire", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      data: {
        action: "out" as const,
        succeeded: 2,
        failed: 1,
        results: [
          {
            student_id: "42",
            ok: true,
            changed: true,
            location: "Abwesend" as const,
          },
          {
            student_id: "43",
            ok: true,
            changed: false,
            location: "Abwesend" as const,
          },
          { student_id: "99", ok: false, changed: false, error: "not_found" },
        ],
      },
    });

    const outcome = await schoolCheckinStudentsBatch(["42", "43", "99"], "out");

    expect(outcome).toEqual({
      action: "out",
      succeeded: 2,
      failed: 1,
      results: [
        {
          studentId: "42",
          ok: true,
          changed: true,
          location: "Abwesend",
          error: undefined,
        },
        {
          studentId: "43",
          ok: true,
          changed: false,
          location: "Abwesend",
          error: undefined,
        },
        {
          studentId: "99",
          ok: false,
          changed: false,
          location: undefined,
          error: "not_found",
        },
      ],
    });

    // The wire body carries the IDs as STRINGS — Number() conversion would
    // corrupt int64 IDs above 2^53-1 (review #2372).
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/api/students/school-checkin/batch",
      expect.objectContaining({
        method: "POST",
        body: { action: "out", student_ids: ["42", "43", "99"] },
        token: "tok-1",
      }),
    );
  });

  it("handles an unwrapped response (no `data` envelope)", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      action: "in" as const,
      succeeded: 1,
      failed: 0,
      results: [
        { student_id: "7", ok: true, changed: true, location: "Anwesend" },
      ],
    });

    const outcome = await schoolCheckinStudentsBatch(["7"], "in");

    expect(outcome.succeeded).toBe(1);
    expect(outcome.results[0]?.studentId).toBe("7");
    expect(outcome.results[0]?.location).toBe("Anwesend");
  });

  it("splits selections beyond the per-request cap into sequential chunks and merges the outcomes", async () => {
    const ids = Array.from({ length: 1001 }, (_, i) => `${i + 1}`);
    mockAuthFetch.mockImplementation(
      (_url: string, options: { body: { student_ids: string[] } }) =>
        Promise.resolve({
          data: {
            action: "in",
            succeeded: options.body.student_ids.length,
            failed: 0,
            results: options.body.student_ids.map((studentId) => ({
              student_id: studentId,
              ok: true,
              changed: true,
              location: "Anwesend",
            })),
          },
        }),
    );

    const outcome = await schoolCheckinStudentsBatch(ids, "in");

    expect(mockAuthFetch).toHaveBeenCalledTimes(2);
    const firstBody = mockAuthFetch.mock.calls[0]?.[1]?.body as {
      student_ids: string[];
    };
    const secondBody = mockAuthFetch.mock.calls[1]?.[1]?.body as {
      student_ids: string[];
    };
    expect(firstBody.student_ids).toHaveLength(1000);
    expect(secondBody.student_ids).toEqual(["1001"]);
    expect(outcome.succeeded).toBe(1001);
    expect(outcome.results).toHaveLength(1001);
  });

  it("propagates a first-request error (per-student failures never reject)", async () => {
    mockAuthFetch.mockRejectedValueOnce(
      new Error("API error (403): Forbidden"),
    );

    await expect(schoolCheckinStudentsBatch(["1", "2"], "in")).rejects.toThrow(
      "API error (403): Forbidden",
    );
  });

  it("reports a failing later chunk as per-student request_failed entries instead of throwing", async () => {
    const ids = Array.from({ length: 1002 }, (_, i) => `${i + 1}`);
    mockAuthFetch
      .mockResolvedValueOnce({
        data: {
          action: "out",
          succeeded: 1000,
          failed: 0,
          results: ids.slice(0, 1000).map((studentId) => ({
            student_id: studentId,
            ok: true,
            changed: true,
            location: "Abwesend",
          })),
        },
      })
      .mockRejectedValueOnce(new Error("API error (500): boom"));

    const outcome = await schoolCheckinStudentsBatch(ids, "out");

    // The committed first chunk is reported truthfully; the unprocessed rest
    // comes back as failed so the caller's retry flow covers it.
    expect(outcome.succeeded).toBe(1000);
    expect(outcome.failed).toBe(2);
    expect(outcome.results).toHaveLength(1002);
    expect(outcome.results.at(-1)).toEqual({
      studentId: "1002",
      ok: false,
      changed: false,
      error: "request_failed",
    });
  });

  it("passes an undefined token when no session exists", async () => {
    mockGetCachedSession.mockResolvedValueOnce(null);
    mockAuthFetch.mockResolvedValueOnce({
      data: { action: "in", succeeded: 0, failed: 0, results: [] },
    });

    await schoolCheckinStudentsBatch(["1"], "in");

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/api/students/school-checkin/batch",
      expect.objectContaining({ token: undefined }),
    );
  });
});
