import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, onTestFinished, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { _resetTraegerHostCache } from "~/lib/traeger-url";
import type { HolidayRegistrationDetail } from "~/lib/traeger-holiday-registrations-api";
import type { HolidayContribution } from "~/lib/traeger-contributions-api";
import {
  HOLIDAY_CONTRIBUTION,
  HOLIDAY_REGISTRATION,
} from "~/test/fixtures/holiday-registrations";

const {
  mockChange,
  mockCancel,
  mockConfirm,
  mockConfirmDebtorNumber,
  mockChangeDebtorNumber,
  mockFamilyChoice,
} = vi.hoisted(() => ({
  mockChange: vi.fn(),
  mockCancel: vi.fn(),
  mockConfirm: vi.fn(),
  mockConfirmDebtorNumber: vi.fn(),
  mockChangeDebtorNumber: vi.fn(),
  mockFamilyChoice: vi.fn(),
}));

vi.mock("~/lib/traeger-holiday-registrations-api", () => ({
  changeHolidayRegistration: (periodId: string, id: string, action: string) =>
    mockChange(periodId, id, action),
}));

vi.mock("~/lib/traeger-contributions-api", async (importOriginal) => ({
  // The refusal "the family has another number" is read as in production.
  familyDebtorNumberConflictOf: (
    await importOriginal<typeof import("~/lib/traeger-contributions-api")>()
  ).familyDebtorNumberConflictOf,
  cancelHolidayRegistration: (periodId: string, id: string) =>
    mockCancel(periodId, id),
  confirmHolidayRegistration: (
    periodId: string,
    id: string,
    benefitsExemption: boolean,
    debtorNumber?: string,
    costBearerId?: string,
    familyChoice?: string,
  ) => {
    mockConfirmDebtorNumber(debtorNumber);
    mockFamilyChoice(familyChoice);
    return costBearerId === undefined
      ? mockConfirm(periodId, id, benefitsExemption)
      : mockConfirm(periodId, id, benefitsExemption, costBearerId);
  },
  changeHolidayDebtorNumber: (
    periodId: string,
    id: string,
    debtorNumber: string,
    familyChoice?: string,
  ) => {
    mockFamilyChoice(familyChoice);
    return mockChangeDebtorNumber(periodId, id, debtorNumber);
  },
}));

import { HolidayRegistrationStatusCard } from "./holiday-registration-status-card";

function renderCard(
  registration: HolidayRegistrationDetail,
  onChanged = vi.fn(),
  contribution: HolidayContribution = HOLIDAY_CONTRIBUTION,
  onReload = vi.fn(() => Promise.resolve()),
  canManageBillingSettings = false,
) {
  const onExemptionChange = vi.fn();
  render(
    <ToastProvider>
      <HolidayRegistrationStatusCard
        periodId="5"
        registration={registration}
        contribution={contribution}
        canManageBillingSettings={canManageBillingSettings}
        exemption={registration.benefitsDeclared}
        onExemptionChange={onExemptionChange}
        onChanged={onChanged}
        onReload={onReload}
        onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
      />
    </ToastProvider>,
  );
  return { onChanged, onReload, onExemptionChange };
}

