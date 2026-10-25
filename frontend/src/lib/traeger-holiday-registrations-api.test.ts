import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api-error";
import {
  changeHolidayRegistration,
  downloadHolidayRegistrations,
  EMPTY_HOLIDAY_REGISTRATION_FILTER,
  fetchHolidayRegistration,
  fetchHolidayRegistrations,
  rebookHolidayChild,
} from "./traeger-holiday-registrations-api";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const BACKEND_DETAIL = {
  id: 7,
  status: "confirmed",
  frozen: true,
  submitted_at: "2026-11-03T08:00:00Z",
  guardian_first_name: "Eva",
  guardian_last_name: "Muster",
  guardian_email: "eva@example.org",
  guardian_phone: "0151 2345678",
  children: [
    {
      id: 9,
      first_name: "Ben",
      last_name: "Muster",
      birth_date: "2016-09-12",
      home_school_id: null,
      guest_school_name: "Grundschule Melle",
      bookings: [
        {
          week_id: 51,
          week_starts_on: "2027-01-04",
          week_ends_on: "2027-01-08",
          host_school_id: 31,
          time_model_id: 61,
          time_model_name: "bis 13 Uhr",
        },
      ],
      duplicate_registration_ids: [8],
    },
  ],
  health: [
    { child_id: 9, allergies: "Erdnüsse", medication: "", health_notes: "" },
  ],
  contacts: [
    {
      kind: "emergency",
      name: "Oma",
      phone: "05402",
      relationship: "Großmutter",
    },
  ],
  privacy_consent_at: "2026-11-03T08:00:00Z",
  health_consent_at: null,
  binding_confirmed_at: "2026-11-03T08:00:00Z",
  history: [
    {
      action: "rebook",
      occurred_at: "2026-11-04T09:00:00Z",
      details: {
        child_id: 9,
        before: [{ week_id: 52, time_model_id: 63 }],
        after: [{ week_id: 51, time_model_id: 61 }],
      },
    },
    {
      action: "confirm",
      occurred_at: "2026-11-05T09:00:00Z",
      details: { from: "received" },
    },
  ],
};

describe("traeger-holiday-registrations-api", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("sends only the set filters and maps the list with string IDs", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: {
          registrations: [BACKEND_DETAIL],
          total: 3,
          status_counts: { confirmed: 1, received: 2 },
        },
      }),
    );

    const list = await fetchHolidayRegistrations("5", {
      ...EMPTY_HOLIDAY_REGISTRATION_FILTER,
      school: "guest",
      timeModel: "bis 13 Uhr",
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/traeger/holiday-periods/5/registrations?school=guest&time_model=bis+13+Uhr",
    );
    expect(list.total).toBe(3);
    expect(list.statusCounts).toEqual({
      received: 2,
      confirmed: 1,
      cancelled: 0,
    });
    const child = list.registrations[0]?.children[0];
    expect(child?.homeSchoolId).toBeNull();
    expect(child?.duplicateRegistrationIds).toEqual(["8"]);
    expect(child?.bookings[0]).toMatchObject({
      weekId: "51",
      hostSchoolId: "31",
      timeModelId: "61",
    });
  });

  it("maps the detail with health information, contacts and history", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, { status: "success", data: BACKEND_DETAIL }),
    );

    const detail = await fetchHolidayRegistration("5", "7");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/traeger/holiday-periods/5/registrations/7",
    );
    expect(detail.health).toEqual([
      { childId: "9", allergies: "Erdnüsse", medication: "", healthNotes: "" },
    ]);
    expect(detail.history).toEqual([
      {
        action: "rebook",
        occurredAt: "2026-11-04T09:00:00Z",
        from: "",
        childId: "9",
        before: [{ weekId: "52", timeModelId: "63" }],
        after: [{ weekId: "51", timeModelId: "61" }],
      },
      {
        action: "confirm",
        occurredAt: "2026-11-05T09:00:00Z",
        from: "received",
        childId: "",
        before: [],
        after: [],
      },
    ]);
  });

  it("posts an action and puts the bookings of a rebooking", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(
        jsonResponse(200, { status: "success", data: BACKEND_DETAIL }),
      ),
    );

    await changeHolidayRegistration("5", "7", "reopen");
    await rebookHolidayChild("5", "7", "9", [
      { weekId: "51", timeModelId: "61" },
    ]);

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/traeger/holiday-periods/5/registrations/7/reopen",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "/api/traeger/holiday-periods/5/registrations/7/children/9/bookings",
    );
    expect(fetchMock.mock.calls[1]?.[1]?.method).toBe("PUT");
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toEqual({
      bookings: [{ week_id: 51, time_model_id: 61 }],
    });
  });

  it("turns a refused change into an ApiError with its code", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(409, {
        status: 409,
        detail: "the holiday registration is frozen",
        code: "enrollment.holiday_registration_frozen",
      }),
    );

    await expect(
      changeHolidayRegistration("5", "7", "cancel"),
    ).rejects.toBeInstanceOf(ApiError);
  });

  it("downloads the export of the filter with the backend's file name", async () => {
    fetchMock.mockResolvedValue(
      new Response("Status;Eingefroren\n", {
        status: 200,
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "Content-Disposition":
            'attachment; filename="ferienanmeldungen-winterferien.csv"',
        },
      }),
    );

    const file = await downloadHolidayRegistrations(
      "5",
      { ...EMPTY_HOLIDAY_REGISTRATION_FILTER, status: "confirmed" },
      "csv",
    );

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/traeger/holiday-periods/5/registrations/export?status=confirmed&format=csv",
    );
    expect(file.filename).toBe("ferienanmeldungen-winterferien.csv");
    expect(await file.blob.text()).toBe("Status;Eingefroren\n");
  });

  it("keeps the code of a refused export", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(403, {
        status: 403,
        detail: "forbidden",
        code: "general.forbidden",
      }),
    );

    await expect(
      downloadHolidayRegistrations(
        "5",
        EMPTY_HOLIDAY_REGISTRATION_FILTER,
        "xlsx",
      ),
    ).rejects.toMatchObject({ status: 403 });
  });
});
