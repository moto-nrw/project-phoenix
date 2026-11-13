import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TargetOriginChip } from "./staff-time-views";

describe("TargetOriginChip (#3259)", () => {
  it("names only the model when no Sonderarbeitszeit applies", () => {
    render(<TargetOriginChip />);

    expect(screen.getByText("Soll aus Arbeitszeitmodell")).toBeInTheDocument();
  });

  it("names the Sonderarbeitszeit too when one applies this week", () => {
    render(<TargetOriginChip hasTargetOverride />);

    expect(screen.getByText("Soll mit Sonderarbeitszeit")).toBeInTheDocument();
  });
});

// #6887: ohne gepflegtes Soll behauptet der Chip kein Arbeitszeitmodell.
describe("TargetOriginChip ohne Soll", () => {
  it("nennt das fehlende Soll und führt die Leitung zum Arbeitszeitmodell", () => {
    const onSetTarget = vi.fn();
    render(<TargetOriginChip hasTarget={false} onSetTarget={onSetTarget} />);

    expect(screen.getByText("Kein Soll hinterlegt")).toBeInTheDocument();
    expect(
      screen.queryByText("Soll aus Arbeitszeitmodell"),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Soll festlegen" }));
    expect(onSetTarget).toHaveBeenCalled();
  });

  it("verlinkt das eigene Arbeitszeitmodell", () => {
    render(
      <TargetOriginChip
        hasTarget={false}
        setTargetHref="/staff/7?tab=arbeitszeitmodell"
      />,
    );

    expect(
      screen.getByRole("link", { name: "Soll festlegen" }),
    ).toHaveAttribute("href", "/staff/7?tab=arbeitszeitmodell");
  });

  it("sagt ohne Recht, wer das Soll einträgt", () => {
    render(<TargetOriginChip hasTarget={false} />);

    expect(screen.getByText("Kein Soll hinterlegt")).toBeInTheDocument();
    expect(
      screen.getByText("Das Soll trägt die Leitung ein."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Soll festlegen" }),
    ).not.toBeInTheDocument();
  });
});
