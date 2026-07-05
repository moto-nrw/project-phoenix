import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
}));

import { ownShiftService, staffShiftService } from "./shift-api";

const backendShift = {
  id: 42,
  staff_id: 7,
  date: "2026-07-06",
  start_time: "08:00",
  end_time: "16:00",
  break_minutes: 30,
};

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

  it("falls back when JSON error bodies are malformed", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("{", {
        status: 400,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(staffShiftService.deleteShift("42")).rejects.toMatchObject({
      status: 400,
      detail: "Schicht konnte nicht gelöscht werden",
    });
  });

  it("falls back when JSON error bodies omit detail", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ message: "wrong shape" }), {
        status: 422,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(
      staffShiftService.updateShift("42", {
        staffId: "7",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "16:00",
        breakMinutes: 30,
      }),
    ).rejects.toMatchObject({
      status: 422,
      detail: "Schicht konnte nicht gespeichert werden",
    });
  });
});

describe("staffShiftService requests", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps nullable list responses to an empty list", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: null }, { status: 200 }),
    );

    await expect(
      staffShiftService.getShifts("2026-07-06", "2026-07-10"),
    ).resolves.toEqual([]);
  });

  it("loads own shifts through the time-tracking endpoint", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: [backendShift] }, { status: 200 }),
    );

    await expect(
      ownShiftService.getOwnShifts("2026-07-06", "2026-07-10"),
    ).resolves.toEqual([
      {
        id: "42",
        staffId: "7",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "16:00",
        breakMinutes: 30,
        notes: "",
      },
    ]);
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/time-tracking/shifts?from=2026-07-06&to=2026-07-10",
    );
  });

  it("serializes create and update payloads for the backend", async () => {
    const payload = {
      staffId: "7",
      date: "2026-07-06",
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
    };
    mockSessionFetch
      .mockResolvedValueOnce(Response.json({ data: backendShift }))
      .mockResolvedValueOnce(Response.json({ data: backendShift }));

    await staffShiftService.createShift(payload);
    await staffShiftService.updateShift("42", payload);

    expect(mockSessionFetch).toHaveBeenNthCalledWith(1, "/api/staff/shifts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        staff_id: 7,
        date: "2026-07-06",
        start_time: "08:00",
        end_time: "16:00",
        break_minutes: 30,
      }),
    });
    expect(mockSessionFetch).toHaveBeenNthCalledWith(
      2,
      "/api/staff/shifts/42",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          staff_id: 7,
          date: "2026-07-06",
          start_time: "08:00",
          end_time: "16:00",
          break_minutes: 30,
        }),
      },
    );
  });

  it("returns cleanly after successful deletes", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(staffShiftService.deleteShift("42")).resolves.toBeUndefined();
    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shifts/42", {
      method: "DELETE",
    });
  });
});
