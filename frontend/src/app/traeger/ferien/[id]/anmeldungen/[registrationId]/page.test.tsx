import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import {
  HOLIDAY_CONTRIBUTION,
  HOLIDAY_PERIOD,
  HOLIDAY_REGISTRATION,
  HOLIDAY_REGISTRATION_HEALTH,
  HOLIDAY_SCHOOLS,
} from "~/test/fixtures/holiday-registrations";
import { HOLIDAY_CONSENT_TEXTS } from "~/test/fixtures/holiday-consent-texts";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useParams: () => ({ id: "5", registrationId: "7" }),
}));
vi.mock("~/lib/traeger-api", () => ({
  fetchHolidayPeriod: () => Promise.resolve(HOLIDAY_PERIOD),
  fetchTraegerSchools: () => Promise.resolve(HOLIDAY_SCHOOLS),
}));
const {
  mockRegistration,
  mockHealth,
  mockEdit,
  mockConfirmationSettings,
  sessionPermissions,
  mockDownloadPdf,
  mockDownloadBlob,
  contributionOverride,
} = vi.hoisted(() => ({
  mockRegistration: vi.fn(),
  mockHealth: vi.fn(),
  mockEdit: vi.fn(),
  mockConfirmationSettings: vi.fn(),
  sessionPermissions: { current: [] as string[] },
  mockDownloadPdf: vi.fn(),
  mockDownloadBlob: vi.fn(),
  contributionOverride: { current: null as unknown },
}));
vi.mock("~/lib/traeger-holiday-registrations-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/traeger-holiday-registrations-api")
  >()),
  fetchHolidayRegistration: () => mockRegistration(),
  fetchHolidayRegistrationHealth: () => mockHealth(),
  editHolidayRegistration: (...args: unknown[]) => mockEdit(...args),
  fetchHolidayConfirmationSettings: () => mockConfirmationSettings(),
  downloadHolidayRegistrationPdf: (...args: unknown[]) =>
    mockDownloadPdf(...args),
}));
vi.mock("~/lib/file-download", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/file-download")>()),
  downloadBlob: (blob: Blob, name: string) => mockDownloadBlob(blob, name),
}));
const { mockContribution } = vi.hoisted(() => ({ mockContribution: vi.fn() }));
vi.mock("~/lib/traeger-contributions-api", () => ({
  // A test sets the contribution through the override (#5629) or the
  // mock (#5628).
  fetchHolidayContribution: () =>
    contributionOverride.current
      ? Promise.resolve(contributionOverride.current)
      : mockContribution(),
}));

const { mockSplit } = vi.hoisted(() => ({ mockSplit: vi.fn() }));
vi.mock("~/lib/traeger-sibling-groups-api", () => ({
  splitHolidaySibling: (periodId: string, registrationId: string) =>
    mockSplit(periodId, registrationId),
}));

import TraegerHolidayRegistrationPage from "./page";

