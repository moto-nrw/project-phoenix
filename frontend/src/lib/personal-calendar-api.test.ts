import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  createStaffAppointment,
  getCalendarRecipientOptions,
  getParentCalendar,
  getStaffCalendar,
  respondParentCalendar,
  respondStaffCalendar,
} from "./personal-calendar-api";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

describe("personal calendar API", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("loads staff and parent calendars with local calendar dates", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: { from: "2026-01-05", to: "2026-01-11", events: [] },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: { from: "2026-01-05", to: "2026-01-11", events: [] },
        }),
      );

    await getStaffCalendar(new Date(2026, 0, 5), new Date(2026, 0, 11));
    await getParentCalendar(new Date(2026, 0, 5), new Date(2026, 0, 11));

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/calendar/my?from=2026-01-05&to=2026-01-11",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/parent/calendar?from=2026-01-05&to=2026-01-11",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("posts staff and parent RSVP responses with numeric recipient ids", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: { status: "accepted" } }))
      .mockResolvedValueOnce(jsonResponse({ data: { status: "declined" } }));

    await respondStaffCalendar(42, "accepted");
    await respondParentCalendar(77, "declined");

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/calendar/recipients/42/response",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ status: "accepted" }),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/parent/calendar/recipients/77/response",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ status: "declined" }),
      }),
    );
  });

  it("creates appointments and recipient option queries", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: { appointment: { id: 1 } } }))
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            staff: [],
            parents: [],
            groups: [],
            classes: ["1a"],
            students: [],
          },
        }),
      );

    await createStaffAppointment({
      title: "Planning",
      start_date: "2026-01-05",
      end_date: "2026-01-05",
      start_time: "09:00",
      end_time: "10:00",
      all_day: false,
      delivery_mode: "rsvp_required",
      recurrence: {
        frequency: "weekly",
        interval_count: 1,
        weekdays: ["monday"],
      },
      targets: [{ type: "staff", id: 7 }],
    });
    await getCalendarRecipientOptions("  anna  ");

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/calendar/appointments",
      expect.objectContaining({
        method: "POST",
        body: expect.stringContaining('"title":"Planning"'),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/calendar/recipient-options?q=anna&limit=30",
      expect.any(Object),
    );
  });

  it("surfaces backend error messages", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ error: "calendar access forbidden" }, { status: 403 }),
    );

    await expect(
      getStaffCalendar(new Date(2026, 0, 5), new Date(2026, 0, 11)),
    ).rejects.toThrow("calendar access forbidden");
  });
});
