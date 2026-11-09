import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { NumberRange } from "~/lib/traeger-billing-api";
import type {
  ContributionRule,
  CostBearer,
} from "~/lib/traeger-contributions-api";
import { catalogText } from "~/test/error-catalog-text";

const { billingApi, termsApi, masterApi, session, api } = vi.hoisted(() => ({
  // Zahlung und Rechnungen (#4689): the defaults of every service.
  termsApi: {
    fetchBillingTerms: vi.fn(() =>
      Promise.resolve({
        services: [
          { service: "care", bundleWith: [] },
          { service: "meal", bundleWith: ["care"] },
          { service: "holiday", bundleWith: [] },
        ],
        terms: ["care", "meal", "holiday"].map((service) => ({
          service,
          paymentMethod: "transfer",
          dueDays: 14,
          bundleWith: service === "meal" ? "care" : "",
          custom: false,
          updatedAt: null,
        })),
      }),
    ),
    saveServiceTerms: vi.fn(),
  },
  session: {
    permissions: [
      "traeger_overview:read",
      "traeger_billing:manage",
      "traeger_settings:manage",
    ],
  },
  billingApi: {
    fetchHolidayNumberRange: vi.fn(),
    saveHolidayNumberRange: vi.fn(),
    fetchCareNumberRange: vi.fn(),
    saveCareNumberRange: vi.fn(),
  },
  masterApi: {
    fetchTraegerMasterData: vi.fn(),
    createTraegerBankAccount: vi.fn(),
    updateTraegerBankAccount: vi.fn(),
    deleteTraegerBankAccount: vi.fn(),
    // The switch of the online mandate (#4696) reads the Träger settings.
    fetchTraegerSettingsSchema: vi.fn().mockResolvedValue({ tabs: [] }),
  },
  api: {
    fetchContributionRules: vi.fn(),
    createContributionRule: vi.fn(),
    updateContributionRule: vi.fn(),
    deleteContributionRule: vi.fn(),
    fetchCostBearers: vi.fn(),
    saveCostBearer: vi.fn(),
    deleteCostBearer: vi.fn(),
  },
}));

vi.mock("~/lib/traeger-contributions-api", () => api);
vi.mock("~/lib/traeger-billing-api", () => billingApi);
vi.mock("~/lib/traeger-billing-terms-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...termsApi,
}));
vi.mock("~/lib/traeger-api", () => masterApi);
// The creditor of the SEPA direct debit (#4695) loads with the defaults.
vi.mock("~/lib/traeger-sepa-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  fetchSepaCreditor: vi.fn().mockResolvedValue({
    creditorId: "",
    prenotificationDays: 14,
    missingMandate: "transfer",
    updatedAt: null,
  }),
  saveSepaCreditor: vi.fn(),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", permissions: session.permissions } },
    status: "authenticated",
  }),
}));

import TraegerAbrechnungsvorgabenPage from "./page";

const SIBLING: ContributionRule = {
  id: "3",
  kind: "sibling",
  validFrom: "2026-08-01",
  validUntil: null,
  siblingFromChild: 2,
  discountBasisPoints: 5000,
  discountCents: null,
  note: "",
};

const YOUTH_OFFICE: CostBearer = {
  id: "4",
  name: "Fachdienst Jugend des Landkreises Musterstadt",
  address: "Am Kreishaus 1\n12345 Musterstadt",
  fileReference: "FJ 51-2027",
  active: true,
  inUse: true,
};

const MASTER_DATA = {
  contact: { name: "", phone: "", email: "" },
  logo: null,
  bankAccounts: [
    {
      id: "7",
      label: "Elternbeiträge",
      accountHolder: "Ganztag im Quartier e.V.",
      iban: "DE89370400440532013000",
      bic: "COBADEFFXXX",
      bankName: "Commerzbank",
      isDefault: true,
    },
  ],
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerAbrechnungsvorgabenPage />
    </ToastProvider>,
  );
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