describe("HolidayRegistrationStatusCard", () => {
  beforeEach(() => {
    // The hint at Bestätigen links to the period (#6672).
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", window.location.host);
    _resetTraegerHostCache();
    onTestFinished(() => {
      vi.unstubAllEnvs();
      _resetTraegerHostCache();
    });
    mockChange.mockReset();
    mockCancel.mockReset();
    mockConfirm.mockReset();
    mockConfirmDebtorNumber.mockReset();
    mockChangeDebtorNumber.mockReset();
    mockFamilyChoice.mockReset();
  });

  it("confirms a received registration through the contribution and says parents get the acceptance mail", async () => {
    mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
    const { onReload, onExemptionChange } = renderCard(HOLIDAY_REGISTRATION);

    expect(
      screen.getByText(/Beim Bestätigen bekommen die Eltern eine E-Mail/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Text der Annahme-Mail" }),
    ).toHaveAttribute("href", "/ferien/5");
    expect(
      screen.getByRole("radio", { name: /^Die Familie bezahlt/ }),
    ).toBeChecked();
    const exemptionChoice = screen.getByRole("radio", {
      name: /Von der Zahlung befreien \(Leistungsbezug\)/,
    });
    expect(
      screen.getByText(
        "Dann gilt die Befreiungsregel. Niemand bezahlt den Betrag.",
      ),
    ).toBeInTheDocument();
    expect(exemptionChoice).not.toBeChecked();
    fireEvent.click(exemptionChoice);
    expect(onExemptionChange).toHaveBeenCalledWith(true);
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => expect(onReload).toHaveBeenCalled());
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
          onReload={vi.fn(() => Promise.resolve())}
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
      onReload: vi.fn(() => Promise.resolve()),
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
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", window.location.host);
    _resetTraegerHostCache();
    onTestFinished(() => {
      vi.unstubAllEnvs();
      _resetTraegerHostCache();
    });
    renderCard(
      { ...HOLIDAY_REGISTRATION, benefitsDeclared: true },
      vi.fn(),
      {
        ...HOLIDAY_CONTRIBUTION,
        exemptionAvailable: false,
      },
      undefined,
      true,
    );

    expect(
      screen.getByText(
        /Die Eltern geben an, dass sie Leistungen beziehen\. Am ersten gebuchten Tag gilt keine Befreiungsregel\. Sie legen sie unter/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Abrechnungsvorgaben" }),
    ).toHaveAttribute("href", "/einstellungen/abrechnung");
    expect(screen.getByRole("button", { name: "Bestätigen" })).toBeEnabled();
    expect(screen.queryByRole("radio")).toBeNull();
  });

  it("names the missing rule without a link when billing settings are unavailable", () => {
    renderCard({ ...HOLIDAY_REGISTRATION, benefitsDeclared: true }, vi.fn(), {
      ...HOLIDAY_CONTRIBUTION,
      exemptionAvailable: false,
    });

    // Anker: Der Hinweis zur fehlenden Befreiungsregel ist sichtbar.
    expect(
      screen.getByText(
        "Die Eltern geben an, dass sie Leistungen beziehen. Am ersten gebuchten Tag gilt keine Befreiungsregel. Die Rolle „Abrechnung“ legt sie an.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Abrechnungsvorgaben" }),
    ).not.toBeInTheDocument();
  });

  it("cancels only after the confirmation, through the contribution, and reads the page again", async () => {
    mockCancel.mockResolvedValue(undefined);
    const { onReload, onChanged } = renderCard(HOLIDAY_REGISTRATION);

    fireEvent.click(screen.getByRole("button", { name: "Stornieren" }));
    expect(mockCancel).not.toHaveBeenCalled();
    expect(
      await screen.findByText("Anmeldung stornieren?"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/berechnet den Ferienbeitrag der Geschwister neu/),
    ).not.toBeInTheDocument();
    const buttons = screen.getAllByRole("button", { name: "Stornieren" });
    fireEvent.click(buttons[buttons.length - 1]!);

    await waitFor(() => expect(mockCancel).toHaveBeenCalledWith("5", "7"));
    await waitFor(() => expect(onReload).toHaveBeenCalledTimes(1));
    expect(mockChange).not.toHaveBeenCalled();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it("says before a cancellation that the siblings are computed again (#5662)", async () => {
    renderCard(HOLIDAY_REGISTRATION, vi.fn(), {
      ...HOLIDAY_CONTRIBUTION,
      siblings: [
        {
          registrationId: "8",
          cancelled: false,
          children: [{ id: "81", name: "Tom Muster" }],
        },
      ],
    });

    fireEvent.click(screen.getByRole("button", { name: "Stornieren" }));

    expect(
      await screen.findByText(
        "Die Anmeldung zählt dann nicht mehr zu den Geschwistern. moto berechnet den Ferienbeitrag der Geschwister neu.",
      ),
    ).toBeInTheDocument();
    expect(mockCancel).not.toHaveBeenCalled();
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
            onReload={vi.fn(() => Promise.resolve())}
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

    it("proposes the debtor number of the sibling group (#5628)", async () => {
      mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
      render(
        <ToastProvider>
          <HolidayRegistrationStatusCard
            periodId="5"
            registration={HOLIDAY_REGISTRATION}
            contribution={{
              ...HOLIDAY_CONTRIBUTION,
              siblingDebtorNumber: "10 4711",
            }}
            debtorNumberRequired
            exemption={false}
            onExemptionChange={vi.fn()}
            onChanged={vi.fn()}
            onReload={vi.fn(() => Promise.resolve())}
            onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
          />
        </ToastProvider>,
      );

      expect(screen.getByLabelText(/Debitorennummer/)).toHaveValue("10 4711");
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
      await waitFor(() =>
        expect(mockConfirmDebtorNumber).toHaveBeenCalledWith("10 4711"),
      );
    });

    it("follows the group's proposal after a split and keeps a typed number (#5628)", async () => {
      mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
      const card = (siblingDebtorNumber: string, registrationId = "7") => (
        <ToastProvider>
          <HolidayRegistrationStatusCard
            periodId="5"
            registration={{ ...HOLIDAY_REGISTRATION, id: registrationId }}
            contribution={{ ...HOLIDAY_CONTRIBUTION, siblingDebtorNumber }}
            debtorNumberRequired
            exemption={false}
            onExemptionChange={vi.fn()}
            onChanged={vi.fn()}
            onReload={vi.fn(() => Promise.resolve())}
            onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
          />
        </ToastProvider>
      );
      const { rerender } = render(card("10 4711"));
      expect(screen.getByLabelText(/Debitorennummer/)).toHaveValue("10 4711");

      // The split leaves the registration without a group and its number.
      rerender(card(""));
      expect(screen.getByLabelText(/Debitorennummer/)).toHaveValue("");

      fireEvent.change(screen.getByLabelText(/Debitorennummer/), {
        target: { value: "55" },
      });
      rerender(card("20 0815"));
      expect(screen.getByLabelText(/Debitorennummer/)).toHaveValue("55");

      // Another registration starts with its own proposal.
      rerender(card("20 0815", "8"));
      expect(screen.getByLabelText(/Debitorennummer/)).toHaveValue("20 0815");
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
      await waitFor(() =>
        expect(mockConfirmDebtorNumber).toHaveBeenCalledWith("20 0815"),
      );
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
            onReload={vi.fn(() => Promise.resolve())}
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
            onReload={vi.fn(() => Promise.resolve())}
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

  describe("format of the debtor number (#6675)", () => {
    const FIVE = { digitsOnly: true, minDigits: 5, maxDigits: 5 };

    it("names the format at the field, opens the number keyboard and refuses another format before it confirms", async () => {
      render(
        <ToastProvider>
          <HolidayRegistrationStatusCard
            periodId="5"
            registration={HOLIDAY_REGISTRATION}
            contribution={HOLIDAY_CONTRIBUTION}
            debtorNumberRequired
            debtorNumberFormat={FIVE}
            exemption={false}
            onExemptionChange={vi.fn()}
            onChanged={vi.fn()}
            onReload={vi.fn(() => Promise.resolve())}
            onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
          />
        </ToastProvider>,
      );

      const field = screen.getByRole("textbox", { name: /^Debitorennummer/ });
      expect(field).toHaveAttribute("inputmode", "numeric");
      expect(field).toHaveAccessibleDescription(
        "Die Nummer hat 5 Ziffern, zum Beispiel 20815. Die Nummer der Familie bei der Kasse der Gemeinde. Die Familie gibt sie bei der Überweisung an.",
      );
      fireEvent.change(field, { target: { value: "20 815" } });
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

      expect(
        await screen.findByText(
          "Die Debitorennummer hat 5 Ziffern, zum Beispiel 20815.",
        ),
      ).toBeInTheDocument();
      expect(field).toHaveAttribute("aria-invalid", "true");
      expect(mockConfirm).not.toHaveBeenCalled();
    });

    it("keeps the plain field without a format", () => {
      render(
        <ToastProvider>
          <HolidayRegistrationStatusCard
            periodId="5"
            registration={HOLIDAY_REGISTRATION}
            contribution={HOLIDAY_CONTRIBUTION}
            debtorNumberRequired
            exemption={false}
            onExemptionChange={vi.fn()}
            onChanged={vi.fn()}
            onReload={vi.fn(() => Promise.resolve())}
            onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
          />
        </ToastProvider>,
      );

      const field = screen.getByRole("textbox", { name: /^Debitorennummer/ });
      expect(field).not.toHaveAttribute("inputmode");
      expect(field).toHaveAccessibleDescription(
        "Die Nummer der Familie bei der Kasse der Gemeinde. Die Familie gibt sie bei der Überweisung an.",
      );
    });
  });

  describe("debtor number of the family (#6692)", () => {
    const conflict = new ApiError("conflict", 409, {
      code: "parent_billing.family_debtor_number_conflict",
      details: {
        family_debtor_number: "10 4711",
        debtor_number: "99",
        family_email: "papa.muster@example.org",
      },
    });

    function renderWithFamily(
      registration: HolidayRegistrationDetail = HOLIDAY_REGISTRATION,
      debtorNumberRequired = false,
    ) {
      const onReload = vi.fn(() => Promise.resolve());
      render(
        <ToastProvider>
          <HolidayRegistrationStatusCard
            periodId="5"
            registration={registration}
            contribution={HOLIDAY_CONTRIBUTION}
            debtorNumberRequired={debtorNumberRequired}
            familyDebtorNumber="10 4711"
            exemption={false}
            onExemptionChange={vi.fn()}
            onChanged={vi.fn()}
            onReload={onReload}
            onDebtorNumberChanged={vi.fn(() => Promise.resolve())}
          />
        </ToastProvider>,
      );
      return { onReload };
    }

    it("proposes the family's number at the field and fills it only on Übernehmen", async () => {
      mockConfirm.mockResolvedValue({ ...HOLIDAY_CONTRIBUTION, frozen: true });
      const { onReload } = renderWithFamily();

      const field = screen.getByRole("textbox", { name: /^Debitorennummer/ });
      expect(field).toHaveValue("");
      expect(field).not.toBeRequired();
      expect(field).toHaveAccessibleDescription(
        /Bekannte Nummer dieser Familie: 10 4711 ?\. Bitte prüfen\./,
      );
      fireEvent.click(
        screen.getByRole("button", {
          name: "Debitorennummer 10 4711 übernehmen",
        }),
      );
      expect(field).toHaveValue("10 4711");
      expect(field).toHaveFocus();
      expect(screen.queryByText(/Bekannte Nummer dieser Familie/)).toBeNull();
      expect(mockConfirm).not.toHaveBeenCalled();

      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
      await waitFor(() => expect(onReload).toHaveBeenCalled());
      expect(mockConfirmDebtorNumber).toHaveBeenCalledWith("10 4711");
      expect(mockFamilyChoice).toHaveBeenCalledWith(undefined);
    });

    it("asks which number applies when the number differs and confirms with the choice", async () => {
      mockConfirm
        .mockRejectedValueOnce(conflict)
        .mockResolvedValueOnce({ ...HOLIDAY_CONTRIBUTION, frozen: true });
      const { onReload } = renderWithFamily(HOLIDAY_REGISTRATION, true);

      fireEvent.change(
        screen.getByRole("textbox", { name: /^Debitorennummer/ }),
        {
          target: { value: "99" },
        },
      );
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

      const dialog = await screen.findByRole("dialog", {
        name: "Welche Debitorennummer gilt?",
      });
      expect(dialog).toHaveTextContent("10 4711");
      expect(dialog).toHaveTextContent("99");
      // #6692: in a sibling group the family can be the sibling's.
      expect(dialog).toHaveTextContent(
        "Für die Familie papa.muster@example.org ist schon eine andere Nummer gemerkt.",
      );
      const keep = screen.getByRole("button", {
        name: /Gemerkte Nummer behalten/,
      });
      expect(keep).toHaveTextContent(
        "Nur diese Anmeldung bekommt 99. Die Familie behält 10 4711.",
      );
      fireEvent.click(keep);

      await waitFor(() => expect(onReload).toHaveBeenCalled());
      expect(mockFamilyChoice).toHaveBeenLastCalledWith("keep");
      expect(mockConfirmDebtorNumber).toHaveBeenLastCalledWith("99");
      expect(screen.queryByText(/Bitte prüfen Sie/)).toBeNull();
    });

    it("shows every conflicting family before replacing their numbers", async () => {
      mockConfirm
        .mockRejectedValueOnce(
          new ApiError("conflict", 409, {
            code: "parent_billing.family_debtor_number_conflict",
            details: {
              family_email: "eva.muster@example.org",
              family_debtor_number: "40100",
              debtor_number: "99",
              families: [
                {
                  family_email: "eva.muster@example.org",
                  family_debtor_number: "40100",
                },
                {
                  family_email: "papa.muster@example.org",
                  family_debtor_number: "30100",
                },
              ],
            },
          }),
        )
        .mockResolvedValueOnce({ ...HOLIDAY_CONTRIBUTION, frozen: true });
      const { onReload } = renderWithFamily();
      fireEvent.change(
        screen.getByRole("textbox", { name: /^Debitorennummer/ }),
        { target: { value: "99" } },
      );
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
      const dialog = await screen.findByRole("dialog", {
        name: "Welche Debitorennummer gilt?",
      });
      expect(dialog).toHaveTextContent("eva.muster@example.org: 40100");
      expect(dialog).toHaveTextContent("papa.muster@example.org: 30100");
      fireEvent.click(
        screen.getByRole("button", {
          name: /Neue Nummer für alle genannten Familien merken/,
        }),
      );
      await waitFor(() => expect(onReload).toHaveBeenCalled());
      expect(mockFamilyChoice).toHaveBeenLastCalledWith("replace");
    });

    it("asks too when a confirmed registration gets another number", async () => {
      mockChangeDebtorNumber
        .mockRejectedValueOnce(conflict)
        .mockResolvedValueOnce("99");
      renderWithFamily({ ...HOLIDAY_REGISTRATION, status: "confirmed" }, true);

      fireEvent.click(
        screen.getByRole("button", { name: "Debitorennummer eintragen" }),
      );
      fireEvent.change(
        screen.getByRole("textbox", { name: /^Debitorennummer/ }),
        {
          target: { value: "99" },
        },
      );
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      expect(
        await screen.findByRole("dialog", {
          name: "Welche Debitorennummer gilt?",
        }),
      ).toBeInTheDocument();
      fireEvent.click(
        screen.getByRole("button", {
          name: /Neue Nummer für die Familie merken/,
        }),
      );

      await waitFor(() =>
        expect(mockChangeDebtorNumber).toHaveBeenCalledTimes(2),
      );
      expect(mockFamilyChoice).toHaveBeenLastCalledWith("replace");
    });
  });
});
