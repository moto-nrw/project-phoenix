import { describe, expect, it } from "vitest";
import {
  centsToEuroInput,
  emptyHolidayPeriod,
  euroInputToCents,
  holidayPeriodComplete,
  type HolidayPeriodDraft,
  inputFromDraft,
  validateHolidayPeriod,
} from "./holiday-period-model";

function completeDraft(): HolidayPeriodDraft {
  const draft = emptyHolidayPeriod();
  const week = draft.weeks[0]!;
  return {
    ...draft,
    name: " Osterferien 2027 ",
    registrationOpensOn: "2027-01-11",
    registrationClosesOn: "2027-02-26",
    weeks: [
      {
        ...week,
        startsOn: "2027-03-22",
        endsOn: "2027-03-26",
        hostSchoolId: "31",
        timeModels: [
          {
            ...week.timeModels[0]!,
            name: "bis 13 Uhr",
            startsAt: "08:00",
            endsAt: "13:00",
            price: "45,5",
          },
        ],
      },
    ],
  };
}

describe("holiday period model", () => {
  it("reads euro prices as whole cent and shows cent as euro", () => {
    expect(euroInputToCents("45")).toBe(4500);
    expect(euroInputToCents("45,5")).toBe(4550);
    expect(euroInputToCents("45.05")).toBe(4505);
    expect(euroInputToCents(" 0,00 ")).toBe(0);
    expect(euroInputToCents("45,555")).toBeNull();
    expect(euroInputToCents("-1")).toBeNull();
    expect(euroInputToCents("")).toBeNull();
    expect(centsToEuroInput(6750)).toBe("67,50");
  });

  it("accepts a complete draft and sends prices in cent", () => {
    const draft = completeDraft();
    expect(validateHolidayPeriod(draft)).toEqual({});
    const input = inputFromDraft(draft);
    expect(input.name).toBe("Osterferien 2027");
    expect(input.weeks[0]!.timeModels[0]!.priceCents).toBe(4550);
    expect(input.weeks[0]!.hostSchoolId).toBe("31");
  });

  it("marks every missing or contradicting field by its server name", () => {
    const draft = completeDraft();
    const week = draft.weeks[0]!;
    const errors = validateHolidayPeriod({
      ...draft,
      name: " ",
      registrationClosesOn: "2027-01-01",
      weeks: [
        week,
        {
          ...week,
          key: "second",
          startsOn: "2027-03-29",
          endsOn: "2027-04-09",
          hostSchoolId: "",
          timeModels: [
            { ...week.timeModels[0]!, endsAt: "07:00", price: "viel" },
            { ...week.timeModels[0]!, key: "twin" },
          ],
        },
      ],
    });
    expect(Object.keys(errors).sort()).toEqual([
      "name",
      "registration_closes_on",
      "weeks.1.ends_on",
      "weeks.1.host_school_id",
      "weeks.1.time_models.0.ends_at",
      "weeks.1.time_models.0.price_cents",
      "weeks.1.time_models.1.name",
    ]);
  });

  it("names the earlier week an overlapping week shares days with", () => {
    const draft = completeDraft();
    const week = draft.weeks[0]!;
    const errors = validateHolidayPeriod({
      ...draft,
      weeks: [
        week,
        {
          ...week,
          key: "overlap",
          startsOn: "2027-03-26",
          endsOn: "2027-03-29",
        },
      ],
    });
    expect(errors["weeks.1.starts_on"]).toBe(
      "Diese Woche überschneidet sich mit Woche 1.",
    );
  });

  it("calls a period complete only with weeks that all have a time model", () => {
    const base = {
      id: "1",
      name: "Ferien",
      registrationOpensOn: "2027-01-11",
      registrationClosesOn: "2027-02-26",
      binding: true,
      notes: "",
      status: "draft" as const,
      publishedAt: null,
      publicToken: "token",
      registrationCount: 0,
    };
    expect(holidayPeriodComplete({ ...base, weeks: [] })).toBe(false);
    const week = {
      id: "2",
      startsOn: "2027-03-22",
      endsOn: "2027-03-26",
      hostSchoolId: "31",
      timeModels: [],
    };
    expect(holidayPeriodComplete({ ...base, weeks: [week] })).toBe(false);
    expect(
      holidayPeriodComplete({
        ...base,
        weeks: [
          {
            ...week,
            timeModels: [
              {
                id: "3",
                name: "bis 13 Uhr",
                startsAt: "08:00",
                endsAt: "13:00",
                priceCents: 4500,
              },
            ],
          },
        ],
      }),
    ).toBe(true);
  });
});
