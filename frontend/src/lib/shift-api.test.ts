import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
}));

import { staffShiftService } from "./shift-api";

describe("staffShiftService errors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("preserves status and backend error detail for save failures", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "shift overlaps existing row" }), {
        status: 409,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(
      staffShiftService.createShift({
        staffId: "7",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "16:00",
        breakMinutes: 30,
      }),
    ).rejects.toMatchObject({
      status: 409,
      detail: "shift overlaps existing row",
    });
  });

  it("preserves status for load failures with text bodies", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("database timeout", { status: 500 }),
    );

    await expect(
      staffShiftService.getShifts("2026-07-06", "2026-07-10"),
    ).rejects.toMatchObject({
      status: 500,
      detail: "database timeout",
    });
  });
});
