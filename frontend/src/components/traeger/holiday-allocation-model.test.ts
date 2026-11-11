import { describe, expect, it } from "vitest";
import {
  allocationBoard,
  allocationEntry,
  allocationWeek,
} from "~/test/fixtures/holiday-allocation";
import {
  allocationChips,
  capacityBoundary,
  criterionTone,
  deviatesFromProposal,
  freePlacesText,
  openWeeks,
  proposalDecisions,
  proposalSummary,
  proposalSummaryText,
} from "./holiday-allocation-model";

describe("holiday allocation model (#4700)", () => {
  it("shows met and unanswered criteria in priority order, never unmet ones", () => {
    const board = allocationBoard();
    const [first, second, third] = board.weeks[0]!.pending;
    expect(
      allocationChips(first!, board.criteria).map((chip) => [
        chip.criterion.name,
        chip.value,
      ]),
    ).toEqual([["Beide Eltern berufstätig", "yes"]]);
    expect(allocationChips(second!, board.criteria)).toEqual([
      {
        criterion: board.criteria[0],
        value: "yes",
        checkedByOffice: true,
      },
      {
        criterion: board.criteria[1],
        value: "unknown",
        checkedByOffice: false,
      },
    ]);
    expect(allocationChips(third!, board.criteria)).toEqual([]);
  });

  it("reads a criterion the entry has no value for as unanswered", () => {
    const board = allocationBoard();
    const chips = allocationChips(
      allocationEntry({ criteria: [] }),
      board.criteria,
    );
    expect(chips.map((chip) => chip.value)).toEqual(["unknown", "unknown"]);
  });

  it("draws the capacity line before the first proposed waiting list entry", () => {
    expect(capacityBoundary(allocationWeek().pending)).toBe(2);
    expect(
      capacityBoundary([
        allocationEntry({ proposal: "waitlisted" }),
        allocationEntry({ proposal: "waitlisted" }),
      ]),
    ).toBeNull();
    expect(capacityBoundary([allocationEntry()])).toBeNull();
  });

  it("hides the capacity line when a later day booking still gets a place", () => {
    expect(
      capacityBoundary([
        allocationEntry({ proposal: "placed" }),
        allocationEntry({ proposal: "waitlisted" }),
        allocationEntry({ proposal: "placed" }),
      ]),
    ).toBeNull();
  });

  it("asks for a reason only when the decision differs from the proposal", () => {
    const entry = allocationEntry({ proposal: "placed" });
    expect(deviatesFromProposal(entry, "placed")).toBe(false);
    expect(deviatesFromProposal(entry, "waitlisted")).toBe(true);
  });

  it("takes over the proposal of the open weeks without a reason", () => {
    const board = allocationBoard({
      weeks: [allocationWeek(), allocationWeek({ weekId: "52", pending: [] })],
    });
    const weeks = openWeeks(board);
    expect(weeks.map((week) => week.weekId)).toEqual(["51"]);
    expect(proposalDecisions(weeks)).toEqual([
      { bookingId: "100", placement: "placed", proposal: "placed", reason: "" },
      { bookingId: "101", placement: "placed", proposal: "placed", reason: "" },
      {
        bookingId: "102",
        placement: "waitlisted",
        proposal: "waitlisted",
        reason: "",
      },
    ]);
  });

  it("counts places, waiting list and the registrations that get an e-mail", () => {
    const summary = proposalSummary([allocationWeek()]);
    expect(summary).toEqual({ places: 2, waitlisted: 1, registrations: 2 });
    expect(proposalSummaryText(summary)).toBe(
      "2 Kinder bekommen einen Platz. 1 Kind kommt auf die Warteliste.",
    );
    expect(
      proposalSummaryText({ places: 1, waitlisted: 0, registrations: 1 }),
    ).toBe("1 Kind bekommt einen Platz.");
  });

  it("names the free places of a week", () => {
    expect(freePlacesText(allocationWeek())).toBe("2 Plätze frei");
    expect(freePlacesText(allocationWeek({ free: 0 }))).toBe("Kein Platz frei");
    expect(
      freePlacesText(allocationWeek({ free: 0, bookingUnit: "day" })),
    ).toBe("Mindestens ein Tag voll");
    expect(freePlacesText(allocationWeek({ free: null }))).toBe("Ohne Grenze");
    expect(
      freePlacesText(allocationWeek({ free: 1, bookingUnit: "day" })),
    ).toBe("An jedem Tag mindestens 1 Platz frei");
  });

  it("gives every priority its colour and keeps purple for the open place", () => {
    expect([1, 2, 3, 4, 5, 6].map(criterionTone)).toEqual([
      "blue",
      "cyan",
      "green",
      "orange",
      "gray",
      "gray",
    ]);
  });
});
