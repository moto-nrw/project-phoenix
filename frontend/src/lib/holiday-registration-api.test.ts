import { afterEach, describe, expect, it, vi } from "vitest";
import {
  fetchHolidayRegistrationFamily,
  quoteHolidayContribution,
} from "./holiday-registration-api";

function answer(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("fetchHolidayRegistrationFamily (#4662)", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("reads the family of the parent session for the link", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      answer(200, {
        data: {
          guardian: {
            first_name: "Sabine",
            last_name: "Schneider",
            email: "s@example.org",
            phone: "0151",
            address: {
              street: "Lindenweg 4",
              postal_code: "49143",
              city: "Bissendorf",
            },
          },
          children: [
            {
              student_id: "41",
              school_id: "31",
              first_name: "Felix",
              last_name: "Schneider",
              birth_date: "2018-03-01",
              contacts: [
                {
                  guardian_profile_id: "51",
                  first_name: "Erika",
                  last_name: "Brandt",
                  phone: "0170 1",
                  relationship_type: "relative",
                  full_guardian: false,
                  emergency: true,
                  pickup: true,
                },
              ],
            },
            {
              student_id: "42",
              school_id: "31",
              first_name: "Mia",
              last_name: "Schneider",
            },
          ],
        },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const family = await fetchHolidayRegistrationFamily("a/b");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/parent/holiday-registrations/a%2Fb/family",
      { cache: "no-store" },
    );
    expect(family).toEqual({
      guardian: {
        firstName: "Sabine",
        lastName: "Schneider",
        email: "s@example.org",
        phone: "0151",
        address: {
          street: "Lindenweg 4",
          postalCode: "49143",
          city: "Bissendorf",
        },
      },
      children: [
        {
          studentId: "41",
          schoolId: "31",
          firstName: "Felix",
          lastName: "Schneider",
          birthDate: "2018-03-01",
          holidayWish: "",
          holidayWishProof: "",
          contacts: [
            {
              guardianProfileId: "51",
              firstName: "Erika",
              lastName: "Brandt",
              email: "",
              phone: "0170 1",
              relationshipType: "relative",
              fullGuardian: false,
              emergency: true,
              pickup: true,
            },
          ],
        },
        {
          studentId: "42",
          schoolId: "31",
          firstName: "Mia",
          lastName: "Schneider",
          birthDate: "",
          holidayWish: "",
          holidayWishProof: "",
          contacts: [],
        },
      ],
      accountProof: "",
    });
  });

  it("hands the account's proof on for the registration (#6673)", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        answer(200, {
          data: {
            guardian: {
              first_name: "Sabine",
              last_name: "Schneider",
              email: "s@example.org",
              phone: "0151",
            },
            children: [],
            account_proof: "sealed-proof",
          },
        }),
      ),
    );
    const family = await fetchHolidayRegistrationFamily("token");
    expect(family?.accountProof).toBe("sealed-proof");
  });

  it("starts empty without a parent session", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(answer(401, {})));
    await expect(fetchHolidayRegistrationFamily("token")).resolves.toBeNull();
  });

  it("names the refusal of an unknown link", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        answer(404, {
          status: 404,
          code: "enrollment.holiday_registration_not_found",
          detail: "holiday registration link not found",
        }),
      ),
    );
    await expect(fetchHolidayRegistrationFamily("token")).rejects.toMatchObject(
      { status: 404, code: "enrollment.holiday_registration_not_found" },
    );
  });
});

describe("quoteHolidayContribution (#5664)", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("sends the order of the children by age, never a date of birth", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: {
            children: [],
            total_cents: 0,
            exemption_available: false,
            benefits_exemption: false,
          },
        }),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await quoteHolidayContribution(
      "abc",
      [[{ week_id: 1, time_model_id: 2 }], []],
      false,
      [2, 1],
    );

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(String(init.body)) as unknown;
    expect(body).toEqual({
      children: [
        { bookings: [{ week_id: 1, time_model_id: 2 }], age_rank: 2 },
        { bookings: [], age_rank: 1 },
      ],
      benefits_declared: false,
    });
  });
});
