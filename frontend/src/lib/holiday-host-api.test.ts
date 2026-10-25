import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api-error";
import {
  downloadHolidayParticipants,
  fetchHolidayParticipants,
  fetchHostedHolidayWeeks,
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
      },
    ]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/holiday-weeks/");
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
    });
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
});
