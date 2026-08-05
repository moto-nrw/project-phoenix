import { describe, expect, it } from "vitest";

import { mapBirthdayOverview } from "./birthdays-api";

describe("mapBirthdayOverview", () => {
  it("maps the backend payload into the camelCase shape the card renders", () => {
    const overview = mapBirthdayOverview({
      enabled: true,
      include_staff: true,
      today: "2026-08-03",
      celebrations: [
        {
          kind: "student",
          id: 12,
          name: "Lina Adler",
          group_name: "Delfine",
          school_class: "1a",
          date: "2026-08-03",
          age: 8,
          is_today: true,
        },
        {
          kind: "staff",
          id: 7,
          name: "Anna Berg",
          date: "2026-08-01",
          is_today: false,
        },
      ],
    });

    expect(overview.enabled).toBe(true);
    expect(overview.includeStaff).toBe(true);
    expect(overview.today).toBe("2026-08-03");
    expect(overview.celebrations).toEqual([
      {
        kind: "student",
        id: "12",
        name: "Lina Adler",
        groupName: "Delfine",
        schoolClass: "1a",
        date: "2026-08-03",
        age: 8,
        isToday: true,
      },
      {
        kind: "staff",
        id: "7",
        name: "Anna Berg",
        groupName: undefined,
        schoolClass: undefined,
        date: "2026-08-01",
        age: undefined,
        isToday: false,
      },
    ]);
  });

  // Go marshals an empty slice as null; the card must render an empty list
  // rather than crash on it.
  it("treats a null celebration list as empty", () => {
    const overview = mapBirthdayOverview({
      enabled: true,
      include_staff: false,
      today: "2026-08-05",
      celebrations: null,
    });

    expect(overview.celebrations).toEqual([]);
  });

  it("keeps a disabled display disabled", () => {
    const overview = mapBirthdayOverview({
      enabled: false,
      include_staff: false,
      today: "2026-08-05",
      celebrations: null,
    });

    expect(overview.enabled).toBe(false);
  });
});