describe("TraegerAbrechnungsvorgabenPage", () => {
  beforeEach(() => {
    for (const fn of [
      ...Object.values(api),
      ...Object.values(billingApi),
      ...Object.values(masterApi),
    ]) {
      fn.mockReset();
    }
    session.permissions = [
      "traeger_overview:read",
      "traeger_billing:manage",
      "traeger_settings:manage",
    ];
    api.fetchContributionRules.mockResolvedValue([SIBLING]);
    api.fetchCostBearers.mockResolvedValue([]);
    billingApi.fetchHolidayNumberRange.mockResolvedValue({
      service: "holiday",
      pattern: "FE-{JJJJ}-{NNNN}",
      example: "FE-2026-0001",
    });
    billingApi.fetchCareNumberRange.mockResolvedValue({
      service: "care",
      pattern: "",
      example: "",
    });
    masterApi.fetchTraegerMasterData.mockResolvedValue(MASTER_DATA);
  });

  it("lists the rules by kind and names the kind without a rule", async () => {
    renderPage();

    expect(
      (await screen.findAllByText("ab 01.08.2026"))[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("ab dem 2. Kind 50 % weniger")[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Keine Befreiungsregel. Alle Familien zahlen, auch bei Leistungsbezug.",
      )[0],
    ).toBeInTheDocument();
  });

  it("adds a sibling rule with a share in basis points", async () => {
    api.createContributionRule.mockResolvedValue(SIBLING);
    renderPage();
    await screen.findAllByText("ab 01.08.2026");

    fireEvent.click(
      screen.getByRole("button", { name: "Geschwisterregel hinzufügen" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "ab dem 3. Kind" }),
    );
    fireEvent.change(screen.getByLabelText("Anteil in Prozent"), {
      target: { value: "33,5" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(api.createContributionRule).toHaveBeenCalled());
    expect(api.createContributionRule).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: "sibling",
        siblingFromChild: 3,
        discountBasisPoints: 3350,
        discountCents: null,
        validFrom: "2026-09-09",
        validUntil: null,
      }),
    );
    await waitFor(() =>
      expect(api.fetchContributionRules).toHaveBeenCalledTimes(2),
    );
  });

  it("asks for the note of an exemption and shows an overlap at the form", async () => {
    api.createContributionRule.mockRejectedValue(
      new ApiError("overlap", 409, {
        code: "parent_billing.contribution_rule_overlap",
      }),
    );
    renderPage();
    await screen.findAllByText("ab 01.08.2026");

    expect(
      screen.getAllByText(
        "Die Regel gilt für alle Familien mit Leistungsbezug. Welche Familie befreit ist, wählen Sie beim Bestätigen der Anmeldung. Befreite Familien zahlen nichts.",
      )[0],
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Befreiungsregel hinzufügen" }),
    );
    expect(
      await screen.findByRole("heading", {
        name: "Befreiungsregel hinzufügen",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die Regel gilt für alle Familien mit Leistungsbezug. Welche Familie befreit ist, wählen Sie beim Bestätigen der Anmeldung.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText("Vermerk auf dem Beitrag"),
    ).toHaveAccessibleDescription(
      "Der Vermerk steht beim Beitrag jeder befreiten Familie.",
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte geben Sie den Vermerk an."),
    ).toBeInTheDocument();
    expect(api.createContributionRule).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Vermerk auf dem Beitrag"), {
      target: { value: "Beitragsfrei mit Nachweis" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText(
        catalogText("parent_billing.contribution_rule_overlap", "die Regel"),
      ),
    ).toBeInTheDocument();
    expect(api.createContributionRule).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: "benefits_exemption",
        note: "Beitragsfrei mit Nachweis",
        siblingFromChild: null,
      }),
    );
  });

  it("adds a cost bearer and asks for its name", async () => {
    api.saveCostBearer.mockResolvedValue(YOUTH_OFFICE);
    renderPage();
    expect(
      (
        await screen.findAllByText(
          "Kein Kostenträger. Beim Bestätigen bezahlt die Familie oder ist befreit.",
        )
      )[0],
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Kostenträger hinzufügen" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte geben Sie den Namen ein."),
    ).toBeInTheDocument();
    expect(api.saveCostBearer).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText(/^Name/), {
      target: { value: YOUTH_OFFICE.name },
    });
    fireEvent.change(
      screen.getByLabelText("Ihr Aktenzeichen beim Kostenträger (freiwillig)"),
      { target: { value: "FJ 51-2027" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.saveCostBearer).toHaveBeenCalledWith(null, {
        name: YOUTH_OFFICE.name,
        address: "",
        fileReference: "FJ 51-2027",
        active: true,
      }),
    );
    await waitFor(() => expect(api.fetchCostBearers).toHaveBeenCalledTimes(2));
  });

  it("only switches off a cost bearer a contribution names", async () => {
    api.fetchCostBearers.mockResolvedValue([YOUTH_OFFICE]);
    api.saveCostBearer.mockResolvedValue({ ...YOUTH_OFFICE, active: false });
    renderPage();
    const name = (await screen.findAllByText(YOUTH_OFFICE.name))[0];
    expect(name).toHaveClass("wrap-anywhere");

    fireEvent.click(
      screen.getAllByRole("button", {
        name: `Aktionen für ${YOUTH_OFFICE.name}`,
      })[0]!,
    );
    expect(
      await screen.findByRole("menuitem", { name: "Ausschalten" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Ändern" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Löschen" })).toBeNull();
    fireEvent.click(screen.getByRole("menuitem", { name: "Ausschalten" }));

    await waitFor(() =>
      expect(api.saveCostBearer).toHaveBeenCalledWith("4", {
        ...YOUTH_OFFICE,
        active: false,
      }),
    );
  });

  it("changes a cost bearer in its row, one row at a time, without a dialog", async () => {
    const social: CostBearer = {
      id: "5",
      name: "Sozialamt Musterstadt",
      address: "",
      fileReference: "",
      active: true,
      inUse: false,
    };
    api.fetchCostBearers.mockResolvedValue([YOUTH_OFFICE, social]);
    api.saveCostBearer.mockResolvedValue(YOUTH_OFFICE);
    renderPage();
    await screen.findAllByText(YOUTH_OFFICE.name);

    const openChange = async (name: string) => {
      fireEvent.click(
        screen.getAllByRole("button", { name: `Aktionen für ${name}` })[0]!,
      );
      fireEvent.click(await screen.findByRole("menuitem", { name: "Ändern" }));
    };

    await openChange(YOUTH_OFFICE.name);
    const form = await screen.findByRole("form", {
      name: `${YOUTH_OFFICE.name} ändern`,
    });
    expect(form.closest("li")).not.toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByDisplayValue("FJ 51-2027")).toBeInTheDocument();
    expect(screen.getByText("Sozialamt Musterstadt")).toBeInTheDocument();

    // Opening another row closes the first one.
    await openChange(social.name);
    expect(
      await screen.findByRole("form", { name: "Sozialamt Musterstadt ändern" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("form", { name: `${YOUTH_OFFICE.name} ändern` }),
    ).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(
      screen.queryByRole("form", { name: "Sozialamt Musterstadt ändern" }),
    ).toBeNull();
    expect(screen.getByText("Sozialamt Musterstadt")).toBeInTheDocument();

    await openChange(YOUTH_OFFICE.name);
    const name = await screen.findByDisplayValue(YOUTH_OFFICE.name);
    fireEvent.change(name, { target: { value: " " } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte geben Sie den Namen ein."),
    ).toBeInTheDocument();
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(api.saveCostBearer).not.toHaveBeenCalled();

    fireEvent.change(name, { target: { value: "Jugendamt Musterstadt" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(api.saveCostBearer).toHaveBeenCalledWith(
        "4",
        expect.objectContaining({
          name: "Jugendamt Musterstadt",
          address: YOUTH_OFFICE.address,
          fileReference: "FJ 51-2027",
          active: true,
        }),
      ),
    );
    await waitFor(() => expect(api.fetchCostBearers).toHaveBeenCalledTimes(2));
    expect(
      screen.queryByRole("form", { name: "Jugendamt Musterstadt ändern" }),
    ).toBeNull();
  });

  it("shows bank accounts, number range, rules and cost bearers together (#5685)", async () => {
    renderPage();

    expect(
      await screen.findByRole("button", { name: "Bankverbindung hinzufügen" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("FE-2026-0001")).toBeInTheDocument();
    expect(screen.getByText("Geschwister")).toBeInTheDocument();
    expect(screen.getByText("Kostenträger")).toBeInTheDocument();
  });

  it("shows only the read-only bank accounts without the right Abrechnung (#5685)", async () => {
    session.permissions = ["traeger_overview:read"];
    renderPage();

    expect(
      (await screen.findAllByText("Elternbeiträge"))[0],
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Die Bankverbindungen können Sie nur ansehen/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/pflegt die Rolle „Abrechnung“/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Bankverbindung hinzufügen" }),
    ).not.toBeInTheDocument();
    expect(api.fetchContributionRules).not.toHaveBeenCalled();
    expect(api.fetchCostBearers).not.toHaveBeenCalled();
    expect(billingApi.fetchHolidayNumberRange).not.toHaveBeenCalled();
  });

  it("hides billing defaults when the billing permission ends", async () => {
    const page = renderPage();
    expect(await screen.findByText("FE-2026-0001")).toBeInTheDocument();

    session.permissions = ["traeger_overview:read"];
    page.rerender(
      <ToastProvider>
        <TraegerAbrechnungsvorgabenPage />
      </ToastProvider>,
    );

    expect(
      screen.getByText(/pflegt die Rolle „Abrechnung“/),
    ).toBeInTheDocument();
    expect(screen.queryByText("FE-2026-0001")).not.toBeInTheDocument();
    expect(screen.queryByText("Geschwister")).not.toBeInTheDocument();
  });

  it("ignores billing defaults that arrive after the billing permission ends", async () => {
    const rules = deferred<ContributionRule[]>();
    const bearers = deferred<CostBearer[]>();
    const numberRange = deferred<NumberRange>();
    api.fetchContributionRules.mockReturnValueOnce(rules.promise);
    api.fetchCostBearers.mockReturnValueOnce(bearers.promise);
    billingApi.fetchHolidayNumberRange.mockReturnValueOnce(numberRange.promise);
    const page = renderPage();

    await waitFor(() =>
      expect(api.fetchContributionRules).toHaveBeenCalledOnce(),
    );
    session.permissions = ["traeger_overview:read"];
    page.rerender(
      <ToastProvider>
        <TraegerAbrechnungsvorgabenPage />
      </ToastProvider>,
    );
    expect(
      screen.getByText(/pflegt die Rolle „Abrechnung“/),
    ).toBeInTheDocument();
    await act(async () => {
      rules.resolve([SIBLING]);
      bearers.resolve([]);
      numberRange.resolve({
        service: "holiday",
        pattern: "FE-{JJJJ}-{NNNN}",
        example: "FE-2026-0001",
      });
      await Promise.all([rules.promise, bearers.promise, numberRange.promise]);
    });

    expect(screen.queryByText("FE-2026-0001")).not.toBeInTheDocument();
    expect(screen.queryByText("Geschwister")).not.toBeInTheDocument();
  });

  it("lets the Verwaltung change bank accounts without the right Abrechnung (#5685)", async () => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    renderPage();

    expect(
      await screen.findByRole("button", { name: "Bankverbindung hinzufügen" }),
    ).toBeInTheDocument();
    expect(api.fetchContributionRules).not.toHaveBeenCalled();
  });

  it("shows a failed load and retries", async () => {
    billingApi.fetchHolidayNumberRange.mockRejectedValueOnce(
      new ApiError("unavailable", 503, { code: "general.unavailable" }),
    );
    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Abrechnungsvorgaben"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText("FE-2026-0001")).toBeInTheDocument();
  });

  it("keeps the bank accounts when reading them again fails (#5685)", async () => {
    masterApi.createTraegerBankAccount.mockResolvedValue({
      ...MASTER_DATA.bankAccounts[0],
      id: "9",
    });
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Bankverbindung hinzufügen" }),
    );
    fireEvent.change(screen.getByLabelText(/Bezeichnung/), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText(/Kontoinhaber/), {
      target: { value: "Träger e.V." },
    });
    fireEvent.change(screen.getByLabelText(/^IBAN/), {
      target: { value: "GB82 WEST 1234 5698 7654 32" },
    });
    masterApi.fetchTraegerMasterData.mockRejectedValueOnce(
      new ApiError("unavailable", 503, { code: "general.unavailable" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Bankverbindungen"),
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Elternbeiträge")[0]).toBeInTheDocument();
  });

  it("asks for a pattern before the first release and saves it", async () => {
    billingApi.fetchHolidayNumberRange.mockResolvedValue({
      service: "holiday",
      pattern: "",
      example: "",
    });
    billingApi.saveHolidayNumberRange.mockResolvedValue({
      service: "holiday",
      pattern: "R-{NNN}",
      example: "R-001",
    });
    // The monthly care has its pattern, so only the holiday card asks.
    billingApi.fetchCareNumberRange.mockResolvedValue({
      service: "care",
      pattern: "MB-{NNNN}",
      example: "MB-0001",
    });
    renderPage();

    expect(
      await screen.findByText(/Ohne Muster können Sie keinen Lauf freigeben/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Muster festlegen" }));
    fireEvent.change(
      await screen.findByLabelText("Muster der Rechnungsnummer"),
      { target: { value: "R-{NNN}" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(billingApi.saveHolidayNumberRange).toHaveBeenCalledWith("R-{NNN}"),
    );
    await waitFor(() =>
      expect(billingApi.fetchHolidayNumberRange).toHaveBeenCalledTimes(2),
    );
  });

  it("keeps the number range of the monthly contribution apart (#4684)", async () => {
    billingApi.saveCareNumberRange.mockResolvedValue({
      service: "care",
      pattern: "MB-{JJJJ}-{NNNN}",
      example: "MB-2026-0001",
    });
    renderPage();

    // The holiday range has its pattern; only the monthly one asks.
    expect(
      await screen.findByText("Rechnungsnummern der Monatsbeiträge"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Muster festlegen" }));
    fireEvent.change(
      await screen.findByLabelText("Muster der Rechnungsnummer"),
      { target: { value: "MB-{JJJJ}-{NNNN}" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(billingApi.saveCareNumberRange).toHaveBeenCalledWith(
        "MB-{JJJJ}-{NNNN}",
      ),
    );
    expect(billingApi.saveHolidayNumberRange).not.toHaveBeenCalled();
  });

  it("adds a bank account and reads the master data again", async () => {
    masterApi.createTraegerBankAccount.mockResolvedValue({
      ...MASTER_DATA.bankAccounts[0],
      id: "9",
    });
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Bankverbindung hinzufügen" }),
    );
    fireEvent.change(screen.getByLabelText(/Bezeichnung/), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText(/Kontoinhaber/), {
      target: { value: "Ganztag im Quartier e.V." },
    });
    fireEvent.change(screen.getByLabelText(/^IBAN/), {
      target: { value: "GB82 WEST 1234 5698 7654 32" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(masterApi.createTraegerBankAccount).toHaveBeenCalledWith({
        label: "Mensa",
        accountHolder: "Ganztag im Quartier e.V.",
        iban: "GB82 WEST 1234 5698 7654 32",
        bic: "",
        bankName: "",
        isDefault: false,
      }),
    );
    await waitFor(() =>
      expect(masterApi.fetchTraegerMasterData).toHaveBeenCalledTimes(2),
    );
  });

  it("shows a refused IBAN at its field", async () => {
    masterApi.createTraegerBankAccount.mockRejectedValue(
      new ApiError("invalid", 400, {
        code: "traeger.master_data_invalid",
        errors: [{ field: "iban", reason: "is not a valid IBAN" }],
      }),
    );
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Bankverbindung hinzufügen" }),
    );
    fireEvent.change(screen.getByLabelText(/Bezeichnung/), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText(/Kontoinhaber/), {
      target: { value: "Träger e.V." },
    });
    fireEvent.change(screen.getByLabelText(/^IBAN/), {
      target: { value: "DE00 1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        catalogText("traeger.master_data_invalid", "die Bankverbindung"),
      ),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByLabelText(/^IBAN/)).toHaveAttribute(
        "aria-invalid",
        "true",
      ),
    );
  });
});
