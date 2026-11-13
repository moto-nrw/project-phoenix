import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { OriginChip } from "./origin-chip";

describe("OriginChip", () => {
  it("renders the source label as muted text without any brand color", () => {
    render(<OriginChip label="Soll aus Arbeitszeitmodell" />);

    const chip = screen.getByText("Soll aus Arbeitszeitmodell");
    expect(chip).toBeInTheDocument();
    // Neutral grays only.
    expect(chip.className).toContain("text-gray-700");
    expect(chip.className).toContain("border-gray-300");
    expect(chip.className).toContain("bg-white");
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
});
