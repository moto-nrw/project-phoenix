import { describe, expect, it } from "vitest";

import {
  type OfferingAdjustmentSnapshot as Snapshot,
  describeOfferingAdjustmentChanges,
} from "./offering-adjustment-history";

describe("describeOfferingAdjustmentChanges", () => {
  const randstunde: Snapshot = {
    offering_id: "1",
    offering_name: "Randstunde",
    days_of_week_mode: "parent_choice",
    available_days: ["mon", "tue", "wed", "thu", "fri"],
    selected_days: ["tue", "wed", "thu", "fri"],
  };
  const ganztag1430 = (selectedDays: string[]): Snapshot => ({
    offering_id: "2",
    offering_name: "Ganztagsbetreuung bis 14.30 Uhr",
    days_of_week_mode: "parent_choice",
    available_days: ["mon", "tue", "wed", "thu", "fri"],
    selected_days: selectedDays,
  });
  const ganztag16 = (selectedDays: string[]): Snapshot => ({
    offering_id: "3",
    offering_name: "Ganztagsbetreuung bis 16 Uhr",
    days_of_week_mode: "parent_choice",
    available_days: ["mon", "tue", "wed", "thu", "fri"],
    selected_days: selectedDays,
  });

  function lines(before: Snapshot[], after: Snapshot[]): string[] {
    return describeOfferingAdjustmentChanges(before, after).map((line) =>
      line.days ? `${line.days}: ${line.text}` : line.text,
    );
  }

  it("names a weekday that was added to an offering whose name stayed the same", () => {
    expect(
      lines(
        [randstunde, ganztag1430(["mon"])],
        [randstunde, ganztag1430(["mon", "wed"])],
      ),
    ).toEqual(["Mi: Ganztagsbetreuung bis 14.30 Uhr gebucht"]);
  });

  it("groups the removed days of one offering into one line", () => {
    expect(
      lines(
        [randstunde, ganztag1430(["mon", "wed"])],
        [ganztag1430(["mon", "wed"])],
      ),
    ).toEqual(["Di, Mi, Do, Fr: Randstunde abgemeldet"]);
  });

  it("shows before and after when one offering replaces another on a day", () => {
    expect(
      lines(
        [ganztag1430(["mon", "wed"])],
        [ganztag1430(["mon"]), ganztag16(["wed"])],
      ),
    ).toEqual([
      "Mi: vorher Ganztagsbetreuung bis 14.30 Uhr, nachher Ganztagsbetreuung bis 16 Uhr",
    ]);
  });

  it("uses the available days of a fixed offering without selected days", () => {
    expect(
      lines(
        [],
        [
          {
            offering_id: "4",
            offering_name: "Frühbetreuung",
            days_of_week_mode: "fixed",
            available_days: ["fri", "mon"],
            selected_days: [],
          },
        ],
      ),
    ).toEqual(["Mo, Fr: Frühbetreuung gebucht"]);
  });

  it("falls back to a name change for offerings without known days", () => {
    expect(
      lines(
        [{ offering_id: "7", days_of_week_mode: "parent_choice" }],
        [
          {
            offering_id: "8",
            offering_name: "Ferienbetreuung",
            days_of_week_mode: "fixed",
            available_days: [],
          },
          ganztag16(["thu"]),
        ],
      ),
    ).toEqual([
      "Do: Ganztagsbetreuung bis 16 Uhr gebucht",
      "Angebot #7 abgemeldet",
      "Ferienbetreuung gebucht",
    ]);
  });

  it("keeps the known days of an offering whose days are unknown on one side", () => {
    expect(
      lines([ganztag16(["mon"])], [{ ...ganztag16([]), selected_days: [] }]),
    ).toEqual(["Mo: Ganztagsbetreuung bis 16 Uhr abgemeldet"]);
  });

  it("reports unchanged offerings", () => {
    expect(lines([randstunde], [{ ...randstunde }])).toEqual([
      "Angebote unverändert",
    ]);
    expect(lines([], [])).toEqual(["Angebote unverändert"]);
  });

  it("orders days from Monday and joins several offerings", () => {
    expect(
      lines(
        [],
        [
          ganztag16(["fri", "mon"]),
          { ...randstunde, selected_days: ["mon", "fri"] },
          ganztag1430(["wed"]),
        ],
      ),
    ).toEqual([
      "Mo, Fr: Ganztagsbetreuung bis 16 Uhr und Randstunde gebucht",
      "Mi: Ganztagsbetreuung bis 14.30 Uhr gebucht",
    ]);
  });
});
