import { describe, expect, it } from "vitest";
import { HOLIDAY_PERIOD } from "~/test/fixtures/holiday-registrations";
import { formatWeeks } from "./holiday-period-format";

describe("formatWeeks", () => {
  it("counts weeks and their days", () => {
    expect(formatWeeks(HOLIDAY_PERIOD)).toBe(
      "2 Wochen: 04.01.2027 bis 15.01.2027",
    );
  });

  it("counts parallel weeks once and names the schools (#5689)", () => {
    const [first, second] = HOLIDAY_PERIOD.weeks;
    expect(
      formatWeeks({
        ...HOLIDAY_PERIOD,
        weeks: [
          first!,
          { ...second!, startsOn: first!.startsOn, endsOn: first!.endsOn },
        ],
      }),
    ).toBe("1 Woche an 2 Schulen: 04.01.2027 bis 08.01.2027");
  });
});
