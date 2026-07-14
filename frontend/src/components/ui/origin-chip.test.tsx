import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { OriginChip } from "./origin-chip";

describe("OriginChip", () => {
  it("renders the source label as muted text without any brand color", () => {
    render(<OriginChip label="Soll aus Arbeitszeitmodell" />);

    const chip = screen.getByText("Soll aus Arbeitszeitmodell");
    expect(chip).toBeInTheDocument();
    // Anatomy from docs/04-designsprache.md 6.2: neutral grays only.
    expect(chip.className).toContain("text-gray-600");
    expect(chip.className).toContain("border-gray-200");
    expect(chip.className).toContain("bg-gray-50");
    expect(chip.className).toContain("rounded-full");
  });

  it("exposes the long form as a native tooltip", () => {
    render(
      <OriginChip
        label="Soll aus Arbeitszeitmodell"
        title="Wochensaldo und Stundenkonto rechnen gegen das im Arbeitszeitmodell hinterlegte Soll."
      />,
    );

    expect(screen.getByText("Soll aus Arbeitszeitmodell")).toHaveAttribute(
      "title",
      "Wochensaldo und Stundenkonto rechnen gegen das im Arbeitszeitmodell hinterlegte Soll.",
    );
  });

  it("renders an optional leading icon", () => {
    render(
      <OriginChip
        label="Quelle"
        icon={<svg data-testid="chip-icon" width="12" height="12" />}
      />,
    );

    expect(screen.getByTestId("chip-icon")).toBeInTheDocument();
  });
});
