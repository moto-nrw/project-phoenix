import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api-error";
import {
  assignHolidayGuestGroup,
  downloadHolidayParticipants,
  fetchHolidayGuestSheet,
  fetchHolidayStudentHealth,
  fetchHolidayParticipants,
  fetchHostedHolidayWeeks,
  formatHolidayHostAnswer,
} from "./holiday-host-api";

const { downloadBlobMock } = vi.hoisted(() => ({ downloadBlobMock: vi.fn() }));

vi.mock("~/lib/file-download", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/file-download")>()),
  downloadBlob: downloadBlobMock,
}));

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const BACKEND_WEEK = {
  id: 51,
  period_name: "Sommerferien 2026",
  starts_on: "2026-09-14",
  ends_on: "2026-09-18",
  participants: 1,
};

describe("holiday host api", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    fetchMock.mockReset();
    downloadBlobMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("maps the hosted weeks with string IDs", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, { status: "success", data: [BACKEND_WEEK] }),
    );

    await expect(fetchHostedHolidayWeeks()).resolves.toEqual([
      {
        id: "51",
        periodName: "Sommerferien 2026",
        startsOn: "2026-09-14",
        endsOn: "2026-09-18",
        participants: 1,
        guestGroupId: null,
        guests: 0,
      },
    ]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/holiday-weeks");
  });

  it("maps a participant list, keeping holiday guests without a school", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: {
          week: BACKEND_WEEK,
          time_models: [
            {
              id: 61,
              name: "bis 13 Uhr",
              starts_at: "08:00",
              ends_at: "13:00",
              count: 1,
            },
          ],
          home_schools: [{ school_id: null, name: "Feriengäste", count: 1 }],
          participants: [
            {
              child_id: 9,
              first_name: "Ben",
              last_name: "Muster",
              birth_date: "2016-09-12",
              home_school_id: null,
              home_school_name: "Feriengast (Grundschule Melle)",
              time_model_id: 61,
            },
          ],
        },
      }),
    );

    const list = await fetchHolidayParticipants("51");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/holiday-weeks/51/participants",
    );
    expect(list.homeSchools).toEqual([
      { schoolId: null, name: "Feriengäste", count: 1 },
    ]);
    expect(list.participants[0]).toEqual({
      childId: "9",
      firstName: "Ben",
      lastName: "Muster",
      birthDate: "2016-09-12",
      homeSchoolId: null,
      homeSchoolName: "Feriengast (Grundschule Melle)",
      timeModelId: "61",
      secondGuardian: null,
      answers: [],
      health: null,
    });
    expect(list.questions).toEqual([]);
  });

  it("maps the shared answers and the second guardian of a participant (#5202)", async () => {
    const answers = [
      {
        key: "darf_allein_nach_hause",
        label: "Darf allein nach Hause gehen",
        type: "boolean",
        value: true,
      },
      { key: "ausflug_ab", label: "Ausflug ab", type: "date", value: null },
    ];
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: {
          week: BACKEND_WEEK,
          time_models: [],
          home_schools: [],
          questions: answers.map(({ key, label, type }) => ({
            key,
            label,
            type,
          })),
          participants: [
            {
              child_id: 9,
              first_name: "Ida",
              last_name: "Sommer",
              birth_date: "2018-03-01",
              home_school_id: 31,
              home_school_name: "OGS Süd",
              time_model_id: 61,
              second_guardian: { name: "Tom Zweit", phone: "0170 2222222" },
              answers,
            },
          ],
        },
      }),
    );

    const list = await fetchHolidayParticipants("51");

    expect(list.questions.map((question) => question.key)).toEqual([
      "darf_allein_nach_hause",
      "ausflug_ab",
    ]);
    expect(list.participants[0]?.secondGuardian).toEqual({
      name: "Tom Zweit",
      phone: "0170 2222222",
    });
    expect(list.participants[0]?.answers).toEqual(answers);
  });

  it("writes a shared answer as the school reads it", () => {
    expect(formatHolidayHostAnswer({ type: "boolean", value: true })).toBe(
      "Ja",
    );
    expect(formatHolidayHostAnswer({ type: "boolean", value: false })).toBe(
      "Nein",
    );
    expect(formatHolidayHostAnswer({ type: "date", value: "2026-09-15" })).toBe(
      "15.09.2026",
    );
    expect(
      formatHolidayHostAnswer({ type: "select", value: "Mit dem Bus" }),
    ).toBe("Mit dem Bus");
    expect(formatHolidayHostAnswer({ type: "text", value: null })).toBe("–");
    expect(formatHolidayHostAnswer(undefined)).toBe("–");
  });

  it("keeps the error code of an ended week", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(404, {
        status: 404,
        detail: "hosted holiday week not found",
        code: "enrollment.hosted_holiday_week_not_found",
      }),
    );

    const error = await fetchHolidayParticipants("51").catch(
      (caught: unknown) => caught,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(404);
    expect((error as ApiError).code).toBe(
      "enrollment.hosted_holiday_week_not_found",
    );
  });

  it("downloads the export under the backend's file name", async () => {
    fetchMock.mockResolvedValue(
      new Response("Nachname;Vorname", {
        status: 200,
        headers: {
          "Content-Type": "text/csv",
          "Content-Disposition":
            'attachment; filename="teilnehmerliste-2026-09-14.csv"',
        },
      }),
    );

    await downloadHolidayParticipants("51", "csv");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/holiday-weeks/51/participants/export/csv",
    );
    expect(downloadBlobMock).toHaveBeenCalledWith(
      expect.any(Blob),
      "teilnehmerliste-2026-09-14.csv",
    );
  });

  it("maps the group of the holiday guests of a week", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: [{ ...BACKEND_WEEK, guest_group_id: 7, guests: 2 }],
      }),
    );

    const [week] = await fetchHostedHolidayWeeks();

    expect(week?.guestGroupId).toBe("7");
    expect(week?.guests).toBe(2);
  });

  it("takes the holiday guests into a group and returns their number", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: { week_id: 51, group_id: 7, guests: 2 },
      }),
    );

    await expect(assignHolidayGuestGroup("51", "7")).resolves.toBe(2);

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe("/api/holiday-weeks/51/guest-group");
    expect(init?.method).toBe("PUT");
    expect(init?.body).toBe(JSON.stringify({ group_id: "7" }));
  });

  it("keeps the code of a group that is not the school's", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(400, {
        status: 400,
        detail: "holiday guest group is not a group of the school",
        code: "enrollment.holiday_guest_group_invalid",
      }),
    );

    const error = await assignHolidayGuestGroup("51", "8").catch(
      (caught: unknown) => caught,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe(
      "enrollment.holiday_guest_group_invalid",
    );
  });

  it("maps the sheet of a holiday guest with care, emergency and pickup", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: {
          student_id: 91,
          week_id: 51,
          starts_on: "2026-09-14",
          ends_on: "2026-09-18",
          time_model_name: "bis 13 Uhr",
          time_model_starts_at: "08:00",
          time_model_ends_at: "13:00",
          home_school_id: null,
          guest_school_name: "Grundschule Melle",
          emergency_contacts: [
            {
              name: "Eva Muster",
              phone: "0151 2345678",
              relationship: "Eltern",
            },
          ],
          pickup_contacts: null,
          read_only: false,
        },
      }),
    );

    await expect(fetchHolidayGuestSheet("91")).resolves.toEqual({
      studentId: "91",
      startsOn: "2026-09-14",
      endsOn: "2026-09-18",
      timeModelName: "bis 13 Uhr",
      timeModelStartsAt: "08:00",
      timeModelEndsAt: "13:00",
      homeSchoolId: null,
      guestSchoolName: "Grundschule Melle",
      emergencyContacts: [
        { name: "Eva Muster", phone: "0151 2345678", relationship: "Eltern" },
      ],
      pickupContacts: [],
      answers: [],
      health: null,
      readOnly: false,
    });
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/holiday-guests/91");
  });

  it("maps the health information and the own child's health (#4722)", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        data: [
          {
            week_id: 51,
            period_name: "Sommerferien",
            starts_on: "2026-09-14",
            ends_on: "2026-09-18",
            health: {
              allergies: "Erdnüsse",
              medication: "",
              health_notes: "Brille",
              answers: null,
            },
          },
        ],
      }),
    );

    await expect(fetchHolidayStudentHealth("7")).resolves.toEqual([
      {
        weekId: "51",
        periodName: "Sommerferien",
        startsOn: "2026-09-14",
        endsOn: "2026-09-18",
        health: {
          allergies: "Erdnüsse",
          medication: "",
          healthNotes: "Brille",
          answers: [],
        },
      },
    ]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/holiday-weeks/students/7/health",
    );
  });
});
