import { describe, expect, it } from "vitest";
import {
  bookedWeeksOverlap,
  groupParallelWeeks,
  weekGroupBooking,
  withWeekGroupBooking,
} from "./holiday-week-groups";

const week = (id: string, startsOn: string, endsOn: string) => ({
  id,
  startsOn,
  endsOn,
  timeModels: [{ id: `${id}-model` }],
});

const nord = week("1", "2027-01-04", "2027-01-08");
const sued = week("2", "2027-01-04", "2027-01-08");
const later = week("3", "2027-01-11", "2027-01-15");
const partly = week("4", "2027-01-08", "2027-01-12");

describe("groupParallelWeeks", () => {
  it("keeps weeks on other days apart", () => {
    expect(groupParallelWeeks([nord, later])).toEqual([[nord], [later]]);
  });

  it("puts the same days at several schools into one group", () => {
    expect(groupParallelWeeks([nord, sued, later])).toEqual([
      [nord, sued],
      [later],
    ]);
  });

  it("keeps weeks that share only some days apart, so no booking hides", () => {
    expect(groupParallelWeeks([nord, partly, later])).toEqual([
      [nord],
      [partly],
      [later],
    ]);
  });
});

describe("bookedWeeksOverlap", () => {
  const weeks = [nord, partly, later];

  it("finds booked weeks that share a day", () => {
    expect(bookedWeeksOverlap(weeks, ["1", "4"])).toBe(true);
  });

  it("accepts booked weeks without a shared day", () => {
    expect(bookedWeeksOverlap(weeks, ["1", "3"])).toBe(false);
  });
});

describe("the booking of a group", () => {
  it("replaces the booking of the other school", () => {
    const bookings = { "1": "1-model", "3": "3-model" };
    expect(withWeekGroupBooking(bookings, [nord, sued], "2-model")).toEqual({
      "2": "2-model",
      "3": "3-model",
    });
  });

  it("removes the booking of the group when nothing is chosen", () => {
    expect(withWeekGroupBooking({ "1": "1-model" }, [nord, sued], "")).toEqual(
      {},
    );
  });

  it("reads the time model booked in the group", () => {
    expect(weekGroupBooking({ "2": "2-model" }, [nord, sued])).toBe("2-model");
    expect(weekGroupBooking({ "3": "3-model" }, [nord, sued])).toBe("");
  });
});