describe("TraegerHolidayRegistrationPage", () => {
  beforeEach(() => {
    mockRegistration.mockResolvedValue(HOLIDAY_REGISTRATION);
    mockContribution.mockResolvedValue(HOLIDAY_CONTRIBUTION);
    mockConfirmationSettings.mockResolvedValue({ debtorNumberRequired: false });
    mockHealth.mockReset();
    mockHealth.mockResolvedValue(HOLIDAY_REGISTRATION_HEALTH);
    mockEdit.mockReset();
    sessionPermissions.current = [];
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
    // The Träger portal has no school: no TenantProvider above the page.
    vi.mocked(useTenantSlugSafe).mockReturnValue(null);
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
  });

  describe("siblings from another registration (#5628)", () => {
    const SIBLINGS = [
      {
        registrationId: "8",
        cancelled: false,
        children: [{ id: "81", name: "Tom Muster" }],
      },
      {
        registrationId: "9",
        cancelled: true,
        children: [{ id: "91", name: "Ida Muster" }],
      },
    ];
    beforeEach(() => {
      mockSplit.mockReset();
    });

    function renderPage() {
      render(
        <ToastProvider>
          <TraegerHolidayRegistrationPage />
        </ToastProvider>,
      );
    }

    async function confirmSplit() {
      fireEvent.click(await screen.findByRole("button", { name: "Trennen" }));
      expect(
        await screen.findByText("Von den Geschwistern trennen?"),
      ).toBeInTheDocument();
      const buttons = screen.getAllByRole("button", { name: "Trennen" });
      fireEvent.click(buttons[buttons.length - 1]!);
    }

    it("shows no sibling card without siblings", async () => {
      renderPage();

      expect(
        await screen.findByRole("heading", {
          level: 1,
          name: "Anmeldung von Eva Muster",
        }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("heading", { name: "Geschwister" }),
      ).not.toBeInTheDocument();
    });

    it("names the children of the other registrations and splits after asking", async () => {
      mockContribution.mockResolvedValue({
        ...HOLIDAY_CONTRIBUTION,
        siblings: SIBLINGS,
      });
      mockSplit.mockResolvedValue(undefined);
      renderPage();

      expect(
        await screen.findByRole("link", { name: "Anmeldung Nr. 8" }),
      ).toHaveAttribute("href", "/traeger/ferien/5/anmeldungen/8");
      expect(screen.getByText(/Tom Muster/)).toBeInTheDocument();
      expect(
        screen.getByText(/Ida Muster \(storniert, zählt nicht mit\)/),
      ).toBeInTheDocument();
      mockContribution.mockResolvedValue(HOLIDAY_CONTRIBUTION);
      await confirmSplit();

      await waitFor(() => expect(mockSplit).toHaveBeenCalledWith("5", "7"));
      expect(
        await screen.findByText(
          "Die Anmeldung ist von den Geschwistern getrennt.",
        ),
      ).toBeInTheDocument();
      await waitFor(() =>
        expect(
          screen.queryByRole("heading", { name: "Geschwister" }),
        ).not.toBeInTheDocument(),
      );
    });

    it("shows the issued invoice when the split is refused", async () => {
      mockContribution.mockResolvedValue({
        ...HOLIDAY_CONTRIBUTION,
        siblings: SIBLINGS,
      });
      mockSplit.mockRejectedValue(
        new ApiError("invoiced", 409, {
          code: "parent_billing.sibling_group_invoiced",
        }),
      );
      renderPage();
      await confirmSplit();

      expect(
        await screen.findByText(
          "Für diese Anmeldung gibt es schon eine Rechnung. Geschwister können Sie jetzt nicht mehr ändern.",
        ),
      ).toBeInTheDocument();
    });

    it("offers no split once an invoice bills any registration of the group", async () => {
      mockContribution.mockResolvedValue({
        ...HOLIDAY_CONTRIBUTION,
        invoiced: false,
        siblings: SIBLINGS,
        siblingsInvoiced: true,
      });
      renderPage();

      expect(
        await screen.findByText(
          "Für eine dieser Anmeldungen gibt es schon eine Rechnung. Geschwister können Sie jetzt nicht mehr ändern.",
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("link", { name: "Anmeldung Nr. 8" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Trennen" }),
      ).not.toBeInTheDocument();
    });

    it("asks to check the invoice when a cancelled sibling changed the amount (#5662)", async () => {
      mockRegistration.mockResolvedValue({
        ...HOLIDAY_REGISTRATION,
        status: "confirmed",
      });
      mockContribution.mockResolvedValue({
        ...HOLIDAY_CONTRIBUTION,
        frozen: true,
        confirmedAt: "2026-09-08T10:00:00Z",
        invoiced: true,
        correctionNeeded: true,
      });
      renderPage();

      expect(await screen.findByText("Rechnung prüfen")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Der Beitrag hat sich geändert, zum Beispiel durch ein Storno bei den Geschwistern. Die Rechnung zeigt noch den alten Betrag. Stornieren Sie die Rechnung. Der nächste Abrechnungslauf stellt dann den neuen Betrag aus.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("names the parents in the title and keeps the number in the status line (#5146)", async () => {
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    // The registration ID is the database key; the title names the parents.
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Anmeldung von Eva Muster",
      }),
    ).toBeInTheDocument();
    // Export and duplicate hint refer to „Anmeldung Nr. 7“, so the number
    // stays visible next to the status.
    // Anker: die Angaben der Statuszeile stehen einzeln, ohne Trennpunkte (#5684).
    expect(await screen.findByText("Nr. 7")).toBeInTheDocument();
    expect(screen.getByText("Winterferien 2027")).toBeInTheDocument();
    expect(screen.getByText(/^Eingegangen am/)).toBeInTheDocument();
    expect(screen.queryByText(/·/)).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Zurück zu den Anmeldungen/ }),
    ).toHaveAttribute("href", "/traeger/ferien/5/anmeldungen");
  });
  it("asks for the debtor number at the confirmation while the switch is on (#5621)", async () => {
    mockConfirmationSettings.mockResolvedValue({ debtorNumberRequired: true });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    expect(
      await screen.findByRole("textbox", { name: /Debitorennummer/ }),
    ).toBeRequired();
  });

  it("shows and changes the debtor number of a confirmed registration with its history (#5621)", async () => {
    mockConfirmationSettings.mockResolvedValue({ debtorNumberRequired: true });
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      status: "confirmed",
      debtorNumber: "10 4711",
      history: [
        {
          action: "confirm",
          occurredAt: "2026-09-04T08:00:00Z",
          from: "received",
          childId: "",
          before: [],
          after: [],
          debtorNumber: { before: "", after: "10 4710" },
        },
        {
          action: "debtor_number",
          occurredAt: "2026-09-05T08:00:00Z",
          from: "",
          childId: "",
          before: [],
          after: [],
          debtorNumber: { before: "10 4710", after: "10 4711" },
        },
      ],
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    const card = (
      await screen.findByRole("heading", { name: "Status" })
    ).closest("section");
    expect(card).not.toBeNull();
    expect(within(card!).getByText("10 4711")).toBeInTheDocument();
    expect(
      within(card!).getByRole("button", { name: "Debitorennummer ändern" }),
    ).toBeEnabled();
    expect(screen.getByText(/Debitorennummer 10 4710$/)).toBeInTheDocument();
    expect(screen.getByText("Debitorennummer geändert")).toBeInTheDocument();
    expect(
      screen.getByText(/Debitorennummer vorher 10 4710, jetzt 10 4711/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("textbox", { name: /Debitorennummer/ }),
    ).not.toBeInTheDocument();
  });

  it("opens the uploaded proofs of benefits from their own card (#5238)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      benefitsDeclared: true,
      benefitsProofs: [
        {
          id: "91",
          filename: "Bescheid Bürgergeld.pdf",
          sizeBytes: 250_000,
          contentType: "application/pdf",
          uploadedAt: "2026-09-03T08:00:00Z",
        },
      ],
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const heading = await screen.findByRole("heading", {
      name: "Nachweis für den Leistungsbezug",
    });
    const card = within(heading.closest("section")!);
    expect(card.getByText("Bescheid Bürgergeld.pdf")).toBeInTheDocument();
    expect(card.getByText("244 KB")).toBeInTheDocument();
    const open = card.getByRole("link", {
      name: "Bescheid Bürgergeld.pdf",
    });
    expect(open).toHaveAttribute(
      "href",
      "/api/traeger/holiday-periods/5/registrations/7/proofs/91?inline=1",
    );
    expect(open).toHaveAttribute("target", "_blank");
    expect(open).toHaveAttribute("rel", "noopener");
    expect(open).toHaveAttribute("title", "In neuem Tab öffnen");
    // Öffnen und Herunterladen wie bei den PDFs der Hinweise (#5684).
    expect(
      card.getByRole("link", { name: "Bescheid Bürgergeld.pdf öffnen" }),
    ).toHaveAttribute(
      "href",
      "/api/traeger/holiday-periods/5/registrations/7/proofs/91?inline=1",
    );
    expect(
      card.getByRole("link", { name: "Bescheid Bürgergeld.pdf herunterladen" }),
    ).toHaveAttribute(
      "href",
      "/api/traeger/holiday-periods/5/registrations/7/proofs/91",
    );
  });

  it("asks the office to collect a proof the parents did not upload (#5238)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      benefitsDeclared: true,
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const heading = await screen.findByRole("heading", {
      name: "Nachweis für den Leistungsbezug",
    });
    expect(
      within(heading.closest("section")!).getByText(
        /Die Eltern haben keinen Nachweis hochgeladen/,
      ),
    ).toBeInTheDocument();
  });

  it("offers the upload of a paper proof without the statement on benefits (#5238, #5627)", async () => {
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    expect(
      await screen.findByRole("heading", { name: "Eltern" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Nachweis für den Leistungsbezug" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Kein Nachweis. Bringt eine Familie mit Leistungsbezug den Nachweis auf Papier, laden Sie ihn hier hoch.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Nachweis hochladen" }),
    ).toBeInTheDocument();
  });

  it("says when moto deletes the registration and downloads it as a PDF (#6691)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      deletesOn: "2028-01-09",
      healthDeletesOn: "2027-04-11",
    });
    const blob = new Blob(["%PDF-"]);
    mockDownloadPdf.mockResolvedValue({
      blob,
      filename: "ferienanmeldung-7-muster.pdf",
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    expect(
      await screen.findByText(
        "moto löscht diese Anmeldung am 09.01.2028. Laden Sie sie vorher herunter, wenn Sie sie länger brauchen. Gesundheitsangaben löscht moto schon am 11.04.2027.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Anmeldung herunterladen" }),
    );
    // Without the right to see health information there is no such file.
    expect(
      screen.queryByRole("menuitem", { name: "PDF mit Gesundheitsangaben" }),
    ).not.toBeInTheDocument();
    fireEvent.click(await screen.findByRole("menuitem", { name: "PDF" }));
    await waitFor(() =>
      expect(mockDownloadBlob).toHaveBeenCalledWith(
        blob,
        "ferienanmeldung-7-muster.pdf",
      ),
    );
    expect(mockDownloadPdf).toHaveBeenCalledWith("5", "7", false);
  });

  it("downloads the PDF with health information only with the right to see it (#6691)", async () => {
    sessionPermissions.current = ["traeger_health:read"];
    mockHealth.mockResolvedValue({ customData: {}, children: [] });
    mockDownloadPdf.mockResolvedValue({
      blob: new Blob(["%PDF-"]),
      filename: null,
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldung herunterladen" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "PDF mit Gesundheitsangaben",
      }),
    );
    await waitFor(() =>
      expect(mockDownloadPdf).toHaveBeenCalledWith("5", "7", true),
    );
    expect(mockDownloadBlob).toHaveBeenCalledWith(
      expect.any(Blob),
      "ferienanmeldung-7.pdf",
    );
  });

  it("shows the declaration with name, custody, version and its texts (#5626)", async () => {
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const parents = within(
      (await screen.findByRole("heading", { name: "Eltern" })).closest(
        "section",
      )!,
    );
    const field = (label: string) =>
      parents.getByText(label).closest("div")!.querySelector("dd");
    expect(field("Sorgerecht bestätigt")).toHaveTextContent(
      "Ja, am 03.09.2026",
    );
    expect(field("Name als Erklärung")).toHaveTextContent("Eva Muster");
    expect(field("Fassung der Texte")).toHaveTextContent("Fassung 4");

    const wording = screen.getByRole("heading", {
      name: "Wortlaut der Erklärung",
    });
    fireEvent.click(
      within(wording.closest("section")!).getByRole("button", {
        expanded: false,
      }),
    );
    const texts = within(wording.closest("section")!);
    expect(
      texts.getByText(HOLIDAY_CONSENT_TEXTS.custodyConfirmed),
    ).toBeInTheDocument();
    expect(
      texts.getByText(HOLIDAY_CONSENT_TEXTS.privacyConsent),
    ).toBeInTheDocument();
    expect(
      texts.getByText(HOLIDAY_CONSENT_TEXTS.declarationName),
    ).toBeInTheDocument();
  });

  it("names a registration from before the declaration without texts (#5626)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      custodyConfirmedAt: null,
      declarationName: "",
      consentVersion: 0,
      consentTexts: null,
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const parents = within(
      (await screen.findByRole("heading", { name: "Eltern" })).closest(
        "section",
      )!,
    );
    expect(
      parents.getByText("Vor Fassung 2, Wortlaut nicht gespeichert"),
    ).toBeInTheDocument();
    expect(parents.getByText("Nicht angegeben")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Anmeldung von Eva Muster" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Wortlaut der Erklärung" }),
    ).not.toBeInTheDocument();
  });

  it("shows the second guardian only in its own contact card", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      secondGuardian: {
        firstName: "Tom",
        lastName: "Anders",
        email: "tom@example.org",
        phone: "0151 9999999",
      },
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const heading = await screen.findByRole("heading", {
      name: "Zweite sorgeberechtigte Person",
    });
    const second = within(heading.closest("section")!);
    expect(second.getByText("Tom Anders")).toBeInTheDocument();
    expect(second.getByText("tom@example.org")).toBeInTheDocument();
    expect(second.getByText("0151 9999999")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Anmeldung von Eva Muster" }),
    ).toBeInTheDocument();
  });

  it("shows the postal address with the contact details of the parent (#5625)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      guardianAddress: {
        street: "Musterweg 12",
        postalCode: "01234",
        city: "Bissendorf",
      },
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const heading = await screen.findByRole("heading", { name: "Eltern" });
    const parents = within(heading.closest("section")!);
    expect(parents.getByText("Anschrift")).toBeInTheDocument();
    expect(
      parents.getByText("Musterweg 12, 01234 Bissendorf"),
    ).toBeInTheDocument();
  });

  it("names no address when the registration has none (#5625)", async () => {
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const heading = await screen.findByRole("heading", { name: "Eltern" });
    const parents = within(heading.closest("section")!);
    expect(parents.getByText("eva@example.org")).toBeInTheDocument();
    expect(parents.queryByText("Anschrift")).not.toBeInTheDocument();
  });

  it("reads the health information only with the right „Gesundheitsangaben sehen“ (#4722)", async () => {
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    // Anker: die Anmeldung ist geladen und die Karten sagen, wer die Angaben sieht.
    expect(
      (
        await screen.findAllByText(
          "Gesundheitsangaben sieht nur die Rolle Verwaltung.",
        )
      ).length,
    ).toBeGreaterThan(0);
    expect(mockHealth).not.toHaveBeenCalled();
    expect(screen.queryByText("Erdnüsse")).not.toBeInTheDocument();
  });

  it("shows the health information to the right (#4722)", async () => {
    sessionPermissions.current = [
      "traeger_enrollment:manage",
      "traeger_health:read",
    ];
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    // Anker: die Gesundheitsangaben sind geladen.
    expect(await screen.findByText("Erdnüsse")).toBeInTheDocument();
    expect(mockHealth).toHaveBeenCalledTimes(1);
    expect(
      screen.queryByText("Gesundheitsangaben sieht nur die Rolle Verwaltung."),
    ).not.toBeInTheDocument();
  });

  it("links to billing settings only with the right Abrechnung (#5685)", async () => {
    sessionPermissions.current = ["traeger_enrollment:manage"];
    const page = render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    // Anker: Die angewandte Geschwisterregel ist geladen.
    expect(
      await screen.findByText("Geschwister: 2. Kind, 50 % weniger"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Abrechnungsvorgaben" }),
    ).not.toBeInTheDocument();

    sessionPermissions.current = [
      "traeger_enrollment:manage",
      "traeger_overview:read",
    ];
    page.rerender(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    // Anker: Die angewandte Geschwisterregel ist weiter geladen.
    expect(
      await screen.findByText("Geschwister: 2. Kind, 50 % weniger"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Abrechnungsvorgaben" }),
    ).not.toBeInTheDocument();

    sessionPermissions.current = [
      "traeger_enrollment:manage",
      "traeger_billing:manage",
    ];
    page.rerender(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    expect(
      await screen.findByRole("link", { name: "Abrechnungsvorgaben" }),
    ).toHaveAttribute("href", "/traeger/einstellungen/abrechnung");
  });

  it("shows the answers with the labels of the form version the parents answered (#5198)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      form: {
        version: 2,
        fields: [
          {
            key: "quelle",
            label: "Wie haben Sie von den Ferien erfahren?",
            type: "text",
            sort_order: 0,
          },
          {
            key: "tetanus",
            label: "Tetanus-Impfschutz",
            type: "select",
            applies_to_child: true,
            health: true,
            sort_order: 1,
            options: [{ label: "weiß nicht", value: "weiss_nicht" }],
          },
          {
            key: "ausfluege",
            label: "Darf an Ausflügen teilnehmen",
            type: "boolean",
            applies_to_child: true,
            sort_order: 2,
          },
        ],
      },
      answers: { quelle: "Elternbrief" },
      childAnswers: { "9": { tetanus: "weiss_nicht", ausfluege: false } },
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );

    expect(
      await screen.findByText(
        "Antworten auf die Fragen des Formulars, Version 2. Antworten zu einem Kind stehen beim Kind.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Wie haben Sie von den Ferien erfahren?"),
    ).toBeInTheDocument();
    expect(screen.getByText("Elternbrief")).toBeInTheDocument();
    // A health answer carries its marker; the label comes from version 2.
    expect(
      screen.getByText("Tetanus-Impfschutz (Gesundheit)"),
    ).toBeInTheDocument();
    expect(screen.getByText("weiß nicht")).toBeInTheDocument();
    expect(
      screen.getByText("Darf an Ausflügen teilnehmen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Nein")).toBeInTheDocument();
    // Ben answered nothing: only Lia's card lists answers.
    expect(screen.getAllByText("Weitere Angaben")).toHaveLength(1);
    expect(screen.getByText("Allgemeine Angaben")).toBeInTheDocument();
  });
  it("shows pinned legal acknowledgements with Berlin time and German PDF actions", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      form: {
        version: 2,
        fields: [],
        legalBlocks: [
          {
            key: "hinweis",
            kind: "terms",
            source: "custom",
            enabled: true,
            required: true,
            title: "Damals gültiger Ferienbrief",
            label: "Gelesen",
            text: "Der **historische Ferienbrief** bleibt verbindlich.",
            sort_order: 0,
            attachments: [{ label: "Ferienbrief", document_url: "81" }],
          },
          {
            key: "foto",
            kind: "consent",
            source: "custom",
            enabled: true,
            required: false,
            title: "Fotos",
            label: "Fotos erlaubt",
            text: "",
            sort_order: 1,
          },
          {
            key: "info",
            kind: "notice",
            source: "custom",
            enabled: true,
            required: false,
            title: "Trinkflaschen",
            label: "Bitte eine Trinkflasche mitgeben.",
            text: "",
            sort_order: 2,
            attachments: [{ label: "Packliste", document_url: "82" }],
          },
        ],
      },
      legalAcceptances: { hinweis: "2026-11-03T10:12:00Z" },
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    const title = await screen.findByRole("heading", {
      name: "Hinweise und Einwilligungen",
    });
    const evidence = within(title.closest("section")!);
    expect(
      evidence.getByText("Damals gültiger Ferienbrief"),
    ).toBeInTheDocument();
    expect(evidence.getByText("Gelesen")).toBeInTheDocument();
    expect(evidence.getByText("historische Ferienbrief")).toHaveProperty(
      "tagName",
      "STRONG",
    );
    expect(
      evidence.getByText("Zugestimmt am 03.11.2026, 11:12 Uhr"),
    ).toBeInTheDocument();
    expect(evidence.getAllByText("Nicht zugestimmt")).toHaveLength(1);
    // Anker: the information-only notice and its PDF remain visible.
    const notice = within(
      evidence.getByRole("heading", { name: "Trinkflaschen" }).parentElement!,
    );
    expect(
      notice.getByRole("button", { name: "Packliste öffnen" }),
    ).toBeInTheDocument();
    expect(notice.queryByText("Nicht zugestimmt")).not.toBeInTheDocument();
    expect(
      evidence.getByRole("button", { name: "Ferienbrief öffnen" }),
    ).toHaveTextContent("Öffnen");
    expect(
      evidence.getByRole("button", { name: "Ferienbrief herunterladen" }),
    ).toHaveTextContent("Herunterladen");
  });
});

