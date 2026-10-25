import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { HolidayRegistrationDetail } from "~/lib/traeger-holiday-registrations-api";
import { HOLIDAY_REGISTRATION } from "~/test/fixtures/holiday-registrations";

const { mockChange } = vi.hoisted(() => ({ mockChange: vi.fn() }));

vi.mock("~/lib/traeger-holiday-registrations-api", () => ({
  changeHolidayRegistration: (periodId: string, id: string, action: string) =>
    mockChange(periodId, id, action),
}));

import { HolidayRegistrationStatusCard } from "./holiday-registration-status-card";

function renderCard(
  registration: HolidayRegistrationDetail,
  onChanged = vi.fn(),
) {
  render(
    <ToastProvider>
      <HolidayRegistrationStatusCard
        periodId="5"
        registration={registration}
        onChanged={onChanged}
      />
    </ToastProvider>,
  );
  return onChanged;
}

describe("HolidayRegistrationStatusCard", () => {
  beforeEach(() => {
    mockChange.mockReset();
  });

  it("confirms a received registration and says parents get no message yet", async () => {
    const confirmed = { ...HOLIDAY_REGISTRATION, status: "confirmed" as const };
    mockChange.mockResolvedValue(confirmed);
    const onChanged = renderCard(HOLIDAY_REGISTRATION);

    expect(
      screen.getByText(
        "Eltern bekommen beim Bestätigen noch keine Nachricht von moto.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(confirmed));
    expect(mockChange).toHaveBeenCalledWith("5", "7", "confirm");
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
    mockChange.mockImplementation(() =>
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
