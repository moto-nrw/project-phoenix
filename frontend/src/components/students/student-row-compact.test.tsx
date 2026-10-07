import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PickupTimeRow } from "./student-card";

// The one-line phone list (#3834) shows the bare pickup time.
describe("PickupTimeRow compact variant", () => {
  const now = new Date();

  it("shows the time without label, unit or icon", () => {
    const { container } = render(
      <PickupTimeRow
        pickupTime="15:30"
        isException={false}
        notes="Oma holt ab"
        now={now}
        variant="compact"
      />,
    );

    expect(screen.getByText("15:30")).toBeInTheDocument();
    expect(screen.queryByText(/Uhr/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Gehzeit/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Oma holt ab/)).not.toBeInTheDocument();
    expect(container.querySelector("svg")).toBeNull();
  });

  it("keeps label and unit on the card", () => {
    render(<PickupTimeRow pickupTime="15:30" isException={false} now={now} />);

    expect(screen.getByText("Gehzeit: 15:30 Uhr")).toBeInTheDocument();
  });
});
