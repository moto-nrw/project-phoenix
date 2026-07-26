import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    info: vi.fn(),
    error: vi.fn(),
    warn: vi.fn(),
  }),
}));

import { closingDayService } from "./closing-day-api";
import { formatClosingDayRange } from "./closing-day-helpers";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

const backendDay = {
  id: 7,
  start_date: "2026-12-24",
  end_date: "2026-12-31",
  reason: "Weihnachtsschließung",
  created_at: "2026-07-01T08:00:00Z",
  updated_at: "2026-07-01T08:00:00Z",
};

describe("closingDayService", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("lists and maps closing days", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: [backendDay] }));

    await expect(closingDayService.list()).resolves.toEqual([
      {
        id: "7",
        startDate: "2026-12-24",
        endDate: "2026-12-31",
        reason: "Weihnachtsschließung",
      },
    ]);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/closing-days",
      expect.objectContaining({ method: "GET", credentials: "include" }),
    );
  });

  it("lists to an empty array when the backend returns null data", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: null }));

    await expect(closingDayService.list()).resolves.toEqual([]);
  });

  it("creates and updates a closing day with the expected body", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: backendDay }))
      .mockResolvedValueOnce(jsonResponse({ data: backendDay }));

    const body = {
      start_date: "2026-12-24",
      end_date: "2026-12-31",
      reason: "Weihnachtsschließung",
    };

    await expect(closingDayService.create(body)).resolves.toMatchObject({
      id: "7",
    });
    await expect(closingDayService.update("7", body)).resolves.toMatchObject({
      id: "7",
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/closing-days",
      expect.objectContaining({ method: "POST", body: JSON.stringify(body) }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/closing-days/7",
      expect.objectContaining({ method: "PUT", body: JSON.stringify(body) }),
    );
  });

  it("deletes closing days and accepts 204 without JSON", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(closingDayService.delete("7")).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/closing-days/7",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("surfaces backend error messages and generic non-JSON failures", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ error: "Zeitraum überlappt" }, { status: 409 }),
    );
    await expect(closingDayService.list()).rejects.toMatchObject({
      message: "Zeitraum überlappt",
      httpStatus: 409,
    });

    fetchMock.mockResolvedValueOnce(
      new Response("bad gateway", {
        status: 502,
        headers: { "Content-Type": "text/plain" },
      }),
    );
    await expect(closingDayService.delete("7")).rejects.toMatchObject({
      message: "Anfrage fehlgeschlagen (HTTP 502)",
      httpStatus: 502,
    });

    fetchMock.mockResolvedValueOnce(
      jsonResponse({ error: "Grund fehlt" }, { status: 400 }),
    );
    await expect(closingDayService.delete("7")).rejects.toMatchObject({
      message: "Grund fehlt",
      httpStatus: 400,
    });
  });
});

describe("formatClosingDayRange", () => {
  it("collapses single-day ranges and joins multi-day ranges", () => {
    expect(
      formatClosingDayRange({
        id: "1",
        startDate: "2026-05-15",
        endDate: "2026-05-15",
        reason: "Pädagogischer Tag",
      }),
    ).toBe("15.05.2026");
    expect(
      formatClosingDayRange({
        id: "2",
        startDate: "2026-12-24",
        endDate: "2026-12-31",
        reason: "Weihnachtsschließung",
      }),
    ).toBe("24.12.2026 – 31.12.2026");
  });
});
