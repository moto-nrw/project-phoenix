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

const {
  mockChange,
  mockConfirm,
  mockConfirmDebtorNumber,
  mockChangeDebtorNumber,
} = vi.hoisted(() => ({
  mockChange: vi.fn(),
  mockConfirm: vi.fn(),
  mockConfirmDebtorNumber: vi.fn(),
  mockChangeDebtorNumber: vi.fn(),
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
    debtorNumber?: string,
    costBearerId?: string,
  ) => {
    mockConfirmDebtorNumber(debtorNumber);
    return costBearerId === undefined
      ? mockConfirm(periodId, id, benefitsExemption)
      : mockConfirm(periodId, id, benefitsExemption, costBearerId);
  },
  changeHolidayDebtorNumber: (
    periodId: string,
    id: string,
    debtorNumber: string,
  ) => mockChangeDebtorNumber(periodId, id, debtorNumber),
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
        onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
      />
    </ToastProvider>,
  );
  return { onChanged, onConfirmed, onExemptionChange };
}

describe("HolidayRegistrationStatusCard", () => {
  beforeEach(() => {
    mockChange.mockReset();
    mockConfirm.mockReset();
    mockConfirmDebtorNumber.mockReset();
    mockChangeDebtorNumber.mockReset();
  });

  it("confirms a received registration through the contribution and says parents get no message yet", async () => {
    mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
    const { onConfirmed, onExemptionChange } = renderCard(HOLIDAY_REGISTRATION);

    expect(
      screen.getByText(
        "Eltern bekommen beim Bestätigen noch keine Nachricht von moto.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /^Die Familie bezahlt/ }),
    ).toBeChecked();
    const exemptionChoice = screen.getByRole("radio", {
      name: /Von der Zahlung befreien \(Leistungsbezug\)/,
    });
    expect(exemptionChoice).not.toBeChecked();
    fireEvent.click(exemptionChoice);
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
      screen.getByRole("radio", {
        name: /Von der Zahlung befreien \(Leistungsbezug\)/,
      }),
    ).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() =>
      expect(mockConfirm).toHaveBeenCalledWith("5", "7", true),
    );
  });

  it("waits with the confirmation until the preview is computed again", () => {
    render(
      <ToastProvider>
        <HolidayRegistrationStatusCard
          periodId="5"
          registration={HOLIDAY_REGISTRATION}
          contribution={HOLIDAY_CONTRIBUTION}
          previewPending
          exemption
          onExemptionChange={vi.fn()}
          onChanged={vi.fn()}
          onConfirmed={vi.fn(() => Promise.resolve())}
          onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
        />
      </ToastProvider>,
    );

    expect(screen.getByRole("button", { name: "Bestätigen" })).toBeDisabled();
    expect(
      screen.getByRole("radio", {
        name: /Von der Zahlung befreien \(Leistungsbezug\)/,
      }),
    ).toBeChecked();
  });

  it("lets a cost bearer pay instead of the family and confirms with it", async () => {
    mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
    const onCostBearerChange = vi.fn();
    const onExemptionChange = vi.fn();
    const props = {
      periodId: "5",
      registration: { ...HOLIDAY_REGISTRATION, benefitsDeclared: true },
      contribution: HOLIDAY_CONTRIBUTION,
      exemption: true,
      onExemptionChange,
      costBearers: [
        { id: "3", name: "Fachdienst Jugend des Landkreises Musterstadt" },
        { id: "4", name: "Sozialamt Musterstadt" },
      ],
      onCostBearerChange,
      onChanged: vi.fn(),
      onConfirmed: vi.fn(() => Promise.resolve()),
      onDebtorNumberChanged: vi.fn(() => Promise.resolve()),
    };
    const { rerender } = render(
      <ToastProvider>
        <HolidayRegistrationStatusCard {...props} costBearerId={null} />
      </ToastProvider>,
    );

    expect(screen.getAllByRole("radio")).toHaveLength(3);
    fireEvent.click(
      screen.getByRole("radio", { name: /Kosten übernimmt ein Kostenträger/ }),
    );
    expect(onCostBearerChange).toHaveBeenCalledWith("3");

    rerender(
      <ToastProvider>
        <HolidayRegistrationStatusCard {...props} costBearerId="4" />
      </ToastProvider>,
    );
    expect(
      screen.getByRole("radio", { name: /Kosten übernimmt ein Kostenträger/ }),
    ).toBeChecked();
    expect(
      screen.getByRole("radio", { name: /Von der Zahlung befreien/ }),
    ).not.toBeChecked();
    expect(screen.getByText("Sozialamt Musterstadt")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
    await waitFor(() =>
      expect(mockConfirm).toHaveBeenCalledWith("5", "7", false, "4"),
    );

    fireEvent.click(
      screen.getByRole("radio", { name: /^Die Familie bezahlt/ }),
    );
    expect(onCostBearerChange).toHaveBeenLastCalledWith(null);
    expect(onExemptionChange).toHaveBeenLastCalledWith(false);
  });

  it("names the missing rule when the parents state benefits", () => {
    renderCard({ ...HOLIDAY_REGISTRATION, benefitsDeclared: true }, vi.fn(), {
      ...HOLIDAY_CONTRIBUTION,
      exemptionAvailable: false,
    });

    expect(
      screen.getByText(
        "Die Eltern geben an, dass sie Leistungen beziehen. Für eine Befreiung braucht es eine Regel unter „Ermäßigungen“.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bestätigen" })).toBeEnabled();
    expect(screen.queryByRole("radio")).toBeNull();
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

  describe("debtor number (#5621)", () => {
    function renderWithSwitch(debtorNumberRequired: boolean) {
      render(
        <ToastProvider>
          <HolidayRegistrationStatusCard
            periodId="5"
            registration={HOLIDAY_REGISTRATION}
            contribution={HOLIDAY_CONTRIBUTION}
            debtorNumberRequired={debtorNumberRequired}
            exemption={false}
            onExemptionChange={vi.fn()}
            onChanged={vi.fn()}
            onConfirmed={vi.fn(() => Promise.resolve())}
            onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
          />
        </ToastProvider>,
      );
    }

    it("asks for no debtor number while the switch is off", async () => {
      mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
      renderWithSwitch(false);

      expect(screen.getByRole("button", { name: "Bestätigen" })).toBeEnabled();
      expect(screen.queryByLabelText(/Debitorennummer/)).toBeNull();
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
      await waitFor(() =>
        expect(mockConfirm).toHaveBeenCalledWith("5", "7", false),
      );
      expect(mockConfirmDebtorNumber).toHaveBeenCalledWith("");
    });

    it("requires the debtor number while the switch is on and marks the field", async () => {
      renderWithSwitch(true);

      const field = screen.getByLabelText(/Debitorennummer/);
      expect(field).toBeRequired();
      expect(field).toHaveAccessibleDescription(
        "Die Nummer der Familie bei der Kasse der Gemeinde. Die Familie gibt sie bei der Überweisung an.",
      );
      fireEvent.change(field, { target: { value: "   " } });
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

      expect(
        await screen.findByText("Bitte geben Sie die Debitorennummer ein."),
      ).toBeInTheDocument();
      expect(field).toHaveAttribute("aria-invalid", "true");
      await waitFor(() => expect(field).toHaveFocus());
      expect(mockConfirm).not.toHaveBeenCalled();
    });

    it("confirms with the trimmed debtor number", async () => {
      mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
      renderWithSwitch(true);

      fireEvent.change(screen.getByLabelText(/Debitorennummer/), {
        target: { value: " 10 4711 " },
      });
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

      await waitFor(() =>
        expect(mockConfirm).toHaveBeenCalledWith("5", "7", false),
      );
      expect(mockConfirmDebtorNumber).toHaveBeenCalledWith("10 4711");
    });

    it("puts the server's refusal at the field and focuses it", async () => {
      mockConfirm.mockImplementation(() =>
        Promise.reject(
          new ApiError("invalid", 400, {
            code: "parent_billing.debtor_number_invalid",
            errors: [{ field: "debtor_number", reason: "must be one line" }],
          }),
        ),
      );
      renderWithSwitch(true);
      const field = screen.getByLabelText(/Debitorennummer/);
      fireEvent.change(field, { target: { value: "10 4711" } });
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

      await waitFor(() =>
        expect(field).toHaveAttribute("aria-invalid", "true"),
      );
      await waitFor(() => expect(field).toHaveFocus());
      expect(
        screen.getByText("Bitte prüfen Sie die Debitorennummer."),
      ).toBeInTheDocument();
    });

    it("keeps Enter in the field from confirming while the preview is computed again", () => {
      render(
        <ToastProvider>
          <HolidayRegistrationStatusCard
            periodId="5"
            registration={HOLIDAY_REGISTRATION}
            contribution={HOLIDAY_CONTRIBUTION}
            previewPending
            debtorNumberRequired
            exemption={false}
            onExemptionChange={vi.fn()}
            onChanged={vi.fn()}
            onConfirmed={vi.fn(() => Promise.resolve())}
            onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
          />
        </ToastProvider>,
      );
      const field = screen.getByLabelText(/Debitorennummer/);
      fireEvent.change(field, { target: { value: "10 4711" } });
      fireEvent.submit(field.closest("form")!);

      expect(screen.getByRole("button", { name: "Bestätigen" })).toBeDisabled();
      expect(mockConfirm).not.toHaveBeenCalled();
    });

    it("sends one confirmation however often Enter is pressed", async () => {
      let finish: (value: HolidayContribution) => void = () => undefined;
      mockConfirm.mockImplementation(
        () =>
          new Promise<HolidayContribution>((resolve) => {
            finish = resolve;
          }),
      );
      renderWithSwitch(true);
      const field = screen.getByLabelText(/Debitorennummer/);
      fireEvent.change(field, { target: { value: "10 4711" } });
      const form = field.closest("form")!;
      fireEvent.submit(form);
      fireEvent.submit(form);
      fireEvent.submit(form);

      await waitFor(() => expect(mockConfirm).toHaveBeenCalledTimes(1));
      finish({ ...HOLIDAY_CONTRIBUTION, frozen: true });
      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Bestätigen" }),
        ).toBeEnabled(),
      );
      expect(mockConfirm).toHaveBeenCalledTimes(1);
    });
  });

  describe("debtor number of a confirmed registration (#5621)", () => {
    const CONFIRMED: HolidayRegistrationDetail = {
      ...HOLIDAY_REGISTRATION,
      status: "confirmed",
    };

    function renderConfirmed(
      debtorNumber: string,
      required = true,
      invoiced = false,
    ) {
      const onDebtorNumberChanged = vi.fn(() => Promise.resolve());
      render(
        <ToastProvider>
          <HolidayRegistrationStatusCard
            periodId="5"
            registration={{ ...CONFIRMED, debtorNumber }}
            contribution={{ ...HOLIDAY_CONTRIBUTION, invoiced }}
            debtorNumberRequired={required}
            exemption={false}
            onExemptionChange={vi.fn()}
            onChanged={vi.fn()}
            onConfirmed={vi.fn(() => Promise.resolve())}
            onDebtorNumberChanged={onDebtorNumberChanged}
          />
        </ToastProvider>,
      );
      return { onDebtorNumberChanged };
    }

    it("enters a missing number in place and reads the registration again", async () => {
      mockChangeDebtorNumber.mockResolvedValue("10 4711");
      const { onDebtorNumberChanged } = renderConfirmed("");

      expect(screen.getByText("Noch keine Nummer.")).toBeInTheDocument();
      expect(
        screen.getByText(
          /Ohne Nummer steht im Schreiben die Rechnungsnummer als Verwendungszweck\./,
        ),
      ).toBeInTheDocument();
      fireEvent.click(
        screen.getByRole("button", { name: "Debitorennummer eintragen" }),
      );
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      const field = screen.getByRole("textbox", { name: /Debitorennummer/ });
      expect(field).toHaveFocus();
      expect(field).toBeRequired();
      expect(field).toHaveAttribute("maxLength", "35");
      expect(field).toHaveAccessibleDescription(
        "Die Nummer der Familie bei der Kasse der Gemeinde. Die Familie gibt sie bei der Überweisung an.",
      );
      fireEvent.change(field, { target: { value: " 10 4711 " } });
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() =>
        expect(mockChangeDebtorNumber).toHaveBeenCalledWith(
          "5",
          "7",
          "10 4711",
        ),
      );
      await waitFor(() => expect(onDebtorNumberChanged).toHaveBeenCalled());
      expect(
        await screen.findByText("Die Debitorennummer ist gespeichert."),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("textbox", { name: /Debitorennummer/ }),
      ).not.toBeInTheDocument();
    });

    it("keeps a required number and marks the empty field", async () => {
      renderConfirmed("10 4711");

      expect(screen.getByText("10 4711")).toBeInTheDocument();
      fireEvent.click(
        screen.getByRole("button", { name: "Debitorennummer ändern" }),
      );
      const field = screen.getByRole("textbox", { name: /Debitorennummer/ });
      expect(field).toHaveValue("10 4711");
      fireEvent.change(field, { target: { value: "" } });
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      expect(
        await screen.findByText("Bitte geben Sie die Debitorennummer ein."),
      ).toBeInTheDocument();
      expect(field).toHaveAttribute("aria-invalid", "true");
      expect(mockChangeDebtorNumber).not.toHaveBeenCalled();
    });

    it("discards the draft on Abbrechen", () => {
      renderConfirmed("10 4711");

      fireEvent.click(
        screen.getByRole("button", { name: "Debitorennummer ändern" }),
      );
      fireEvent.change(
        screen.getByRole("textbox", { name: /Debitorennummer/ }),
        { target: { value: "10 4712" } },
      );
      fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

      expect(screen.getByText("10 4711")).toBeInTheDocument();
      expect(
        screen.queryByRole("textbox", { name: /Debitorennummer/ }),
      ).not.toBeInTheDocument();
      expect(mockChangeDebtorNumber).not.toHaveBeenCalled();
    });

    it("says that an issued invoice keeps the number", async () => {
      mockChangeDebtorNumber.mockImplementation(() =>
        Promise.reject(
          new ApiError("locked", 409, {
            code: "parent_billing.debtor_number_locked",
          }),
        ),
      );
      renderConfirmed("10 4711");

      fireEvent.click(
        screen.getByRole("button", { name: "Debitorennummer ändern" }),
      );
      fireEvent.change(
        screen.getByRole("textbox", { name: /Debitorennummer/ }),
        { target: { value: "10 4712" } },
      );
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      expect(
        await screen.findByText(
          "Für diese Anmeldung gibt es schon eine Rechnung. Die Debitorennummer lässt sich nicht mehr ändern.",
        ),
      ).toBeInTheDocument();
    });

    it("offers no change once an issued invoice keeps the number", () => {
      renderConfirmed("10 4711", true, true);

      expect(
        screen.getByText(
          "Die Rechnung ist ausgestellt. Sie behält diese Nummer. Ändern geht nicht mehr.",
        ),
      ).toBeInTheDocument();
      expect(screen.getByText("10 4711")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Einfrieren" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /Debitorennummer/ }),
      ).not.toBeInTheDocument();
    });

    it("shows no debtor number while the switch is off and none is set", () => {
      renderConfirmed("", false);

      expect(
        screen.getByRole("button", { name: "Einfrieren" }),
      ).toBeInTheDocument();
      expect(screen.queryByText(/Debitorennummer/)).not.toBeInTheDocument();
    });
  });
});
