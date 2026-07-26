import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { AllowedDepartureModes } from "~/lib/student-helpers";
import { AllowedDepartureModesDisplay } from "./allowed-departure-modes-display";

describe("AllowedDepartureModesDisplay", () => {
  it("collapses to 'Geht immer alleine' when no bus/pickup is configured", () => {
    render(<AllowedDepartureModesDisplay value={{}} />);
    expect(screen.getByText("Geht immer alleine")).toBeInTheDocument();
    // No weekday rows when there is nothing to show.
    expect(screen.queryByText("Mo")).not.toBeInTheDocument();
  });

  it("renders bus + pickup on every weekday without truncation", () => {
    // The exact case that overflowed the modal: both modes on all five days.
    const everyDay: AllowedDepartureModes = {
      mon: ["bus", "pickup"],
      tue: ["bus", "pickup"],
      wed: ["bus", "pickup"],
      thu: ["bus", "pickup"],
      fri: ["bus", "pickup"],
    };
    render(<AllowedDepartureModesDisplay value={everyDay} />);

    for (const day of ["Mo", "Di", "Mi", "Do", "Fr"]) {
      expect(screen.getByText(day)).toBeInTheDocument();
    }
    expect(screen.getAllByText("Bus")).toHaveLength(5);
    expect(screen.getAllByText("Abgeholt")).toHaveLength(5);
  });

  it("folds unconfigured weekdays to a 'Zu Fuß' badge", () => {
    render(<AllowedDepartureModesDisplay value={{ mon: ["bus"] }} />);
    expect(screen.getByText("Bus")).toBeInTheDocument();
    // Tue–Fri have no arrangement and fold to "Zu Fuß".
    expect(screen.getAllByText("Zu Fuß")).toHaveLength(4);
  });
});
