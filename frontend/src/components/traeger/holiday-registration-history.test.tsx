import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import {
  HOLIDAY_PERIOD,
  HOLIDAY_REGISTRATION,
} from "~/test/fixtures/holiday-registrations";
import { HolidayRegistrationHistory } from "./holiday-registration-history";

it("distinguishes a change of days within the same week and time model", () => {
  render(
    <HolidayRegistrationHistory
      period={HOLIDAY_PERIOD}
      names={new Map([["31", "OGS Nord"]])}
      registration={{
        ...HOLIDAY_REGISTRATION,
        history: [
          {
            action: "rebook",
            from: "",
            debtorNumber: null,
            occurredAt: "2026-09-08T10:00:00Z",
            childId: "9",
            before: [{ weekId: "51", timeModelId: "62", days: ["2027-01-04"] }],
            after: [
              {
                weekId: "51",
                timeModelId: "62",
                days: ["2027-01-05", "2027-01-06"],
              },
            ],
            siblings: [],
            fields: [],
          },
        ],
      }}
    />,
  );
  expect(
    screen.getByText(/vorher .*Mo 04\.01\..*jetzt .*Di 05\.01\., Mi 06\.01\./),
  ).toBeInTheDocument();
});

it("names an allocation with its reason and a check of the criteria (#4700)", () => {
  render(
    <HolidayRegistrationHistory
      period={HOLIDAY_PERIOD}
      names={new Map([["31", "OGS Nord"]])}
      registration={{
        ...HOLIDAY_REGISTRATION,
        criteria: [
          {
            criterionId: "1",
            name: "Beide Eltern berufstätig",
            question: "Sind beide Eltern berufstätig?",
            scope: "family",
            childId: null,
            stated: "yes",
            checked: "no",
            checkedAt: "2026-09-08T09:00:00Z",
          },
        ],
        history: [
          {
            action: "allocate",
            from: "",
            debtorNumber: null,
            occurredAt: "2026-09-08T10:00:00Z",
            childId: "9",
            before: [],
            after: [],
            siblings: [],
            fields: [],
            weekIds: ["51"],
            placement: "placed",
            reason: "Härtefall",
          },
          {
            action: "criteria_check",
            from: "",
            debtorNumber: null,
            occurredAt: "2026-09-08T09:00:00Z",
            childId: "",
            before: [],
            after: [],
            siblings: [],
            fields: [],
            checks: [{ criterionId: "1", childId: null, checked: "no" }],
          },
        ],
      }}
    />,
  );
  expect(screen.getByText("Vergabe entschieden")).toBeInTheDocument();
  expect(
    screen.getByText(
      /: Platz · Woche 04\.01\. bis 08\.01\.2027 · Begründung: Härtefall/,
    ),
  ).toBeInTheDocument();
  expect(
    screen.getByText("Angaben für die Vergabe geprüft"),
  ).toBeInTheDocument();
  expect(
    screen.getByText(/Beide Eltern berufstätig: trifft nicht zu/),
  ).toBeInTheDocument();
});
