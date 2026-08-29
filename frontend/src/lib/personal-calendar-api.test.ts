import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  cancelStaffAppointment,
  cancelStaffAppointmentOccurrence,
  createStaffAppointment,
  deleteStaffAppointment,
  getCalendarRecipientOptions,
  getParentAppointmentOverview,
  getParentCalendar,
  getParentCalendarFeed,
  getStaffAppointmentDetail,
  getStaffAppointmentOverview,
  getStaffCalendar,
  getStaffCalendarFeed,
  respondParentCalendar,
  respondStaffCalendar,
  rotateParentCalendarFeed,
  rotateStaffCalendarFeed,
  updateStaffAppointment,
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

  it("posts staff and parent RSVP responses with string recipient ids", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: { status: "accepted" } }))
      .mockResolvedValueOnce(jsonResponse({ data: { status: "declined" } }));

    await respondStaffCalendar("42", "accepted");
    await respondParentCalendar("77", "declined");

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
      .mockResolvedValueOnce(
        jsonResponse({ data: { appointment: { id: "1" } } }),
      )
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
      targets: [{ type: "staff", id: "7" }],
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
            appointment_id: "12",
            delivery_mode: "rsvp_required",
            overview_visibility: "all",
            attendees: [
              {
                recipient_id: "1",
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
            appointment_id: "13",
            delivery_mode: "informational",
            overview_visibility: "all",
            attendees: [],
          },
        }),
      );

    await expect(getStaffAppointmentOverview("12/next")).resolves.toMatchObject(
      {
        appointment_id: "12",
        attendees: [{ name: "Ada", status: "accepted" }],
      },
    );
    await expect(getParentAppointmentOverview("13")).resolves.toMatchObject({
      appointment_id: "13",
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
    await expect(
      respondStaffCalendar("5", "accepted"),
    ).resolves.toBeUndefined();

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/calendar/recipient-options?limit=30",
      expect.any(Object),
    );
  });

  it("edits, cancels, deletes and reads appointment detail", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: { appointment: {} } }))
      .mockResolvedValueOnce(jsonResponse({ data: { appointment: {} } }))
      .mockResolvedValueOnce(jsonResponse({ data: { status: "cancelled" } }))
      .mockResolvedValueOnce(jsonResponse({ data: { status: "deleted" } }))
      .mockResolvedValueOnce(jsonResponse({ data: { status: "cancelled" } }));

    await getStaffAppointmentDetail("15");
    await updateStaffAppointment("15", {
      title: "New",
      start_date: "2026-01-06",
      end_date: "2026-01-06",
      start_time: "10:00",
      end_time: "11:00",
      all_day: false,
    });
    await cancelStaffAppointment("15");
    await deleteStaffAppointment("15");
    await cancelStaffAppointmentOccurrence("15", "2026-01-12");

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/calendar/appointments/15",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/calendar/appointments/15",
      expect.objectContaining({
        method: "PUT",
        body: expect.stringContaining('"title":"New"'),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/calendar/appointments/15/cancel",
      expect.objectContaining({ method: "POST" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      4,
      "/api/calendar/appointments/15",
      expect.objectContaining({ method: "DELETE" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      5,
      "/api/calendar/appointments/15/occurrences/2026-01-12/cancel",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("reads and rotates the parent calendar subscription feed", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            url: "https://parents.test/api/calendar-feed/abc",
            webcal_url: "webcal://parents.test/api/calendar-feed/abc",
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            url: "https://parents.test/api/calendar-feed/new",
            webcal_url: "webcal://parents.test/api/calendar-feed/new",
          },
        }),
      );

    await expect(getParentCalendarFeed()).resolves.toMatchObject({
      webcal_url: "webcal://parents.test/api/calendar-feed/abc",
    });
    await expect(rotateParentCalendarFeed()).resolves.toMatchObject({
      url: "https://parents.test/api/calendar-feed/new",
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/parent/calendar/feed",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/parent/calendar/feed/rotate",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("reads and rotates the staff calendar subscription feed", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            url: "https://moto.test/api/calendar-feed/abc",
            webcal_url: "webcal://moto.test/api/calendar-feed/abc",
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            url: "https://moto.test/api/calendar-feed/new",
            webcal_url: "webcal://moto.test/api/calendar-feed/new",
          },
        }),
      );

    await expect(getStaffCalendarFeed()).resolves.toMatchObject({
      webcal_url: "webcal://moto.test/api/calendar-feed/abc",
    });
    await expect(rotateStaffCalendarFeed()).resolves.toMatchObject({
      url: "https://moto.test/api/calendar-feed/new",
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/calendar/feed",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/calendar/feed/rotate",
      expect.objectContaining({ method: "POST" }),
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

    await expect(getStaffAppointmentOverview("9")).rejects.toThrow(
      "Anfrage fehlgeschlagen (HTTP 500)",
    );
  });
});
