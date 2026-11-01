import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { RosterDeviation } from "~/lib/roster-deviation";

import { RosterDeviationHint } from "./roster-deviation-hint";

const deviation: RosterDeviation = {
  templateId: "7",
  offerings: [{ id: "3", name: "Randstunde" }],
  gradeLevels: [],
  schoolClasses: ["1a"],
  unbookedCount: 8,
  missingCount: 0,
  candidates: [{ id: "3", name: "Randstunde" }],
  combinations: [],
  ambiguous: false,
};

describe("RosterDeviationHint", () => {
  it("says how the list differs and offers both decisions to planners", () => {
    const onConnect = vi.fn();
    const onKeepManual = vi.fn();
    render(
      <RosterDeviationHint
        deviation={deviation}
        onConnect={onConnect}
        onKeepManual={onKeepManual}
      />,
    );

    expect(screen.getByText("Weicht von Anmeldungen ab")).toBeInTheDocument();
    expect(screen.getByText("Passt zu „Randstunde“.")).toBeInTheDocument();
    expect(
      screen.getByText("8 Kinder sind nicht für dieses Angebot angemeldet."),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Mit Angebot verbinden" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Liste so lassen" }));
    expect(onConnect).toHaveBeenCalledTimes(1);
    expect(onKeepManual).toHaveBeenCalledTimes(1);
  });

  it("shows only the information to readers who cannot plan", () => {
    render(<RosterDeviationHint deviation={deviation} />);

    expect(screen.getByText(/Weicht von Anmeldungen ab/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Mit Angebot verbinden" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Liste so lassen" }),
    ).not.toBeInTheDocument();
  });

  it("names one child with its weekday in the sentence", () => {
    render(
      <RosterDeviationHint
        deviation={{
          ...deviation,
          offerings: [
            { id: "3", name: "Randstunde" },
            { id: "4", name: "Ganztag" },
          ],
          unbookedCount: 1,
          unbooked: [{ studentId: "9", name: "Mia Sommer", weekdays: [5] }],
        }}
      />,
    );

    expect(
      screen.getByText(
        "Mia Sommer ist freitags für keines dieser Angebote angemeldet.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Passt zu „Randstunde“ und „Ganztag“."),
    ).toBeInTheDocument();
  });

  it("folds several children into a list the reader opens", () => {
    render(
      <RosterDeviationHint
        deviation={{
          ...deviation,
          unbookedCount: 2,
          unbooked: [
            { studentId: "9", name: "Mia Sommer", weekdays: [5] },
            { studentId: "8", name: "Ben Winter", weekdays: [1, 3] },
          ],
        }}
      />,
    );

    const toggle = screen.getByRole("button", { name: "Namen zeigen" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Mia Sommer: freitags")).not.toBeVisible();

    fireEvent.click(toggle);

    expect(
      screen.getByRole("button", { name: "Namen ausblenden" }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Mia Sommer: freitags")).toBeVisible();
    expect(screen.getByText("Ben Winter: montags und mittwochs")).toBeVisible();
  });
});
