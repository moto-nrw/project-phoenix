import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchHolidayRegistrationFamily } from "./holiday-registration-api";

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
          },
          children: [
            {
              student_id: "41",
              school_id: "31",
              first_name: "Felix",
              last_name: "Schneider",
              birth_date: "2018-03-01",
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
      },
      children: [
        {
          studentId: "41",
          schoolId: "31",
          firstName: "Felix",
          lastName: "Schneider",
          birthDate: "2018-03-01",
        },
        {
          studentId: "42",
          schoolId: "31",
          firstName: "Mia",
          lastName: "Schneider",
          birthDate: "",
        },
      ],
    });
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
