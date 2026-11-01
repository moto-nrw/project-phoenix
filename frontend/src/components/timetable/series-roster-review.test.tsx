import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { RosterDeviation } from "~/lib/roster-deviation";

import { SeriesRosterReview } from "./series-roster-review";

const deviation: RosterDeviation = {
  templateId: "7",
  offerings: [{ id: "3", name: "Randstunde" }],
  gradeLevels: [1],
  schoolClasses: [],
  unbookedCount: 3,
  missingCount: 2,
  candidates: [{ id: "3", name: "Randstunde" }],
  combinations: [],
  ambiguous: false,
};

describe("SeriesRosterReview (#5103)", () => {
  it("offers both decisions on a deviating series", () => {
    const onConnect = vi.fn();
    const onSetManualRoster = vi.fn();
    render(
      <SeriesRosterReview
        deviation={deviation}
        manualRosterConfirmed={false}
        onConnect={onConnect}
        onSetManualRoster={onSetManualRoster}
      />,
    );

    expect(screen.getByText("Weicht von Anmeldungen ab")).toBeInTheDocument();
    expect(
      screen.getByText("3 Kinder sind nicht für dieses Angebot angemeldet."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("2 angemeldete Kinder fehlen in der Liste."),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Mit Angebot verbinden" }),
    );
    expect(onConnect).toHaveBeenCalledWith(deviation);
    fireEvent.click(screen.getByRole("button", { name: "Liste so lassen" }));
    expect(onSetManualRoster).toHaveBeenCalledWith(true);
  });

  it("lets planners undo „Liste so lassen“", () => {
    const onSetManualRoster = vi.fn();
    render(
      <SeriesRosterReview
        manualRosterConfirmed
        onSetManualRoster={onSetManualRoster}
      />,
    );

    expect(
      screen.getByText("Die Kinderliste bleibt bewusst so."),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Hinweis wieder zeigen" }),
    );
    expect(onSetManualRoster).toHaveBeenCalledWith(false);
  });

  it("shows the confirmed state without an action to readers", () => {
    render(<SeriesRosterReview manualRosterConfirmed />);

    expect(
      screen.getByText("Die Kinderliste bleibt bewusst so."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Hinweis wieder zeigen" }),
    ).not.toBeInTheDocument();
  });
});
