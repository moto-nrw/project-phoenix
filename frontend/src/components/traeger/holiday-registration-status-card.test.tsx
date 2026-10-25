import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { HolidayRegistrationDetail } from "~/lib/traeger-holiday-registrations-api";
import type { HolidayContribution } from "~/lib/traeger-contributions-api";
import {
  HOLIDAY_CONTRIBUTION,
  HOLIDAY_REGISTRATION,
} from "~/test/fixtures/holiday-registrations";

const { mockChange, mockConfirm } = vi.hoisted(() => ({
  mockChange: vi.fn(),
  mockConfirm: vi.fn(),
}));

vi.mock("~/lib/traeger-holiday-registrations-api", () => ({
  changeHolidayRegistration: (periodId: string, id: string, action: string) =>
    mockChange(periodId, id, action),
}));

vi.mock("~/lib/traeger-contributions-api", () => ({
  confirmHolidayRegistration: (
    periodId: string,
    id: string,
    benefitsExemption: boolean,
  ) => mockConfirm(periodId, id, benefitsExemption),
}));

import { HolidayRegistrationStatusCard } from "./holiday-registration-status-card";

function renderCard(
  registration: HolidayRegistrationDetail,
  onChanged = vi.fn(),
  contribution: HolidayContribution = HOLIDAY_CONTRIBUTION,
  onConfirmed = vi.fn(() => Promise.resolve()),
) {
  const onExemptionChange = vi.fn();
  render(
    <ToastProvider>
      <HolidayRegistrationStatusCard
        periodId="5"
        registration={registration}
        contribution={contribution}
        exemption={registration.benefitsDeclared}
        onExemptionChange={onExemptionChange}
        onChanged={onChanged}
        onConfirmed={onConfirmed}
      />
    </ToastProvider>,
  );
  return { onChanged, onConfirmed, onExemptionChange };
}

describe("HolidayRegistrationStatusCard", () => {
  beforeEach(() => {
    mockChange.mockReset();
    mockConfirm.mockReset();
  });

  it("confirms a received registration through the contribution and says parents get no message yet", async () => {
    mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
    const { onConfirmed, onExemptionChange } = renderCard(HOLIDAY_REGISTRATION);

    expect(
      screen.getByText(
        "Eltern bekommen beim Bestätigen noch keine Nachricht von moto.",
      ),
    ).toBeInTheDocument();
    const checkbox = screen.getByRole("checkbox", {
      name: "Beim Bestätigen von der Zahlung befreien (Leistungsbezug)",
    });
    expect(checkbox).not.toBeChecked();
    fireEvent.click(checkbox);
    expect(onExemptionChange).toHaveBeenCalledWith(true);
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => expect(onConfirmed).toHaveBeenCalled());
    expect(mockConfirm).toHaveBeenCalledWith("5", "7", false);
    expect(mockChange).not.toHaveBeenCalled();
  });

  it("offers the exemption the parents stated and confirms with it", async () => {
    mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
    renderCard({ ...HOLIDAY_REGISTRATION, benefitsDeclared: true });

    expect(
      screen.getByText(
        "Die Eltern geben an, dass sie Leistungen beziehen. Bitte prüfen Sie den Nachweis.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", {
        name: "Beim Bestätigen von der Zahlung befreien (Leistungsbezug)",
      }),
    ).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() =>
      expect(mockConfirm).toHaveBeenCalledWith("5", "7", true),
    );
  });

  it("names the missing rule when the parents state benefits", () => {
    renderCard({ ...HOLIDAY_REGISTRATION, benefitsDeclared: true }, vi.fn(), {
      ...HOLIDAY_CONTRIBUTION,
      exemptionAvailable: false,
    });

    expect(
      screen.getByText(
        "Die Eltern geben an, dass sie Leistungen beziehen. Für eine Befreiung braucht es eine Regel unter „Beiträge“.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bestätigen" })).toBeEnabled();
    expect(screen.queryByRole("checkbox")).toBeNull();
  });

  it("cancels only after the confirmation", async () => {
    mockChange.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      status: "cancelled",
    });
    renderCard(HOLIDAY_REGISTRATION);

    fireEvent.click(screen.getByRole("button", { name: "Stornieren" }));
    expect(mockChange).not.toHaveBeenCalled();
    expect(
      await screen.findByText("Anmeldung stornieren?"),
    ).toBeInTheDocument();
    const buttons = screen.getAllByRole("button", { name: "Stornieren" });
    fireEvent.click(buttons[buttons.length - 1]!);

    await waitFor(() =>
      expect(mockChange).toHaveBeenCalledWith("5", "7", "cancel"),
    );
  });

  it("offers only reopening for a frozen registration and names why", () => {
    renderCard({ ...HOLIDAY_REGISTRATION, status: "confirmed", frozen: true });

    expect(screen.getByText(/Sie ist eingefroren\./)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Wieder öffnen" }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("button")).toHaveLength(1);
  });

  it("freezes an open registration", async () => {
    mockChange.mockResolvedValue({ ...HOLIDAY_REGISTRATION, frozen: true });
    renderCard(HOLIDAY_REGISTRATION);

    fireEvent.click(screen.getByRole("button", { name: "Einfrieren" }));

    await waitFor(() =>
      expect(mockChange).toHaveBeenCalledWith("5", "7", "freeze"),
    );
  });

  it("leaves a cancelled registration without actions", () => {
    renderCard({ ...HOLIDAY_REGISTRATION, status: "cancelled" });

    expect(
      screen.getByText(
        "Diese Anmeldung ist storniert. Sie lässt sich nicht mehr ändern.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });

  it("shows a refused change as an error message", async () => {
    mockConfirm.mockImplementation(() =>
      Promise.reject(
        new ApiError("frozen", 409, {
          code: "enrollment.holiday_registration_frozen",
        }),
      ),
    );
    renderCard(HOLIDAY_REGISTRATION);

    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
    expect(
      await screen.findByText(
        "Die Anmeldung ist eingefroren. Wählen Sie zuerst „Wieder öffnen“.",
      ),
    ).toBeInTheDocument();
  });
});
