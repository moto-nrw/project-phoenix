import { describe, expect, it } from "vitest";
import { holidayGuestGroupHintText } from "./holiday-host-api";

describe("holidayGuestGroupHintText (#6680)", () => {
  it("names how many children join the holiday group and from when", () => {
    expect(
      holidayGuestGroupHintText({ startsOn: "2027-01-04", guests: 3 }),
    ).toBe("Feriengruppe: 3 Kinder kommen ab 04.01.2027 dazu.");
    expect(
      holidayGuestGroupHintText({ startsOn: "2027-01-04", guests: 1 }),
    ).toBe("Feriengruppe: 1 Kind kommt ab 04.01.2027 dazu.");
  });

  it("says that nobody is confirmed yet instead of counting zero children", () => {
    expect(
      holidayGuestGroupHintText({ startsOn: "2027-01-04", guests: 0 }),
    ).toBe("Feriengruppe ab 04.01.2027: Noch ist kein Feriengast bestätigt.");
  });
});
