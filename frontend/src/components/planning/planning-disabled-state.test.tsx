import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PlanningDisabledState } from "./planning-disabled-state";

describe("PlanningDisabledState", () => {
  it("renders the supplied page title, copy, and stable test id", () => {
    render(
      <PlanningDisabledState
        pageTitle="Dienstplan"
        heading="Dienstplan ist deaktiviert"
        description="Für diese Schule ausgeschaltet."
        testId="dienstplan-disabled-state"
      />,
    );

    expect(screen.getByTestId("dienstplan-disabled-state")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: "Dienstplan" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Für diese Schule ausgeschaltet."),
    ).toBeInTheDocument();
  });
});
