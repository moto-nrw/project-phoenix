import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api-error";
import {
  fetchHolidayPeriods,
  fetchTraegerSchools,
  publishHolidayPeriod,
  traegerLogin,
  updateHolidayPeriod,
} from "./traeger-api";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("traeger-api", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("sends the chosen Träger as a number and returns the token pair", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "authenticated",
        access_token: "access",
        refresh_token: "refresh",
      }),
    );

    await expect(
      traegerLogin({ email: "a@b.de", password: "pw", organizationId: "7" }),
    ).resolves.toEqual({
      status: "authenticated",
      accessToken: "access",
      refreshToken: "refresh",
    });
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/traeger/auth/login");
    expect(JSON.parse(init?.body as string)).toEqual({
      email: "a@b.de",
      password: "pw",
      organization_id: 7,
    });
  });

  it("maps the choice of Träger with string IDs", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "organization_selection_required",
        organizations: [{ id: 7, name: "Träger Nord" }],
      }),
    );

    await expect(
      traegerLogin({ email: "a@b.de", password: "pw" }),
    ).resolves.toEqual({
      status: "organization_selection_required",
      organizations: [{ id: "7", name: "Träger Nord" }],
    });
    const [, init] = fetchMock.mock.calls[0]!;
    expect(JSON.parse(init?.body as string)).not.toHaveProperty(
      "organization_id",
    );
  });

  it("throws the backend code of a refused login", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(403, {
        status: 403,
        detail: "no membership",
        code: "traeger.no_membership",
      }),
    );

    const error = await traegerLogin({ email: "a@b.de", password: "pw" }).catch(
      (err: unknown) => err,
    );
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe("traeger.no_membership");
    expect((error as ApiError).status).toBe(403);
  });

  it("maps the schools of the Träger with string IDs", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: [
          {
            id: 2,
            name: "OGS Am Bach",
            slug: "am-bach",
            subdomain: "am-bach",
            city: null,
          },
        ],
      }),
    );

    await expect(fetchTraegerSchools()).resolves.toEqual([
      {
        id: "2",
        name: "OGS Am Bach",
        slug: "am-bach",
        subdomain: "am-bach",
        city: "",
      },
    ]);
    expect(fetchMock.mock.calls[0]![0]).toBe("/api/traeger/schools");
  });
});

describe("traeger-api holiday periods", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const backendPeriod = {
    id: 5,
    name: "Osterferien",
    registration_opens_on: "2027-01-11",
    registration_closes_on: "2027-02-26",
    binding: true,
    notes: "Lunchpaket",
    status: "draft",
    published_at: null,
    weeks: [
      {
        id: 7,
        starts_on: "2027-03-22",
        ends_on: "2027-03-26",
        host_school_id: 31,
        time_models: [
          {
            id: 9,
            name: "bis 13 Uhr",
            starts_at: "08:00",
            ends_at: "13:00",
            price_cents: 4500,
          },
        ],
      },
    ],
  };

  it("maps the backend period to string IDs and camelCase", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, { status: "success", data: [backendPeriod] }),
    );

    const [period] = await fetchHolidayPeriods();

    expect(period).toMatchObject({
      id: "5",
      registrationOpensOn: "2027-01-11",
      weeks: [
        {
          id: "7",
          hostSchoolId: "31",
          timeModels: [{ id: "9", startsAt: "08:00", priceCents: 4500 }],
        },
      ],
    });
  });

  it("sends kept IDs as numbers and leaves them out for new rows", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, { status: "success", data: backendPeriod }),
    );

    await updateHolidayPeriod("5", {
      name: "Osterferien",
      registrationOpensOn: "2027-01-11",
      registrationClosesOn: "2027-02-26",
      binding: false,
      notes: "",
      weeks: [
        {
          id: "7",
          startsOn: "2027-03-22",
          endsOn: "2027-03-26",
          hostSchoolId: "31",
          timeModels: [
            {
              name: "bis 16 Uhr",
              startsAt: "08:00",
              endsAt: "16:00",
              priceCents: 6500,
            },
          ],
        },
      ],
    });

    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/traeger/holiday-periods/5");
    expect(init?.method).toBe("PUT");
    expect(JSON.parse(String(init?.body))).toEqual({
      name: "Osterferien",
      registration_opens_on: "2027-01-11",
      registration_closes_on: "2027-02-26",
      binding: false,
      notes: "",
      weeks: [
        {
          id: 7,
          starts_on: "2027-03-22",
          ends_on: "2027-03-26",
          host_school_id: 31,
          time_models: [
            {
              name: "bis 16 Uhr",
              starts_at: "08:00",
              ends_at: "16:00",
              price_cents: 6500,
            },
          ],
        },
      ],
    });
  });

  it("turns a refusal into an ApiError with its code", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(409, {
        status: 409,
        code: "enrollment.holiday_period_incomplete",
        detail: "x",
      }),
    );

    await expect(publishHolidayPeriod("5")).rejects.toBeInstanceOf(ApiError);
    expect(fetchMock.mock.calls[0]![0]).toBe(
      "/api/traeger/holiday-periods/5/publish",
    );
  });
});
