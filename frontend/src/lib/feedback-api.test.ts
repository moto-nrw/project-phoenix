import { describe, it, expect, vi, beforeEach } from "vitest";
import { fetchStudentFeedback } from "./feedback-api";

const mockSessionFetch = vi.fn();

vi.mock("./session-cache", () => ({
  sessionFetch: (...args: unknown[]) => mockSessionFetch(...args),
}));

describe("fetchStudentFeedback", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches and maps feedback entries correctly", async () => {
    const backendEntries = [
      {
        id: 10,
        value: "positive",
        day: "2026-04-01",
        time: "15:30:23",
        student_id: 1,
        is_mensa_feedback: false,
        created_at: "2026-04-01T15:30:23Z",
        updated_at: "2026-04-01T15:30:23Z",
      },
      {
        id: 11,
        value: "negative",
        day: "2026-04-02",
        time: "14:10:05",
        student_id: 1,
        is_mensa_feedback: true,
        created_at: "2026-04-02T14:10:05Z",
        updated_at: "2026-04-02T14:10:05Z",
      },
    ];

    mockSessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ data: backendEntries }),
    });

    const result = await fetchStudentFeedback("1");

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/feedback/student/1", {
      method: "GET",
      credentials: "include",
    });

    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({
      id: "10",
      timestamp: "2026-04-01T15:30:23",
      feedback_type: "positive",
      is_mensa_feedback: false,
    });
    expect(result[1]).toEqual({
      id: "11",
      timestamp: "2026-04-02T14:10:05",
      feedback_type: "negative",
      is_mensa_feedback: true,
    });
  });

  it("returns empty array on 404", async () => {
    mockSessionFetch.mockResolvedValue({
      ok: false,
      status: 404,
      statusText: "Not Found",
    });

    const result = await fetchStudentFeedback("999");

    expect(result).toEqual([]);
  });

  it("throws feature_disabled on 403", async () => {
    mockSessionFetch.mockResolvedValue({
      ok: false,
      status: 403,
      statusText: "Forbidden",
    });

    await expect(fetchStudentFeedback("1")).rejects.toThrow("feature_disabled");
  });

  it("throws on non-404 error", async () => {
    mockSessionFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
    });

    await expect(fetchStudentFeedback("1")).rejects.toThrow(
      "Failed to fetch feedback: 500 Internal Server Error",
    );
  });

  it("handles empty data array", async () => {
    mockSessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ data: [] }),
    });

    const result = await fetchStudentFeedback("1");
    expect(result).toEqual([]);
  });

  it("handles null data gracefully", async () => {
    mockSessionFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ data: null }),
    });

    const result = await fetchStudentFeedback("1");
    expect(result).toEqual([]);
  });

  it("throws and logs on network error", async () => {
    mockSessionFetch.mockRejectedValue(new Error("Network error"));

    await expect(fetchStudentFeedback("1")).rejects.toThrow("Network error");
  });
});
