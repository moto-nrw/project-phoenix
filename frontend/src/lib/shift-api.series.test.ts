import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
}));

import { ShiftApiError, staffShiftSeriesService } from "./shift-api";

beforeEach(() => {
  mockSessionFetch.mockReset();
});

describe("staffShiftSeriesService", () => {
  it("creates a series with a snake_case backend body", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { data: { series_id: 5, created: 12, skipped_dates: ["2026-09-07"] } },
        { status: 201 },
      ),
    );

    await expect(
      staffShiftSeriesService.createSeries({
        staffId: "7",
        weekdays: [1, 3],
        startTime: "09:00",
        endTime: "12:00",
        breakMinutes: 15,
        shiftTypeId: "4",
        calendarPeriodId: "8",
        weekPattern: 1,
        validFrom: "2026-09-01",
        validUntil: null,
      }),
    ).resolves.toEqual({
      seriesId: "5",
      created: 12,
      skippedDates: ["2026-09-07"],
    });

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shifts/series", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        staff_id: 7,
        weekdays: [1, 3],
        start_time: "09:00",
        end_time: "12:00",
        break_minutes: 15,
        shift_type_id: 4,
        calendar_period_id: 8,
        week_pattern: 1,
        valid_from: "2026-09-01",
        valid_until: null,
      }),
    });
  });

  it("defaults missing skipped_dates to an empty list and null type stays null", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { data: { series_id: 5, created: 3, skipped_dates: null } },
        { status: 201 },
      ),
    );

    await expect(
      staffShiftSeriesService.createSeries({
        staffId: "7",
        weekdays: [1],
        startTime: "09:00",
        endTime: "12:00",
        breakMinutes: 0,
        shiftTypeId: null,
        calendarPeriodId: "8",
        weekPattern: 0,
        validFrom: "2026-09-01",
        validUntil: "2026-10-01",
      }),
    ).resolves.toEqual({ seriesId: "5", created: 3, skippedDates: [] });

    const init = mockSessionFetch.mock.calls[0]?.[1] as RequestInit | undefined;
    const body = JSON.parse((init?.body as string) ?? "{}") as Record<
      string,
      unknown
    >;
    expect(body.shift_type_id).toBeNull();
    expect(body.valid_until).toBe("2026-10-01");
  });

  it("splits a series sending only the edited fields", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { data: { series_id: 9, created: 4, skipped_dates: [] } },
        { status: 200 },
      ),
    );

    await expect(
      staffShiftSeriesService.splitSeries("5", {
        effectiveDate: "2026-10-05",
        startTime: "10:00",
        endTime: "14:00",
        breakMinutes: 30,
        shiftTypeId: null,
      }),
    ).resolves.toEqual({ seriesId: "9", created: 4, skippedDates: [] });

    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/staff/shifts/series/5/split",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          effective_date: "2026-10-05",
          start_time: "10:00",
          end_time: "14:00",
          break_minutes: 30,
          shift_type_id: null,
        }),
      },
    );
  });

  it("sends the edited rule fields when the series editor changes them", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { data: { series_id: 9, created: 7, skipped_dates: [] } },
        { status: 200 },
      ),
    );

    await staffShiftSeriesService.splitSeries("5", {
      effectiveDate: "2026-10-05",
      occurrenceShiftId: "42",
      startTime: "10:00",
      endTime: "14:00",
      breakMinutes: 30,
      shiftTypeId: "4",
      weekdays: [2, 5],
      weekPattern: 2,
      validUntil: "2026-11-02",
    });

    const init = mockSessionFetch.mock.calls[0]?.[1] as RequestInit | undefined;
    expect(JSON.parse((init?.body as string) ?? "{}")).toEqual({
      effective_date: "2026-10-05",
      occurrence_shift_id: "42",
      start_time: "10:00",
      end_time: "14:00",
      break_minutes: 30,
      shift_type_id: 4,
      weekdays: [2, 5],
      week_pattern: 2,
      valid_until: "2026-11-02",
    });
  });

  it("keeps a large occurrence ID lossless in the split payload", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { data: { series_id: 9, created: 7, skipped_dates: [] } },
        { status: 200 },
      ),
    );

    await staffShiftSeriesService.splitSeries("5", {
      effectiveDate: "2026-10-05",
      occurrenceShiftId: "9223372036854775807",
      startTime: "10:00",
      endTime: "14:00",
      breakMinutes: 30,
      shiftTypeId: null,
    });

    const init = mockSessionFetch.mock.calls[0]?.[1] as RequestInit | undefined;
    expect(JSON.parse((init?.body as string) ?? "{}")).toMatchObject({
      occurrence_shift_id: "9223372036854775807",
    });
  });

  it("sends an explicit null valid_until to run the series to the period end", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { data: { series_id: 9, created: 7, skipped_dates: [] } },
        { status: 200 },
      ),
    );

    await staffShiftSeriesService.splitSeries("5", {
      effectiveDate: "2026-10-05",
      startTime: "10:00",
      endTime: "14:00",
      breakMinutes: 0,
      shiftTypeId: null,
      validUntil: null,
    });

    const init = mockSessionFetch.mock.calls[0]?.[1] as RequestInit | undefined;
    const body = JSON.parse((init?.body as string) ?? "{}") as Record<
      string,
      unknown
    >;
    expect(body).toHaveProperty("valid_until", null);
    expect(body).not.toHaveProperty("weekdays");
    expect(body).not.toHaveProperty("week_pattern");
  });

  it("loads the stored rule behind a series shift", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        {
          data: {
            id: 5,
            staff_id: 7,
            weekdays: [1, 3],
            start_time: "09:00",
            end_time: "12:00",
            break_minutes: 15,
            shift_type_id: 4,
            calendar_period_id: 8,
            week_pattern: 1,
            valid_from: "2026-09-01",
            valid_until: "2026-10-01",
          },
        },
        { status: 200 },
      ),
    );

    await expect(staffShiftSeriesService.getSeries("5")).resolves.toEqual({
      id: "5",
      staffId: "7",
      weekdays: [1, 3],
      startTime: "09:00",
      endTime: "12:00",
      breakMinutes: 15,
      shiftTypeId: "4",
      calendarPeriodId: "8",
      weekPattern: 1,
      validFrom: "2026-09-01",
      validUntil: "2026-10-01",
    });

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shifts/series/5");
  });

  it("maps a rule without weekdays, type, or end date onto safe values", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        {
          data: {
            id: 5,
            staff_id: 7,
            weekdays: null,
            start_time: "09:00",
            end_time: "12:00",
            break_minutes: 0,
            shift_type_id: null,
            calendar_period_id: 8,
            week_pattern: 0,
            valid_from: "2026-09-01",
            valid_until: null,
          },
        },
        { status: 200 },
      ),
    );

    await expect(staffShiftSeriesService.getSeries("5")).resolves.toMatchObject(
      {
        weekdays: [],
        shiftTypeId: null,
        validUntil: null,
      },
    );
  });

  it("maps a failed rule load onto ShiftApiError", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ error: "shift series not found" }, { status: 404 }),
    );

    await expect(staffShiftSeriesService.getSeries("5")).rejects.toMatchObject(
      new ShiftApiError(404, "shift series not found"),
    );
  });

  it("ends a series from the given date", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: { series_id: 5, deleted: 6 } }, { status: 200 }),
    );

    await expect(
      staffShiftSeriesService.endSeries("5", "2026-10-05"),
    ).resolves.toBeUndefined();

    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/staff/shifts/series/5?from=2026-10-05",
      { method: "DELETE" },
    );
  });

  it("maps failures onto ShiftApiError", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ error: "invalid shift series" }, { status: 400 }),
    );
    await expect(
      staffShiftSeriesService.splitSeries("5", {
        effectiveDate: "2026-10-05",
        startTime: "10:00",
        endTime: "14:00",
        breakMinutes: 0,
        shiftTypeId: null,
      }),
    ).rejects.toMatchObject(new ShiftApiError(400, "invalid shift series"));

    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ error: "shift series not found" }, { status: 404 }),
    );
    await expect(
      staffShiftSeriesService.endSeries("5", "2026-10-05"),
    ).rejects.toMatchObject(new ShiftApiError(404, "shift series not found"));
  });
});
