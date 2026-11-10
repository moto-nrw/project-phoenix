import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { HolidayRegistrationChild } from "~/lib/traeger-holiday-registrations-api";
import { HOLIDAY_REGISTRATION } from "~/test/fixtures/holiday-registrations";

const { mockSetWish } = vi.hoisted(() => ({ mockSetWish: vi.fn() }));

vi.mock("~/lib/traeger-holiday-registrations-api", () => ({
  setHolidayChildWish: (...args: unknown[]) => mockSetWish(...args),
}));

import { HolidayWishSection, holidayWishView } from "./holiday-wish";

const lia = HOLIDAY_REGISTRATION.children[0]!;
const guest = HOLIDAY_REGISTRATION.children[1]!;
const unknown: HolidayRegistrationChild = { ...lia, holidayWishCheck: true };
const declined: HolidayRegistrationChild = {
  ...lia,
  holidayWish: "no",
  holidayWishSource: "account",
};

function renderSection(child: HolidayRegistrationChild, wishRule = true) {
  const onChanged = vi.fn();
  render(
    <ToastProvider>
      <HolidayWishSection
        periodId="5"
        registrationId={HOLIDAY_REGISTRATION.id}
        child={child}
        wishRule={wishRule}
        cancelled={false}
        onChanged={onChanged}
      />
    </ToastProvider>,
  );
  return onChanged;
}

describe("holiday care wish (#4703)", () => {
  beforeEach(() => mockSetWish.mockReset());

  it("marks only children of the Träger's schools in a period with the rule", () => {
    expect(holidayWishView(unknown, true)?.label).toBe("Ferienwunsch prüfen");
    expect(holidayWishView(declined, true)?.label).toBe("Ohne Ferienwunsch");
    expect(holidayWishView(unknown, false)).toBeNull();
    expect(
      holidayWishView({ ...guest, holidayWishCheck: true }, true),
    ).toBeNull();
  });

  it("lets the office decide an unknown wish", async () => {
    mockSetWish.mockResolvedValue(HOLIDAY_REGISTRATION);
    const onChanged = renderSection(unknown);

    expect(screen.getByText("Ferienwunsch prüfen")).toBeInTheDocument();
    expect(screen.getByText(/ohne Elternkonto angemeldet/)).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Lia Muster: Mit Ferienwunsch" }),
    );

    await waitFor(() =>
      expect(mockSetWish).toHaveBeenCalledWith(
        "5",
        HOLIDAY_REGISTRATION.id,
        lia.id,
        "yes",
      ),
    );
    await waitFor(() =>
      expect(onChanged).toHaveBeenCalledWith(HOLIDAY_REGISTRATION),
    );
  });

  it("explains the place of a child without a wish and offers the change", () => {
    renderSection(declined);

    expect(
      screen.getByText(/Aus der OGS-Anmeldung, geprüft über das Elternkonto/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/wartet das Kind hinter Kindern/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Lia Muster: Mit Ferienwunsch" }),
    ).toHaveTextContent("Auf „Mit Ferienwunsch“ ändern");
  });

  it("shows nothing for a period without the rule", () => {
    const { container } = render(
      <ToastProvider>
        <p>Karte</p>
        <HolidayWishSection
          periodId="5"
          registrationId={HOLIDAY_REGISTRATION.id}
          child={unknown}
          wishRule={false}
          cancelled={false}
          onChanged={vi.fn()}
        />
      </ToastProvider>,
    );
    expect(screen.getByText("Karte")).toBeInTheDocument();
    expect(container).not.toHaveTextContent("Ferienwunsch");
  });
});
