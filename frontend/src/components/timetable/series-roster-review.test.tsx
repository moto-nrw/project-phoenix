import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { RosterDeviation } from "~/lib/roster-deviation";

import { SeriesRosterReview } from "./series-roster-review";

const deviation: RosterDeviation = {
  templateId: "7",
  offering: { id: "3", name: "Randstunde" },
  gradeLevels: [1],
  schoolClasses: [],
  unbookedCount: 3,
  missingCount: 2,
  candidates: [{ id: "3", name: "Randstunde" }],
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

    expect(
      screen.getByText(
        "Weicht von Anmeldungen ab: 3 Kinder ohne Buchung · 2 angemeldete Kinder fehlen · passt zu „Randstunde“",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Mit Angebot verbinden" }),
    );
    expect(onConnect).toHaveBeenCalledWith(deviation);
    fireEvent.click(
      screen.getByRole("button", { name: "Bleibt von Hand gepflegt" }),
    );
    expect(onSetManualRoster).toHaveBeenCalledWith(true);
  });

  it("lets planners undo „Bleibt von Hand gepflegt“", () => {
    const onSetManualRoster = vi.fn();
    render(
      <SeriesRosterReview
        manualRosterConfirmed
        onSetManualRoster={onSetManualRoster}
      />,
    );

    expect(
      screen.getByText("Sie pflegen die Kinder hier bewusst selbst."),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Hinweis wieder zeigen" }),
    );
    expect(onSetManualRoster).toHaveBeenCalledWith(false);
  });

  it("shows the confirmed state without an action to readers", () => {
    render(<SeriesRosterReview manualRosterConfirmed />);

    expect(
      screen.getByText("Sie pflegen die Kinder hier bewusst selbst."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Hinweis wieder zeigen" }),
    ).not.toBeInTheDocument();
  });
});
