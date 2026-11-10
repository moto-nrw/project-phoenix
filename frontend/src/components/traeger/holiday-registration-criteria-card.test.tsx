import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { HolidayRegistrationDetail } from "~/lib/traeger-holiday-registrations-api";
import { HOLIDAY_REGISTRATION } from "~/test/fixtures/holiday-registrations";

const { check } = vi.hoisted(() => ({ check: vi.fn() }));
vi.mock("~/lib/traeger-holiday-registrations-api", async (original) => ({
  ...(await original<
    typeof import("~/lib/traeger-holiday-registrations-api")
  >()),
  checkHolidayRegistrationCriteria: (...args: unknown[]) => check(...args),
}));

import { HolidayRegistrationCriteriaCard } from "./holiday-registration-criteria-card";

const REGISTRATION: HolidayRegistrationDetail = {
  ...HOLIDAY_REGISTRATION,
  criteria: [
    {
      criterionId: "1",
      name: "Beide Eltern berufstätig",
      question: "Sind beide Eltern berufstätig?",
      scope: "family",
      childId: null,
      stated: "yes",
      checked: null,
      checkedAt: null,
    },
    {
      criterionId: "2",
      name: "Geschwisterkind ist schon angemeldet",
      question: "Ist ein Geschwisterkind angemeldet?",
      scope: "child",
      childId: "9",
      stated: "unknown",
      checked: null,
      checkedAt: null,
    },
  ],
};

describe("criteria of a registration (#4700)", () => {
  it("shows what the parents said and sends only the changed checks", async () => {
    const onChanged = vi.fn();
    check.mockResolvedValue(REGISTRATION);
    render(
      <ToastProvider>
        <HolidayRegistrationCriteriaCard
          periodId="5"
          registration={REGISTRATION}
          onChanged={onChanged}
        />
      </ToastProvider>,
    );
    expect(screen.getByText("Familie")).toBeInTheDocument();
    expect(screen.getByText("Lia Muster")).toBeInTheDocument();
    expect(screen.getByText("Zählt")).toBeInTheDocument();
    expect(screen.getByText("Ungeklärt")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Angaben für die Vergabe prüfen" }),
    );
    expect(
      screen.getByRole("button", { name: "Prüfung speichern" }),
    ).toBeDisabled();
    const childGroup = screen.getByRole("group", {
      name: "Prüfung: Geschwisterkind ist schon angemeldet, Lia Muster",
    });
    fireEvent.click(
      Array.from(childGroup.querySelectorAll("button")).find(
        (button) => button.textContent === "Trifft zu",
      )!,
    );
    fireEvent.click(screen.getByRole("button", { name: "Prüfung speichern" }));
    await waitFor(() =>
      expect(check).toHaveBeenCalledWith("5", "7", [
        { criterionId: "2", childId: "9", checked: "yes" },
      ]),
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(REGISTRATION));
  });

  it("locks the checks while they are saved", async () => {
    let finish: (value: HolidayRegistrationDetail) => void = () => {};
    check.mockReturnValue(
      new Promise<HolidayRegistrationDetail>((resolve) => {
        finish = resolve;
      }),
    );
    render(
      <ToastProvider>
        <HolidayRegistrationCriteriaCard
          periodId="5"
          registration={REGISTRATION}
          onChanged={vi.fn()}
        />
      </ToastProvider>,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Angaben für die Vergabe prüfen" }),
    );
    const familyGroup = screen.getByRole("group", {
      name: "Prüfung: Beide Eltern berufstätig",
    });
    const yes = Array.from(familyGroup.querySelectorAll("button")).find(
      (button) => button.textContent === "Trifft zu",
    )!;
    fireEvent.click(yes);
    fireEvent.click(screen.getByRole("button", { name: "Prüfung speichern" }));
    await waitFor(() => expect(check).toHaveBeenCalled());
    for (const button of familyGroup.querySelectorAll("button")) {
      expect(button).toBeDisabled();
    }
    finish(REGISTRATION);
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Angaben für die Vergabe prüfen" }),
      ).toBeInTheDocument(),
    );
  });

  it("shows no card for a period by arrival", () => {
    const view = (registration: HolidayRegistrationDetail) => (
      <ToastProvider>
        <HolidayRegistrationCriteriaCard
          periodId="5"
          registration={registration}
          onChanged={vi.fn()}
        />
      </ToastProvider>
    );
    const { rerender } = render(view(REGISTRATION));
    // Anker: mit Kriterien steht die Karte da.
    expect(screen.getByText("Angaben für die Vergabe")).toBeInTheDocument();
    rerender(view({ ...REGISTRATION, criteria: [] }));
    expect(
      screen.queryByText("Angaben für die Vergabe"),
    ).not.toBeInTheDocument();
  });
});
