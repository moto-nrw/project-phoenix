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
