import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { CareWithdrawalCompletion } from "~/lib/care-exit-api";

import { OpenWithdrawalCard } from "./withdrawal-cards";

function row(
  overrides: Partial<CareWithdrawalCompletion>,
): CareWithdrawalCompletion {
  return {
    id: "11",
    studentId: "602",
    firstName: "Emelie",
    lastName: "Beispiel",
    schoolClass: "3a",
    firstBookinglessDay: "2026-11-13",
    urgency: "planned",
    state: "pending",
    ...overrides,
  };
}

describe("OpenWithdrawalCard", () => {
  it("names the last care day while the withdrawal is still planned", () => {
    render(
      <OpenWithdrawalCard row={row({})} finish={vi.fn()} remove={vi.fn()} />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Details anzeigen/ }));

    expect(
      screen.getByText(
        "Letzter Betreuungstag ist der 12.11.2026. Bis dahin bleibt alles wie gebucht. Tragen Sie das Ende jetzt mit „Betreuung beenden“ ein.",
      ),
    ).toBeInTheDocument();
  });

  it("names the day since which nothing is booked once it is overdue", () => {
    render(
      <OpenWithdrawalCard
        row={row({ firstBookinglessDay: "2026-09-01", urgency: "overdue" })}
        finish={vi.fn()}
        remove={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Details anzeigen/ }));

    expect(
      screen.getByText(
        "Seit dem 01.09.2026 ist kein Betreuungstag mehr gebucht. Tragen Sie das Ende jetzt mit „Betreuung beenden“ ein.",
      ),
    ).toBeInTheDocument();
  });
});
