import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  createStaffAppointment,
  getCalendarRecipientOptions,
  getParentAppointmentOverview,
  getParentCalendar,
  getStaffAppointmentOverview,
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

  it("loads staff and parent attendee overviews with encoded appointment ids", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            appointment_id: 12,
            delivery_mode: "rsvp_required",
            overview_visibility: "all",
            attendees: [
              {
                recipient_id: 1,
                recipient_type: "staff",
                name: "Ada",
                status: "accepted",
              },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            appointment_id: 13,
            delivery_mode: "informational",
            overview_visibility: "all",
            attendees: [],
          },
        }),
      );

    await expect(getStaffAppointmentOverview("12/next")).resolves.toMatchObject(
      {
        appointment_id: 12,
        attendees: [{ name: "Ada", status: "accepted" }],
      },
    );
    await expect(getParentAppointmentOverview(13)).resolves.toMatchObject({
      appointment_id: 13,
      attendees: [],
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/calendar/appointments/12%2Fnext/overview",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/parent/calendar/appointments/13/overview",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("returns unwrapped plain responses and 204 responses", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          staff: [],
          parents: [],
          groups: [],
          classes: [],
          students: [],
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(getCalendarRecipientOptions("")).resolves.toEqual({
      staff: [],
      parents: [],
      groups: [],
      classes: [],
      students: [],
    });
    await expect(respondStaffCalendar(5, "accepted")).resolves.toBeUndefined();

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/calendar/recipient-options?limit=30",
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

  it("uses generic error messages when error responses are not json", async () => {
    fetchMock.mockResolvedValueOnce(new Response("nope", { status: 500 }));

    await expect(getStaffAppointmentOverview(9)).rejects.toThrow(
      "Anfrage fehlgeschlagen (HTTP 500)",
    );
  });
});
