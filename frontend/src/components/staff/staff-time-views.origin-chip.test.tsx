import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TargetOriginChip } from "./staff-time-views";

describe("TargetOriginChip (#3259)", () => {
  it("names only the model when no Sonderarbeitszeit applies", () => {
    render(<TargetOriginChip />);

    expect(screen.getByText("Soll aus Arbeitszeitmodell")).toBeInTheDocument();
  });

  it("names the Sonderarbeitszeit too when one applies this week", () => {
    render(<TargetOriginChip hasTargetOverride />);

    expect(
      screen.getByText("Soll aus Arbeitszeitmodell und Sonderarbeitszeit"),
    ).toBeInTheDocument();
  });
});
