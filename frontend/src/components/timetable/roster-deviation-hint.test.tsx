import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { RosterDeviation } from "~/lib/roster-deviation";

import { RosterDeviationHint } from "./roster-deviation-hint";

const deviation: RosterDeviation = {
  templateId: "7",
  offering: { id: "3", name: "Randstunde" },
  gradeLevels: [],
  schoolClasses: ["1a"],
  unbookedCount: 8,
  missingCount: 0,
  candidates: [{ id: "3", name: "Randstunde" }],
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

    expect(
      screen.getByText(
        "Weicht von Anmeldungen ab: 8 Kinder ohne Buchung · passt zu „Randstunde“",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Mit Angebot verbinden" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Bleibt von Hand gepflegt" }),
    );
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
      screen.queryByRole("button", { name: "Bleibt von Hand gepflegt" }),
    ).not.toBeInTheDocument();
  });
});
