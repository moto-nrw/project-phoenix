import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RosterMaintenanceBadge } from "./roster-maintenance-badge";
import type { TemplateRosterMaintenance } from "~/lib/timetable-types";

const base: TemplateRosterMaintenance = {
  mode: "manual",
  offeringNames: [],
  gradeLevels: [],
  schoolClasses: [],
  inactiveOfferingNames: [],
  dynamicTargets: false,
  careOfferingsDisabled: false,
};

describe("RosterMaintenanceBadge", () => {
  it("exposes label and explanation to keyboard and screen reader users", () => {
    render(
      <RosterMaintenanceBadge
        state={{ ...base, mode: "automatic", offeringNames: ["Spätdienst"] }}
      />,
    );

    const trigger = screen
      .getByText("Automatisch")
      .closest<HTMLElement>("[tabindex]");
    if (!trigger) throw new Error("badge is not focusable");
    expect(trigger).toHaveAttribute("tabindex", "0");
    expect(trigger).toHaveTextContent("Teilnehmerpflege: Automatisch");

    const bubble = screen.getByRole("tooltip");
    expect(trigger).toHaveAttribute("aria-describedby", bubble.id);
    expect(bubble).toHaveTextContent(
      "Neue Anmeldungen werden automatisch übernommen.",
    );
  });

  it("uses a different label per state, not only a different colour", () => {
    const { rerender } = render(<RosterMaintenanceBadge state={base} />);
    expect(screen.getByText("Manuell")).toBeInTheDocument();

    rerender(
      <RosterMaintenanceBadge
        state={{
          ...base,
          mode: "partial",
          offeringNames: ["Mittagessen"],
          dynamicTargets: true,
        }}
      />,
    );
    expect(screen.getByText("Teils automatisch")).toBeInTheDocument();
  });
});