describe("TraegerHolidayRegistrationPage, Angaben ändern (#5629)", () => {
  beforeEach(() => {
    mockRegistration.mockResolvedValue(HOLIDAY_REGISTRATION);
    mockConfirmationSettings.mockResolvedValue({ debtorNumberRequired: false });
    mockEdit.mockReset();
    sessionPermissions.current = [];
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
    vi.mocked(useTenantSlugSafe).mockReturnValue(null);
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
  });

  async function openParentsEdit() {
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Angaben der Eltern ändern" }),
    );
    const heading = await screen.findByRole("heading", {
      name: "Angaben ändern",
    });
    return within(heading.closest("section")!);
  }

  it("changes the parents in place, shows a refused field at the field and the entry in the history", async () => {
    const edit = await openParentsEdit();
    expect(
      await edit.findByText(
        "Die Änderung steht im Verlauf. Ein schon erstelltes Schreiben an die Familie bleibt, wie es ist.",
      ),
    ).toBeInTheDocument();
    // Only one card is edited at a time.
    expect(
      screen.queryByRole("button", {
        name: "Notfallkontakte und Abholung ändern",
      }),
    ).not.toBeInTheDocument();
    expect(edit.getByLabelText(/^Telefon/)).toHaveValue("0151 2345678");

    mockEdit.mockRejectedValueOnce(
      new ApiError("refused", 400, {
        code: "enrollment.holiday_registration_invalid",
        errors: [{ field: "guardian.email", reason: "invalid" }],
      }),
    );
    fireEvent.change(edit.getByLabelText(/^E-Mail/), {
      target: { value: "eva@" },
    });
    fireEvent.click(edit.getByRole("button", { name: "Änderung speichern" }));
    await waitFor(() =>
      expect(edit.getByLabelText(/^E-Mail/)).toHaveAttribute(
        "aria-invalid",
        "true",
      ),
    );
    expect(edit.getByLabelText(/^Telefon/)).not.toHaveAttribute("aria-invalid");

    mockEdit.mockResolvedValueOnce({
      ...HOLIDAY_REGISTRATION,
      guardianEmail: "eva.muster@example.org",
      guardianPhone: "0151 9999999",
      history: [
        {
          action: "edit",
          occurredAt: "2026-11-03T10:12:00Z",
          from: "",
          childId: "",
          before: [],
          after: [],
          fields: ["guardian.email", "guardian.phone", "children.0.last_name"],
        },
      ],
    });
    fireEvent.change(edit.getByLabelText(/^E-Mail/), {
      target: { value: "eva.muster@example.org" },
    });
    fireEvent.change(edit.getByLabelText(/^Telefon/), {
      target: { value: "0151 9999999" },
    });
    fireEvent.click(edit.getByRole("button", { name: "Änderung speichern" }));

    expect(
      await screen.findByText(
        ": E-Mail des Elternteils, Telefon des Elternteils, Nachname von Kind 1",
        { exact: false },
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Angaben geändert")).toBeInTheDocument();
    expect(mockEdit).toHaveBeenLastCalledWith("5", "7", {
      guardians: {
        guardian: {
          firstName: "Eva",
          lastName: "Muster",
          email: "eva.muster@example.org",
          phone: "0151 9999999",
        },
        address: null,
        secondGuardian: null,
      },
    });
    // Back in the display: the card shows the saved values.
    expect(screen.getByText("0151 9999999")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Angaben ändern" }),
    ).not.toBeInTheDocument();
  });

  it("changes one child and sends no health answer", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      form: {
        version: 2,
        fields: [
          {
            key: "tetanus",
            label: "Tetanus-Impfschutz",
            type: "select",
            applies_to_child: true,
            health: true,
            sort_order: 0,
            options: [{ label: "Ja", value: "ja" }],
          },
          {
            key: "lieblingsessen",
            label: "Lieblingsessen",
            type: "text",
            applies_to_child: true,
            sort_order: 1,
          },
          {
            key: "abholdatum",
            label: "Erster Abholtag",
            type: "date",
            applies_to_child: true,
            help_text: "Der Tag, ab dem jemand anders abholt.",
            sort_order: 3,
          },
          {
            key: "gruppe",
            label: "Wunschgruppe",
            type: "select",
            applies_to_child: true,
            sort_order: 2,
            options: [{ label: "Bären", value: "baeren" }],
          },
        ],
      },
      childAnswers: { "9": { lieblingsessen: "Nudeln" } },
    });
    mockEdit.mockResolvedValueOnce(HOLIDAY_REGISTRATION);
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Angaben von Lia Muster ändern",
      }),
    );
    const heading = await screen.findByRole("heading", {
      name: "Angaben ändern",
    });
    const edit = within(heading.closest("section")!);
    // Anker: the editor has loaded and asks the child's own question.
    expect(await edit.findByLabelText(/^Lieblingsessen/)).toHaveValue("Nudeln");
    expect(edit.queryByText("Tetanus-Impfschutz")).not.toBeInTheDocument();
    // A date question keeps its help text and loads the kit date picker.
    expect(
      edit.getByText("Der Tag, ab dem jemand anders abholt."),
    ).toBeInTheDocument();
    expect(await edit.findByLabelText(/^Erster Abholtag/)).toBeInTheDocument();
    // An unanswered choice asks in German, not with the key of its text.
    expect(edit.getByText("Bitte wählen")).toBeInTheDocument();
    expect(edit.queryByText("fields.choose")).not.toBeInTheDocument();
    fireEvent.change(edit.getByLabelText(/^Nachname/), {
      target: { value: "Mustermann" },
    });
    fireEvent.click(edit.getByRole("button", { name: "Änderung speichern" }));
    await waitFor(() =>
      expect(mockEdit).toHaveBeenCalledWith("5", "7", {
        child: {
          id: "9",
          firstName: "Lia",
          lastName: "Mustermann",
          birthDate: "2018-03-01",
          homeSchoolId: "31",
          guestSchoolName: "",
          answers: { lieblingsessen: "Nudeln" },
        },
      }),
    );
  });

  it.each([
    { homeSchoolId: "99", label: "Bisherige Schule (gelöscht)", count: 4 },
    { homeSchoolId: "31", label: "OGS Nord", count: 3 },
    { homeSchoolId: null, label: "Andere Schule (Feriengast)", count: 3 },
  ])(
    "keeps the saved school $homeSchoolId selectable after a change",
    async ({ homeSchoolId, label, count }) => {
      const registration = {
        ...HOLIDAY_REGISTRATION,
        children: HOLIDAY_REGISTRATION.children.map((child) =>
          child.id === "9" ? { ...child, homeSchoolId } : child,
        ),
      };
      mockRegistration.mockResolvedValue(registration);
      mockEdit.mockResolvedValueOnce(registration);
      render(
        <ToastProvider>
          <TraegerHolidayRegistrationPage />
        </ToastProvider>,
      );
      fireEvent.click(
        await screen.findByRole("button", {
          name: "Angaben von Lia Muster ändern",
        }),
      );
      const school = await screen.findByRole("combobox", { name: /^Schule/ });
      expect(school).toHaveTextContent(label);
      fireEvent.click(school);
      expect(screen.getAllByRole("option")).toHaveLength(count);
      fireEvent.click(screen.getByRole("option", { name: "OGS Süd" }));
      expect(school).toHaveTextContent("OGS Süd");
      fireEvent.click(school);
      fireEvent.click(screen.getByRole("option", { name: label }));
      expect(school).toHaveTextContent(label);
      fireEvent.click(
        screen.getByRole("button", { name: "Änderung speichern" }),
      );
      await waitFor(() =>
        expect(mockEdit).toHaveBeenCalledWith("5", "7", {
          child: expect.objectContaining({ id: "9", homeSchoolId }),
        }),
      );
    },
  );

  it("drops a contact entry whose fields are all empty", async () => {
    mockEdit.mockResolvedValueOnce(HOLIDAY_REGISTRATION);
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Notfallkontakte und Abholung ändern",
      }),
    );
    const heading = await screen.findByRole("heading", {
      name: "Angaben ändern",
    });
    const edit = within(heading.closest("section")!);
    fireEvent.click(
      await edit.findByRole("button", { name: "Abholperson hinzufügen" }),
    );
    expect(
      edit.getByRole("group", { name: "Abholberechtigte" }),
    ).toContainElement(
      within(
        edit.getByRole("group", { name: "Abholberechtigte" }),
      ).getByLabelText(/^Name/),
    );
    fireEvent.click(edit.getByRole("button", { name: "Änderung speichern" }));
    await waitFor(() =>
      expect(mockEdit).toHaveBeenCalledWith("5", "7", {
        contacts: {
          emergency: [
            {
              name: "Oma Muster",
              phone: "05402 1234",
              relationship: "Großmutter",
            },
          ],
          pickup: [],
        },
      }),
    );
  });

  it("changes the postal address with the rules of its form and says an invoice keeps the old one (#5625)", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      status: "confirmed",
      guardianAddress: {
        street: "Musterweg 12",
        postalCode: "01234",
        city: "Bissendorf",
      },
      form: { ...HOLIDAY_REGISTRATION.form, addressRequirement: "required" },
    });
    contributionOverride.current = { ...HOLIDAY_CONTRIBUTION, invoiced: true };
    mockEdit.mockResolvedValueOnce({
      ...HOLIDAY_REGISTRATION,
      history: [
        {
          action: "edit",
          occurredAt: "2026-11-03T10:12:00Z",
          from: "",
          childId: "",
          before: [],
          after: [],
          debtorNumber: null,
          fields: ["guardian_address"],
        },
      ],
    });
    const edit = await openParentsEdit();
    expect(
      await edit.findByText(
        "Für diese Anmeldung gibt es schon eine Rechnung. Die Rechnung behält die bisherige Anschrift.",
      ),
    ).toBeInTheDocument();
    expect(edit.getByLabelText(/^Straße und Hausnummer/)).toHaveValue(
      "Musterweg 12",
    );
    expect(edit.getByLabelText(/^PLZ/)).toBeRequired();
    fireEvent.change(edit.getByLabelText(/^PLZ/), {
      target: { value: "49143" },
    });
    fireEvent.click(edit.getByRole("button", { name: "Änderung speichern" }));
    await waitFor(() =>
      expect(mockEdit).toHaveBeenCalledWith("5", "7", {
        guardians: {
          guardian: {
            firstName: "Eva",
            lastName: "Muster",
            email: "eva@example.org",
            phone: "0151 2345678",
          },
          address: {
            street: "Musterweg 12",
            postalCode: "49143",
            city: "Bissendorf",
          },
          secondGuardian: null,
        },
      }),
    );
    expect(
      await screen.findByText(": Anschrift", { exact: false }),
    ).toBeInTheDocument();
    contributionOverride.current = null;
  });

  it("offers no „Ändern“ on a cancelled registration and says why", async () => {
    mockRegistration.mockResolvedValue({
      ...HOLIDAY_REGISTRATION,
      status: "cancelled",
    });
    render(
      <ToastProvider>
        <TraegerHolidayRegistrationPage />
      </ToastProvider>,
    );
    expect(
      await screen.findByText(
        "Die Anmeldung ist storniert. Die Angaben lassen sich nicht mehr ändern.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /ändern$/ }),
    ).not.toBeInTheDocument();
  });
});

function render(ui: React.ReactElement) {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>{ui}</SWRConfig>,
  );
}

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: {
        id: "A",
        scope: "org",
        orgId: 7,
        permissions: sessionPermissions.current,
      },
    },
    status: "authenticated",
  }),
}));
