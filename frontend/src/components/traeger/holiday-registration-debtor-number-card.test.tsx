import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";

const { mockChange } = vi.hoisted(() => ({ mockChange: vi.fn() }));

vi.mock("~/lib/traeger-contributions-api", () => ({
  changeHolidayDebtorNumber: (
    periodId: string,
    registrationId: string,
    debtorNumber: string,
  ) => mockChange(periodId, registrationId, debtorNumber),
}));

import { HolidayRegistrationDebtorNumberCard } from "./holiday-registration-debtor-number-card";

function renderCard(debtorNumber: string, required = true) {
  const onChanged = vi.fn(() => Promise.resolve());
  render(
    <ToastProvider>
      <HolidayRegistrationDebtorNumberCard
        periodId="5"
        registrationId="7"
        debtorNumber={debtorNumber}
        required={required}
        onChanged={onChanged}
      />
    </ToastProvider>,
  );
  return { onChanged };
}

describe("HolidayRegistrationDebtorNumberCard (#5621)", () => {
  beforeEach(() => {
    mockChange.mockReset();
  });

  it("enters a missing number and reads the registration again", async () => {
    mockChange.mockResolvedValue("10 4711");
    const { onChanged } = renderCard("");

    expect(screen.getByText(/Noch keine Nummer\./)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));
    const field = await screen.findByRole("textbox", {
      name: /Debitorennummer/,
    });
    fireEvent.change(field, { target: { value: " 10 4711 " } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockChange).toHaveBeenCalledWith("5", "7", "10 4711"),
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it("keeps a required number and marks the empty field", async () => {
    renderCard("10 4711");

    expect(screen.getByText("10 4711")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ändern" }));
    const field = await screen.findByRole("textbox", {
      name: /Debitorennummer/,
    });
    expect(field).toHaveValue("10 4711");
    fireEvent.change(field, { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte geben Sie die Debitorennummer ein."),
    ).toBeInTheDocument();
    expect(field).toHaveAttribute("aria-invalid", "true");
    expect(mockChange).not.toHaveBeenCalled();
  });

  it("says that an issued invoice keeps the number", async () => {
    mockChange.mockImplementation(() =>
      Promise.reject(
        new ApiError("locked", 409, {
          code: "parent_billing.debtor_number_locked",
        }),
      ),
    );
    renderCard("10 4711");

    fireEvent.click(screen.getByRole("button", { name: "Ändern" }));
    fireEvent.change(
      await screen.findByRole("textbox", { name: /Debitorennummer/ }),
      {
        target: { value: "10 4712" },
      },
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Für diese Anmeldung gibt es schon eine Rechnung. Die Debitorennummer lässt sich nicht mehr ändern.",
      ),
    ).toBeInTheDocument();
  });
});
