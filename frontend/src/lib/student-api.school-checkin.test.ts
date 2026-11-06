import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// The route answers through sessionFetch; `mockRoute` stands for the BFF
// route: it receives the request as sent and returns the route's JSON
// answer, which the real bffJson then reads.

const { mockRoute } = vi.hoisted(() => ({ mockRoute: vi.fn() }));

vi.mock("./session-cache", () => ({
  sessionFetch: async (url: string, init: RequestInit) => {
    const answer: unknown = await mockRoute(url, {
      method: init.method,
      body: JSON.parse(String(init.body)),
    });
    return new Response(JSON.stringify(answer), { status: 200 });
  },
}));

import {
  schoolCheckinStudent,
  schoolCheckinStudentsBatch,
} from "./student-api";

describe("schoolCheckinStudent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
    mockRoute.mockResolvedValueOnce(backendPayload);

    const result = await schoolCheckinStudent("42", "in");

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

    // Verify the proxy was called with the expected URL, method and body;
    // the route reads the session cookie, no token travels (#5005).
    expect(mockRoute).toHaveBeenCalledWith("/api/students/42/school-checkin", {
      method: "POST",
      body: { action: "in" },
    });
  });

  it("sends a trimmed early-checkout note only with a checkout (#3324)", async () => {
    mockRoute.mockResolvedValue({
      data: {
        student_id: 42,
        status: "checked_out" as const,
        location: "Abwesend" as const,
        changed: true,
      },
    });

    await schoolCheckinStudent("42", "out", "  Arzttermin ");
    expect(mockRoute).toHaveBeenLastCalledWith(
      "/api/students/42/school-checkin",
      expect.objectContaining({
        body: { action: "out", note: "Arzttermin" },
      }),
    );

    await schoolCheckinStudent("42", "out", "   ");
    expect(mockRoute).toHaveBeenLastCalledWith(
      "/api/students/42/school-checkin",
      expect.objectContaining({ body: { action: "out" } }),
    );

    await schoolCheckinStudent("42", "in", "Arzttermin");
    expect(mockRoute).toHaveBeenLastCalledWith(
      "/api/students/42/school-checkin",
      expect.objectContaining({ body: { action: "in" } }),
    );
  });

  it("propagates errors from the underlying fetch", async () => {
    mockRoute.mockRejectedValueOnce(new Error("503 Service Unavailable"));

    await expect(schoolCheckinStudent("42", "in")).rejects.toThrow(
      "503 Service Unavailable",
    );
  });

  it("sends the correct action on 'out' calls", async () => {
    mockRoute.mockResolvedValueOnce({
      data: {
        student_id: 1,
        status: "checked_out",
        location: "Abwesend",
        changed: false,
      },
    });

    await schoolCheckinStudent("1", "out");

    expect(mockRoute).toHaveBeenCalledWith(
      "/api/students/1/school-checkin",
      expect.objectContaining({ body: { action: "out" } }),
    );
  });
});

describe("schoolCheckinStudentsBatch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("maps a batch response and keeps IDs as strings on the wire", async () => {
    mockRoute.mockResolvedValueOnce({
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
    expect(mockRoute).toHaveBeenCalledWith(
      "/api/students/school-checkin/batch",
      {
        method: "POST",
        body: { action: "out", student_ids: ["42", "43", "99"] },
      },
    );
  });

  it("splits selections beyond the per-request cap into sequential chunks and merges the outcomes", async () => {
    const ids = Array.from({ length: 1001 }, (_, i) => `${i + 1}`);
    mockRoute.mockImplementation(
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

    expect(mockRoute).toHaveBeenCalledTimes(2);
    const firstBody = mockRoute.mock.calls[0]?.[1]?.body as {
      student_ids: string[];
    };
    const secondBody = mockRoute.mock.calls[1]?.[1]?.body as {
      student_ids: string[];
    };
    expect(firstBody.student_ids).toHaveLength(1000);
    expect(secondBody.student_ids).toEqual(["1001"]);
    expect(outcome.succeeded).toBe(1001);
    expect(outcome.results).toHaveLength(1001);
  });

  it("propagates a first-request error (per-student failures never reject)", async () => {
    mockRoute.mockRejectedValueOnce(new Error("API error (403): Forbidden"));

    await expect(schoolCheckinStudentsBatch(["1", "2"], "in")).rejects.toThrow(
      "API error (403): Forbidden",
    );
  });

  it("reports a failing later chunk as per-student request_failed entries instead of throwing", async () => {
    const ids = Array.from({ length: 1002 }, (_, i) => `${i + 1}`);
    mockRoute
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
});
