import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  PlanningContextBarSkeleton,
  TimetableContentSkeleton,
} from "./betreuungsplan-skeleton";

// Live-Regionen sagen ihren Text an, nicht ihr aria-label (#4996, wie #5086):
// jede Ladefläche des Betreuungsplans trägt ihren Namen auch als Text.
describe("Betreuungsplan skeletons", () => {
  it.each([
    ["series", "Regeltermine werden geladen"],
    ["day", "Tagesansicht wird geladen"],
    ["week", "Wochenplan wird geladen"],
    ["month", "Monatsplan wird geladen"],
  ] as const)("announces the %s view with text", (view, label) => {
    render(<TimetableContentSkeleton view={view} />);

    expect(screen.getByRole("status", { name: label })).toHaveTextContent(
      label,
    );
  });

  it("announces the head with text", () => {
    render(<PlanningContextBarSkeleton />);

    expect(
      screen.getByRole("status", {
        name: "Betreuungsplan-Kopfzeile wird geladen",
      }),
    ).toHaveTextContent("Betreuungsplan-Kopfzeile wird geladen");
  });
});
