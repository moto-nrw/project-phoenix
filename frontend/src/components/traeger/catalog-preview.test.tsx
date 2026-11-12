import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { TraegerSchoolPlan } from "~/lib/traeger-catalog-api";
import { RolloutResultCard, SchoolPlanCard } from "./catalog-preview";

const schools = [{ id: "11", name: "Grundschule Nord" }] as never;

const plan: TraegerSchoolPlan = {
  schoolId: "11",
  action: "update",
  outcome: "ready",
  errorCode: null,
  careBeyondEnd: 2,
  phase: [
    {
      field: "service_end_date",
      state: "changes",
      traeger: "2028-06-30",
      school: "2028-07-31",
    },
  ],
  offerings: [],
};

// Eine Kürzung beendet keine Betreuung (#6813, ADR 0211): Vorschau und
// Ergebnis nennen, wie viele Kinder länger betreut werden.
describe("care beyond a shortened phase", () => {
  it("warns in the preview of the school", () => {
    render(<SchoolPlanCard plan={plan} schools={schools} />);
    expect(
      screen.getByText(/^2 Kinder werden länger betreut/),
    ).toHaveTextContent("Sagen Sie der Schule Bescheid.");
  });

  it("names the count in the result of the school", () => {
    render(
      <RolloutResultCard
        rollout={{
          traegerPhaseId: "7",
          version: 2,
          schools: [
            {
              schoolId: "11",
              outcome: "applied",
              errorCode: null,
              careBeyondEnd: 1,
            },
          ],
        }}
        schools={schools}
      />,
    );
    expect(
      screen.getAllByText(/^1 Kind wird länger betreut/).length,
    ).toBeGreaterThan(0);
  });
});
