import { describe, expect, it } from "vitest";
import { PARENT_HOLIDAY_REGISTRATION_ENTRIES } from "~/test/fixtures/holiday-registrations";
import { holidayRegistrationEntriesFromBackend } from "./parent-holiday-registration-entries";

describe("holidayRegistrationEntriesFromBackend (#6673)", () => {
  it("maps the wire shape to strings and camelCase in the backend's order", () => {
    const [first, second] = holidayRegistrationEntriesFromBackend(
      PARENT_HOLIDAY_REGISTRATION_ENTRIES,
    );
    expect(first).toEqual({
      id: "12",
      traegerName: "Träger Bissendorf",
      periodName: "Weihnachtsferien 2026/27",
      firstDay: "2027-01-04",
      lastDay: "2027-01-08",
      submittedAt: "2026-09-08T09:30:00Z",
      status: "received",
      children: [
        {
          firstName: "Mia",
          weeks: [
            {
              firstDay: "2027-01-04",
              lastDay: "2027-01-08",
              timeModelName: "bis 16 Uhr",
              placement: "waitlisted",
            },
          ],
        },
      ],
    });
    expect(second?.children.map((child) => child.firstName)).toEqual([
      "Mia",
      "Ben",
    ]);
  });

  it("reads unknown states as the quiet default and tolerates missing lists", () => {
    const [entry] = holidayRegistrationEntriesFromBackend({
      registrations: [
        {
          id: 1,
          traeger_name: "T",
          period_name: "P",
          submitted_at: "2026-09-01T08:00:00Z",
          status: "something_new",
          children: [
            {
              first_name: "Ida",
              weeks: [
                {
                  first_day: "2027-01-04",
                  last_day: "2027-01-08",
                  time_model_name: "",
                  placement: "unknown",
                },
              ],
            },
          ],
        },
      ],
    });
    expect(entry?.status).toBe("received");
    expect(entry?.firstDay).toBe("");
    expect(entry?.children[0]?.weeks[0]?.placement).toBe("placed");
    expect(holidayRegistrationEntriesFromBackend(null)).toEqual([]);
    expect(
      holidayRegistrationEntriesFromBackend({ registrations: null }),
    ).toEqual([]);
  });
});
