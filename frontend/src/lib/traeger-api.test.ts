import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api-error";
import {
  acceptTraegerInvitation,
  createTraegerBankAccount,
  fetchTraegerMasterData,
  uploadTraegerLogo,
  fetchHolidayPeriods,
  fetchTraegerBuero,
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

  it("maps the second factor the login asks for", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(200, {
        status: "mfa_required",
        challenge_token: "challenge",
        masked_email: "a***@b.de",
        organization_id: 7,
      }),
    );
    await expect(
      traegerLogin({ email: "a@b.de", password: "pw" }),
    ).resolves.toEqual({
      status: "mfa_required",
      challengeToken: "challenge",
      maskedEmail: "a***@b.de",
      organizationId: "7",
    });

    fetchMock.mockResolvedValueOnce(
      jsonResponse(200, {
        status: "mfa_enrollment_required",
        access_token: "enrollment",
        masked_email: "a***@b.de",
        mfa_enrollment_required: true,
        organization_id: 7,
      }),
    );
    await expect(
      traegerLogin({ email: "a@b.de", password: "pw" }),
    ).resolves.toEqual({
      status: "mfa_enrollment_required",
      enrollmentToken: "enrollment",
      maskedEmail: "a***@b.de",
      organizationId: "7",
    });
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

  it("maps the Träger-Büro and keeps only known roles (#4650)", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: {
          members: [
            { account_id: 3, email: "a@b.de", roles: ["billing", "unknown"] },
          ],
          invitations: [
            {
              id: 9,
              email: "c@d.de",
              roles: ["read_only"],
              expires_at: "2026-09-11T10:00:00Z",
            },
          ],
        },
      }),
    );

    await expect(fetchTraegerBuero()).resolves.toEqual({
      members: [{ accountId: "3", email: "a@b.de", roles: ["billing"] }],
      invitations: [
        {
          id: "9",
          email: "c@d.de",
          roles: ["read_only"],
          expiresAt: "2026-09-11T10:00:00Z",
        },
      ],
    });
    expect(fetchMock.mock.calls[0]![0]).toBe("/api/traeger/accounts");
  });

  it("accepts an invitation with the password and returns the address", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, { status: "success", data: { email: "a@b.de" } }),
    );

    await expect(
      acceptTraegerInvitation("tok/en", { password: "pw" }),
    ).resolves.toBe("a@b.de");
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/traeger/invitations/tok%2Fen/accept");
    expect(JSON.parse(init!.body as string)).toEqual({
      password: "pw",
      confirm_password: "",
    });
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

  it("maps the master data with string IDs (#4653)", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        status: "success",
        data: {
          contact: { name: "Petra Lindner", phone: "", email: "" },
          logo: {
            content_type: "image/png",
            width: 240,
            height: 80,
            size_bytes: 512,
            updated_at: "2026-09-09T10:00:00Z",
          },
          bank_accounts: [
            {
              id: 7,
              label: "Elternbeiträge",
              account_holder: "Träger e.V.",
              iban: "DE89370400440532013000",
              bic: "",
              bank_name: "",
              is_default: true,
            },
          ],
        },
      }),
    );

    const data = await fetchTraegerMasterData();

    expect(data.contact.name).toBe("Petra Lindner");
    expect(data.logo).toEqual({
      contentType: "image/png",
      width: 240,
      height: 80,
      sizeBytes: 512,
      updatedAt: "2026-09-09T10:00:00Z",
    });
    expect(data.bankAccounts).toEqual([
      {
        id: "7",
        label: "Elternbeiträge",
        accountHolder: "Träger e.V.",
        iban: "DE89370400440532013000",
        bic: "",
        bankName: "",
        isDefault: true,
      },
    ]);
  });

  it("sends a bank account in the backend's field names", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(201, {
        status: "success",
        data: {
          id: 9,
          label: "Mensa",
          account_holder: "Träger e.V.",
          iban: "GB82WEST12345698765432",
          bic: "",
          bank_name: "",
          is_default: false,
        },
      }),
    );

    const created = await createTraegerBankAccount({
      label: "Mensa",
      accountHolder: "Träger e.V.",
      iban: "GB82 WEST 1234 5698 7654 32",
      bic: "",
      bankName: "",
      isDefault: false,
    });

    expect(created.id).toBe("9");
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/traeger/master-data/bank-accounts");
    expect(JSON.parse(init?.body as string)).toEqual({
      label: "Mensa",
      account_holder: "Träger e.V.",
      iban: "GB82 WEST 1234 5698 7654 32",
      bic: "",
      bank_name: "",
      is_default: false,
    });
  });

  it("uploads the logo as the multipart field logo and keeps a refusal's code", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(400, {
        status: 400,
        code: "traeger.logo_invalid",
        detail: "invalid",
      }),
    );
    const file = new File(["x"], "logo.png", { type: "image/png" });

    await expect(uploadTraegerLogo(file)).rejects.toMatchObject({
      code: "traeger.logo_invalid",
    });
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/traeger/master-data/logo");
    expect(init?.method).toBe("PUT");
    const body = init?.body;
    expect(body).toBeInstanceOf(FormData);
    expect((body as FormData).get("logo")).toBe(file);
  });
});
