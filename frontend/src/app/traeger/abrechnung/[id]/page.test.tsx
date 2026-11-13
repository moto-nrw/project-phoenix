import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { BillingRunDetail } from "~/lib/traeger-billing-api";
import { catalogText } from "~/test/error-catalog-text";

const { api } = vi.hoisted(() => ({
  api: {
    fetchBillingRun: vi.fn(),
    correctBillingRun: vi.fn(),
    releaseBillingRun: vi.fn(),
    createBillingLetters: vi.fn(),
    previewBillingLetter: vi.fn(),
    downloadBillingLetter: vi.fn(),
    fetchLetterMailings: vi.fn(),
    mailBillingLetters: vi.fn(),
    mailBillingLetter: vi.fn(),
    printBillingLetters: vi.fn(),
    downloadBillingRunOverview: vi.fn(),
    cancelBillingDocument: vi.fn(),
    changeCarriedCorrection: vi.fn(),
    removeCarriedCorrection: vi.fn(),
    creditBillingDocument: vi.fn(),
    fetchBillingRuns: vi.fn(),
    startBillingRun: vi.fn(),
  },
}));
const { push } = vi.hoisted(() => ({ push: vi.fn() }));
const { session } = vi.hoisted(() => ({
  session: { permissions: [] as string[] },
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { permissions: session.permissions } },
    status: "authenticated",
  }),
}));
const { downloadBlob } = vi.hoisted(() => ({ downloadBlob: vi.fn() }));
vi.mock("~/lib/file-download", () => ({ downloadBlob }));
// The recipient of the handover (ADR 0218) is tested on its own.
vi.mock("~/lib/traeger-export-handovers-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  fetchExportRecipients: () => new Promise(() => undefined),
}));

vi.mock("~/lib/traeger-billing-api", () => api);
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "9" }),
  useRouter: () => ({ push }),
  usePathname: () => "/abrechnung/9",
}));

import TraegerBillingRunPage from "./page";

const PREVIEW: BillingRunDetail = {
  id: "9",
  holidayPeriodId: "5",
  periodName: "Winterferien 2027",
  serviceFrom: "2027-01-04",
  serviceUntil: "2027-01-15",
  status: "calculated",
  createdAt: "2026-09-01T08:00:00Z",
  releasedAt: null,
  issuedOn: null,
  documentCount: null,
  totalCents: null,
  previewCount: 1,
  previewCents: 11250,
  unbilled: 1,
  missingDebtorNumbers: 0,
  paymentTextMissingDebtorNumbers: 0,
  numberPattern: "FE-{NNNN}",
  numberRangeSet: true,
  fingerprint: "c0ffee",
  letters: null,
  documents: [
    {
      id: "",
      kind: "invoice",
      referenceNumber: "",
      reason: "",
      history: [],
      replacedCredits: [],
      number: "",
      issuedOn: null,
      registrationId: "31",
      recipientFirstName: "Eva",
      recipientLastName: "Muster",
      recipientEmail: "eva@example.org",
      debtorNumber: "",
      serviceFrom: "2027-01-04",
      serviceUntil: "2027-01-15",
      totalCents: 11250,
      children: [
        {
          childId: "41",
          firstName: "Lia",
          lastName: "Muster",
          totalCents: 6750,
          corrections: [],
          carriedCorrections: [],
          positions: [
            {
              kind: "base",
              amountCents: 6750,
              weekStartsOn: "2027-01-04",
              weekEndsOn: "2027-01-08",
              timeModelName: "bis 16 Uhr",
              childRank: null,
              discountBasisPoints: null,
              discountCents: null,
              note: "",
            },
          ],
        },
        {
          childId: "42",
          firstName: "Ben",
          lastName: "Muster",
          totalCents: 4500,
          corrections: [],
          carriedCorrections: [],
          positions: [
            {
              kind: "base",
              amountCents: 4500,
              weekStartsOn: "2027-01-04",
              weekEndsOn: "2027-01-08",
              timeModelName: "bis 13 Uhr",
              childRank: null,
              discountBasisPoints: null,
              discountCents: null,
              note: "",
            },
          ],
        },
      ],
    },
  ],
};

const CORRECTED: BillingRunDetail = {
  ...PREVIEW,
  previewCents: 9750,
  documents: [
    {
      ...PREVIEW.documents[0]!,
      totalCents: 9750,
      children: [
        {
          ...PREVIEW.documents[0]!.children[0]!,
          totalCents: 5250,
          positions: [
            ...PREVIEW.documents[0]!.children[0]!.positions,
            {
              kind: "correction",
              amountCents: -1500,
              weekStartsOn: null,
              weekEndsOn: null,
              timeModelName: "",
              childRank: null,
              discountBasisPoints: null,
              discountCents: null,
              note: "Erster Tag krank",
            },
          ],
          corrections: [
            {
              id: "1",
              amountCents: -1500,
              totalBeforeCents: 6750,
              totalAfterCents: 5250,
              reason: "Erster Tag krank",
              createdAt: "2026-09-09T10:00:00Z",
              carriedFromNumber: "",
            },
          ],
        },
        PREVIEW.documents[0]!.children[1]!,
      ],
    },
  ],
};

function renderPage() {
  render(
    <ToastProvider>
      <TraegerBillingRunPage />
    </ToastProvider>,
  );
}

/** The table renders a desktop row and a stacked phone row (#5146). */
async function openInvoiceMenu() {
  const [menu] = await screen.findAllByRole("button", {
    name: "Aktionen für die Rechnung an Eva Muster",
  });
  fireEvent.click(menu!);
}

/** Opens the breakdown of the invoice in the side panel (#5146). */
async function showBreakdown() {
  await openInvoiceMenu();
  fireEvent.click(
    await screen.findByRole("menuitem", { name: "Aufschlüsselung ansehen" }),
  );
  return screen.findByRole("dialog", { name: "Rechnung an Eva Muster" });
}

describe("TraegerBillingRunPage", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    downloadBlob.mockReset();
    api.fetchBillingRun.mockResolvedValue(PREVIEW);
    api.fetchLetterMailings.mockResolvedValue([]);
    session.permissions = ["traeger_billing:manage"];
  });

  it("leads back to its holiday period for whoever may open it (#5688)", async () => {
    session.permissions = [
      "traeger_billing:manage",
      "traeger_enrollment:manage",
    ];
    renderPage();

    expect(
      await screen.findByRole("link", { name: "Zum Ferienzeitraum" }),
    ).toHaveAttribute("href", "/ferien/5");
  });

  it("shows no link to a holiday period the person cannot open", async () => {
    renderPage();

    // Anker: der Lauf ist geladen, seine Statuszeile steht.
    expect((await screen.findAllByText("Muster, Eva"))[0]).toBeInTheDocument();
    expect(screen.getAllByText("Vorschau")[0]).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zum Ferienzeitraum" }),
    ).not.toBeInTheDocument();
  });

  it("shows the preview of every family with its positions", async () => {
    renderPage();

    expect((await screen.findAllByText("Muster, Eva"))[0]).toBeInTheDocument();
    expect(screen.getAllByText("Entwurf ohne Nummer")[0]).toBeInTheDocument();
    const panel = await showBreakdown();
    expect(panel).toHaveTextContent(/Entwurf ohne Nummer/);
    expect(within(panel).getByText("Lia Muster")).toBeInTheDocument();
    expect(
      within(panel).getAllByText("Rechnungsbetrag")[0],
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /1 bestätigte Anmeldung\(en\) haben keinen festgehaltenen Beitrag/,
      ),
    ).toBeInTheDocument();
  });

  it("corrects the amount of a child only with a reason", async () => {
    api.correctBillingRun.mockResolvedValue(CORRECTED);
    renderPage();
    await openInvoiceMenu();
    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "Betrag für Lia korrigieren",
      }),
    );
    fireEvent.change(await screen.findByLabelText("Neuer Betrag in Euro"), {
      target: { value: "52,50" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Korrigieren" }));
    expect(
      await screen.findByText("Bitte geben Sie eine Begründung an."),
    ).toBeInTheDocument();
    expect(api.correctBillingRun).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Erster Tag krank" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Korrigieren" }));

    await waitFor(() =>
      expect(api.correctBillingRun).toHaveBeenCalledWith("9", {
        registrationId: "31",
        childId: "41",
        totalCents: 5250,
        reason: "Erster Tag krank",
      }),
    );
    await waitFor(() =>
      expect(screen.queryByLabelText("Begründung")).not.toBeInTheDocument(),
    );
    const panel = await showBreakdown();
    expect(
      within(panel).getByText("Korrektur: Erster Tag krank"),
    ).toBeInTheDocument();
    expect(within(panel).getByText("Korrekturen für Lia")).toBeInTheDocument();
  });

  it("changes a correction taken over from the cancelled invoice (#5663)", async () => {
    const child = PREVIEW.documents[0]!.children[0]!;
    const carried = {
      positionId: "61",
      sourceNumber: "FE-0001",
      amountCents: -1500,
      sourceAmountCents: -1500,
      reason: "Erster Tag krank",
    };
    api.fetchBillingRun.mockResolvedValue({
      ...PREVIEW,
      documents: [
        {
          ...PREVIEW.documents[0]!,
          referenceNumber: "FE-0001",
          children: [
            { ...child, totalCents: 5250, carriedCorrections: [carried] },
            PREVIEW.documents[0]!.children[1]!,
          ],
        },
      ],
    });
    api.changeCarriedCorrection.mockResolvedValue(CORRECTED);
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", {
        name: /^Aktionen für die Korrektur von Lia über /,
      }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Ändern" }));
    const amount = await screen.findByLabelText("Neuer Betrag in Euro");
    expect(amount).toHaveValue("52,50");
    expect(screen.getByLabelText("Begründung")).toHaveValue("Erster Tag krank");

    // The amount without the correction is what removing it does.
    fireEvent.change(amount, { target: { value: "67,50" } });
    fireEvent.click(screen.getByRole("button", { name: "Ändern" }));
    expect(
      await screen.findByText(/Das ist der Betrag ohne Korrektur/),
    ).toBeInTheDocument();
    expect(api.changeCarriedCorrection).not.toHaveBeenCalled();

    fireEvent.change(amount, { target: { value: "60,00" } });
    fireEvent.click(screen.getByRole("button", { name: "Ändern" }));
    await waitFor(() =>
      expect(api.changeCarriedCorrection).toHaveBeenCalledWith("9", "61", {
        totalCents: 6000,
        reason: "Erster Tag krank",
      }),
    );
  });

  it("uses the recalculated amount without a carried correction (#5663)", async () => {
    const child = PREVIEW.documents[0]!.children[0]!;
    const carried = [
      {
        positionId: "61",
        sourceNumber: "FE-0001",
        amountCents: -8000,
        sourceAmountCents: -8000,
        reason: "Erster Tag krank",
      },
      {
        positionId: "62",
        sourceNumber: "FE-0001",
        amountCents: -2000,
        sourceAmountCents: -12000,
        reason: "Zweiter Tag krank",
      },
    ];
    api.fetchBillingRun.mockResolvedValue({
      ...PREVIEW,
      documents: [
        {
          ...PREVIEW.documents[0]!,
          referenceNumber: "FE-0001",
          children: [
            {
              ...child,
              totalCents: 5000,
              positions: [{ ...child.positions[0]!, amountCents: 10000 }],
              corrections: [
                {
                  id: "1",
                  amountCents: 5000,
                  totalBeforeCents: 0,
                  totalAfterCents: 5000,
                  reason: "Nachberechnung",
                  createdAt: "2026-09-01T08:00:00Z",
                  carriedFromNumber: "",
                },
              ],
              carriedCorrections: carried,
            },
            PREVIEW.documents[0]!.children[1]!,
          ],
        },
      ],
    });
    api.changeCarriedCorrection.mockResolvedValue(CORRECTED);
    renderPage();
    const [menu] = await screen.findAllByRole("button", {
      name: /^Aktionen für die Korrektur von Lia über /,
    });
    fireEvent.click(menu!);
    fireEvent.click(await screen.findByRole("menuitem", { name: "Ändern" }));
    const amount = await screen.findByLabelText("Neuer Betrag in Euro");

    fireEvent.change(amount, { target: { value: "50,00" } });
    fireEvent.click(screen.getByRole("button", { name: "Ändern" }));
    expect(
      await screen.findByText(/Das ist der Betrag ohne Korrektur/),
    ).toBeInTheDocument();
    expect(api.changeCarriedCorrection).not.toHaveBeenCalled();

    fireEvent.change(amount, { target: { value: "80,00" } });
    fireEvent.click(screen.getByRole("button", { name: "Ändern" }));
    await waitFor(() =>
      expect(api.changeCarriedCorrection).toHaveBeenCalledWith("9", "61", {
        totalCents: 8000,
        reason: "Erster Tag krank",
      }),
    );
  });

  it("releases with the checked preview and reports a changed run", async () => {
    api.releaseBillingRun.mockRejectedValue(
      new ApiError("changed", 409, {
        code: "parent_billing.billing_run_changed",
      }),
    );
    renderPage();
    await screen.findAllByText("Muster, Eva");

    fireEvent.click(screen.getByRole("button", { name: "Lauf freigeben" }));
    expect(
      await screen.findByText(
        /1 Rechnung\(en\) über 112,50 € bekommen ihre Nummer/,
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await waitFor(() =>
      expect(api.releaseBillingRun).toHaveBeenCalledWith(PREVIEW),
    );
    expect(
      await screen.findByText(
        catalogText("parent_billing.billing_run_changed", "der Lauf"),
      ),
    ).toBeInTheDocument();
    await waitFor(() => expect(api.fetchBillingRun).toHaveBeenCalledTimes(2));
  });

  it("names the invoices without a debtor number only while the Träger asks for it (#5621)", async () => {
    api.fetchBillingRun.mockResolvedValue({
      ...PREVIEW,
      missingDebtorNumbers: 3,
    });
    renderPage();

    expect(
      await screen.findByText(
        "3 Anmeldungen haben noch keine Debitorennummer. Tragen Sie sie vor dem Freigeben ein.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Lauf freigeben" }),
    ).toBeEnabled();
  });

  it("shows no debtor number hint when the switch is off (#5621)", async () => {
    api.fetchBillingRun.mockResolvedValue({
      ...PREVIEW,
      missingDebtorNumbers: 0,
    });
    renderPage();

    expect((await screen.findAllByText("Muster, Eva"))[0]).toBeInTheDocument();
    expect(screen.queryByText(/Debitorennummer/)).not.toBeInTheDocument();
  });

  it("warns before the release when the payment text needs missing debtor numbers (#5622)", async () => {
    const run = {
      ...PREVIEW,
      missingDebtorNumbers: 3,
      paymentTextMissingDebtorNumbers: 2,
    };
    api.fetchBillingRun.mockResolvedValue(run);
    api.releaseBillingRun.mockResolvedValue({ ...run, status: "released" });
    renderPage();

    const warning = await screen.findByText(
      "2 Rechnungen mit Betrag haben keine Debitorennummer. Ihr Zahlungstext nennt aber die Debitorennummer. Nach dem Freigeben können Sie sie nicht mehr eintragen. Schreiben gehen dann nur ohne den Platzhalter im Zahlungstext. Tragen Sie die Nummern vor dem Freigeben bei den Anmeldungen ein.",
    );
    expect(warning.closest('[role="alert"]')).toHaveTextContent(
      "Schreiben brauchen die Debitorennummer",
    );
    expect(
      screen.queryByText(/3 Anmeldungen haben noch keine Debitorennummer/),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Lauf freigeben" }));
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(
        "2 Rechnungen mit Betrag haben keine Debitorennummer. Ihr Zahlungstext nennt aber die Debitorennummer. Nach dem Freigeben können Sie sie nicht mehr eintragen. Schreiben gehen dann nur ohne den Platzhalter im Zahlungstext.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Trotzdem freigeben" }),
    );
    await waitFor(() =>
      expect(api.releaseBillingRun).toHaveBeenCalledWith(run),
    );
  });

  it("names the switch when the payment text needs a number the registrations do not ask for (#5622)", async () => {
    api.fetchBillingRun.mockResolvedValue({
      ...PREVIEW,
      paymentTextMissingDebtorNumbers: 1,
    });
    renderPage();

    expect(
      await screen.findByText(
        "1 Rechnung mit Betrag hat keine Debitorennummer. Ihr Zahlungstext nennt aber die Debitorennummer. Nach dem Freigeben können Sie sie nicht mehr eintragen. Schreiben gehen dann nur ohne den Platzhalter im Zahlungstext. Tragen Sie die Nummern vor dem Freigeben bei den Anmeldungen ein. Das Feld dafür erscheint, wenn in den Einstellungen unter „Allgemein“ im Bereich „Ferienbetreuung“ „Debitorennummer beim Bestätigen abfragen“ an ist.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Lauf freigeben" }));
    expect(
      await screen.findByRole("button", { name: "Trotzdem freigeben" }),
    ).toBeEnabled();
  });

  it("shows issued invoices without a way to change them", async () => {
    api.fetchBillingRun.mockResolvedValue({
      ...PREVIEW,
      status: "released",
      issuedOn: "2026-09-09",
      documentCount: 1,
      totalCents: 11250,
      unbilled: 0,
      documents: [
        { ...PREVIEW.documents[0]!, number: "FE-0001", issuedOn: "2026-09-09" },
      ],
    });
    renderPage();

    expect((await screen.findAllByText("FE-0001"))[0]).toBeInTheDocument();
    expect(
      screen.getByText(/lassen sich nicht mehr ändern/),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Muster, Eva")[0]).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Lauf freigeben" }),
    ).not.toBeInTheDocument();
    // The row menu only shows the breakdown; nothing changes the invoice.
    await openInvoiceMenu();
    const breakdown = await screen.findByRole("menuitem", {
      name: "Aufschlüsselung ansehen",
    });
    expect(
      screen.queryByRole("menuitem", { name: /korrigieren/ }),
    ).not.toBeInTheDocument();
    fireEvent.click(breakdown);
    expect(
      await screen.findByRole("dialog", { name: "Rechnung an Eva Muster" }),
    ).toHaveTextContent(/Rechnung FE-0001 vom 09.09.2026/);
  });

  const RELEASED: BillingRunDetail = {
    ...PREVIEW,
    status: "released",
    issuedOn: "2026-09-09",
    documentCount: 1,
    totalCents: 11250,
    unbilled: 0,
    documents: [
      {
        ...PREVIEW.documents[0]!,
        id: "51",
        number: "FE-0001",
        issuedOn: "2026-09-09",
      },
    ],
  };

  it("creates the letters of a released run once", async () => {
    api.fetchBillingRun.mockResolvedValue(RELEASED);
    api.createBillingLetters.mockResolvedValue({
      paymentDueOn: "2026-09-23",
      notes: "Hausschuhe mitbringen.",
      emergencyContacts: [{ name: "OGS Am Lindenhof", phone: "0170 1" }],
      createdAt: "2026-09-09T10:00:00Z",
    });
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Schreiben erstellen" }),
    );
    fireEvent.change(
      await screen.findByLabelText(
        "Hinweise für die Eltern (freiwillig)",
        {},
        {
          // The form is lazy-loaded after the click. Its first import can take
          // longer than Testing Library's one-second default in the full suite.
          timeout: 5_000,
        },
      ),
      { target: { value: "Hausschuhe mitbringen." } },
    );
    fireEvent.change(screen.getByLabelText("Betreuung"), {
      target: { value: "OGS Am Lindenhof" },
    });
    const create = () =>
      fireEvent.click(
        screen.getAllByRole("button", { name: "Schreiben erstellen" }).at(-1)!,
      );
    create();
    expect(
      await screen.findByText(
        "Bitte geben Sie zu jeder Notfallnummer Betreuung und Telefon an.",
      ),
    ).toBeInTheDocument();
    expect(api.createBillingLetters).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Telefon"), {
      target: { value: "0170 1" },
    });
    create();

    await waitFor(() =>
      expect(api.createBillingLetters).toHaveBeenCalledWith("9", {
        paymentDueOn: "2026-09-23",
        notes: "Hausschuhe mitbringen.",
        emergencyContacts: [{ name: "OGS Am Lindenhof", phone: "0170 1" }],
      }),
    );
    expect(await screen.findByText("23.09.2026")).toBeInTheDocument();
    expect(screen.getByText("OGS Am Lindenhof: 0170 1")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Schreiben erstellen" }),
    ).not.toBeInTheDocument();
  });

  it("keeps a payment date in the past from going out", async () => {
    api.fetchBillingRun.mockResolvedValue(RELEASED);
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Schreiben erstellen" }),
    );
    fireEvent.change(await screen.findByLabelText("Zahlen bis"), {
      target: { value: "01.09.2026" },
    });
    for (const name of ["Vorschau ansehen", "Schreiben erstellen"]) {
      fireEvent.click(screen.getAllByRole("button", { name }).at(-1)!);
    }

    expect(
      await screen.findAllByText(
        "Das Datum darf nicht in der Vergangenheit liegen.",
      ),
    ).not.toHaveLength(0);
    expect(api.previewBillingLetter).not.toHaveBeenCalled();
    expect(api.createBillingLetters).not.toHaveBeenCalled();
  });

  it("downloads the preview when the browser blocks the tab", async () => {
    const blob = new Blob(["%PDF"], { type: "application/pdf" });
    api.fetchBillingRun.mockResolvedValue(RELEASED);
    api.previewBillingLetter.mockResolvedValue({
      blob,
      filename: "bestaetigung-fe-0001.pdf",
    });
    const open = vi.spyOn(globalThis, "open").mockReturnValue(null);
    const createURL = vi
      .spyOn(URL, "createObjectURL")
      .mockReturnValue("blob:preview");
    const revokeURL = vi
      .spyOn(URL, "revokeObjectURL")
      .mockImplementation(() => undefined);
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Schreiben erstellen" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Vorschau ansehen" }),
    );

    await waitFor(() =>
      expect(downloadBlob).toHaveBeenCalledWith(
        blob,
        "bestaetigung-fe-0001.pdf",
      ),
    );
    expect(open).toHaveBeenCalledTimes(1);
    open.mockRestore();
    createURL.mockRestore();
    revokeURL.mockRestore();
  });

  it("names the missing bank account when the letters are refused", async () => {
    api.fetchBillingRun.mockResolvedValue(RELEASED);
    api.createBillingLetters.mockRejectedValue(
      new ApiError("missing", 409, {
        code: "parent_billing.bank_account_missing",
      }),
    );
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Schreiben erstellen" }),
    );
    fireEvent.click(
      (
        await screen.findAllByRole("button", { name: "Schreiben erstellen" })
      ).at(-1)!,
    );

    expect(
      await screen.findByText(
        catalogText("parent_billing.bank_account_missing", "die Schreiben"),
      ),
    ).toBeInTheDocument();
  });

  it("downloads the letter of a family once the letters exist", async () => {
    const blob = new Blob(["%PDF"], { type: "application/pdf" });
    api.fetchBillingRun.mockResolvedValue({
      ...RELEASED,
      letters: {
        paymentDueOn: "2026-09-23",
        notes: "",
        emergencyContacts: [],
        createdAt: "2026-09-09T10:00:00Z",
      },
    });
    api.downloadBillingLetter.mockResolvedValue({
      blob,
      filename: "bestaetigung-fe-0001.pdf",
    });
    renderPage();

    expect(
      await screen.findByText(/Die Schreiben sind erstellt/),
    ).toBeInTheDocument();
    // With the letters, the run offers to mail them (#4670).
    expect(screen.getByText("Versand per Mail")).toBeInTheDocument();
    await openInvoiceMenu();
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Schreiben herunterladen" }),
    );

    await waitFor(() =>
      expect(downloadBlob).toHaveBeenCalledWith(
        blob,
        "bestaetigung-fe-0001.pdf",
      ),
    );
    expect(api.downloadBillingLetter).toHaveBeenCalledWith("9", "51");
  });

  it("offers the overview for the Kämmerei only once the run is released", async () => {
    renderPage();
    await screen.findAllByText("Muster, Eva");
    expect(screen.getByText("Abrechnungslauf")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Übersicht für die Kämmerei exportieren",
      }),
    ).not.toBeInTheDocument();
  });

  it("downloads the overview of a released run", async () => {
    api.fetchBillingRun.mockResolvedValue(RELEASED);
    const blob = new Blob(["Rechnungsnummer"]);
    api.downloadBillingRunOverview.mockResolvedValue({
      blob,
      filename: "abrechnungsanordnung-Winterferien_2027.csv",
    });
    renderPage();

    // Wie im Schul-Portal: das Menü mit den drei Punkten, Abschnitt
    // „Exportieren“ (#5240).
    const trigger = await screen.findByRole("button", {
      name: "Übersicht für die Kämmerei exportieren",
    });
    // The table renders a desktop row and a stacked phone row.
    expect(screen.getAllByText("FE-0001")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Muster, Eva")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Lia, Ben")[0]).toBeInTheDocument();

    fireEvent.click(trigger);
    expect(await screen.findByText("Exportieren")).toBeInTheDocument();
    expect(
      screen.getAllByRole("menuitem").map((item) => item.textContent),
    ).toEqual(["PDF", "Excel", "CSV"]);
    fireEvent.click(screen.getByRole("menuitem", { name: "CSV" }));
    await waitFor(() =>
      expect(downloadBlob).toHaveBeenCalledWith(
        blob,
        "abrechnungsanordnung-Winterferien_2027.csv",
      ),
    );
    expect(api.downloadBillingRunOverview).toHaveBeenCalledWith("9", "csv");
  });

  it("names the overview when its download fails", async () => {
    api.fetchBillingRun.mockResolvedValue(RELEASED);
    api.downloadBillingRunOverview.mockRejectedValue(
      new ApiError("not released", 409, {
        code: "parent_billing.billing_run_not_released",
      }),
    );
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Übersicht für die Kämmerei exportieren",
      }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "PDF" }));
    expect(
      await screen.findByText(
        catalogText("parent_billing.billing_run_not_released", "die Übersicht"),
      ),
    ).toBeInTheDocument();
    expect(downloadBlob).not.toHaveBeenCalled();
  });

  describe("Storno und Gutschrift (#4690)", () => {
    const invoice = RELEASED.documents[0]!;
    const CANCELLED: BillingRunDetail = {
      ...RELEASED,
      previewCents: 0,
      documents: [
        {
          ...invoice,
          history: [
            {
              id: "52",
              runId: "9",
              kind: "cancellation",
              number: "FE-0002",
              issuedOn: "2026-09-10",
              totalCents: -11250,
              reason: "Anschrift falsch",
            },
          ],
        },
        {
          ...invoice,
          id: "52",
          kind: "cancellation",
          number: "FE-0002",
          referenceNumber: "FE-0001",
          reason: "Anschrift falsch",
          totalCents: -11250,
          children: invoice.children.map((child) => ({
            ...child,
            totalCents: -child.totalCents,
            positions: child.positions.map((position) => ({
              ...position,
              amountCents: -position.amountCents,
            })),
          })),
        },
      ],
    };

    it("cancels an issued invoice only with a reason", async () => {
      api.fetchBillingRun.mockResolvedValue(RELEASED);
      api.cancelBillingDocument.mockResolvedValue({
        ...CANCELLED.documents[1]!,
      });
      renderPage();

      await openInvoiceMenu();
      fireEvent.click(
        await screen.findByRole("menuitem", { name: "Rechnung stornieren" }),
      );
      const dialog = await screen.findByRole("dialog", {
        name: "Rechnung FE-0001 stornieren?",
      });
      expect(dialog).toHaveTextContent(/nimmt 112,50\s€ zurück/);
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Stornieren" }),
      );
      expect(
        await within(dialog).findByText("Bitte geben Sie eine Begründung an."),
      ).toBeInTheDocument();
      expect(api.cancelBillingDocument).not.toHaveBeenCalled();

      fireEvent.change(within(dialog).getByLabelText(/Begründung/), {
        target: { value: "  Anschrift falsch  " },
      });
      api.fetchBillingRun.mockResolvedValue(CANCELLED);
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Stornieren" }),
      );
      await waitFor(() =>
        expect(api.cancelBillingDocument).toHaveBeenCalledWith(
          "9",
          "51",
          "Anschrift falsch",
        ),
      );
      expect(
        await screen.findByText("Storno FE-0002 ist ausgestellt."),
      ).toBeInTheDocument();
      expect((await screen.findAllByText("Storniert"))[0]).toBeInTheDocument();
      expect(screen.getAllByText("Zu FE-0001")[0]).toBeInTheDocument();
    });

    it("credits at most what is left of a child", async () => {
      api.fetchBillingRun.mockResolvedValue(RELEASED);
      renderPage();

      await openInvoiceMenu();
      fireEvent.click(
        await screen.findByRole("menuitem", { name: "Gutschrift für Ben" }),
      );
      const dialog = await screen.findByRole("dialog", {
        name: "Gutschrift für Ben",
      });
      fireEvent.change(within(dialog).getByLabelText(/Betrag in Euro/), {
        target: { value: "50,00" },
      });
      fireEvent.change(within(dialog).getByLabelText(/Begründung/), {
        target: { value: "Krank" },
      });
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Gutschreiben" }),
      );
      expect(
        await within(dialog).findByText(/Höchstens 45,00/),
      ).toBeInTheDocument();
      expect(api.creditBillingDocument).not.toHaveBeenCalled();

      api.creditBillingDocument.mockResolvedValue({
        ...invoice,
        id: "53",
        kind: "credit",
        number: "FE-0003",
      });
      fireEvent.change(within(dialog).getByLabelText(/Betrag in Euro/), {
        target: { value: "5,00" },
      });
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Gutschreiben" }),
      );
      await waitFor(() =>
        expect(api.creditBillingDocument).toHaveBeenCalledWith("9", "51", {
          childId: "42",
          amountCents: 500,
          reason: "Krank",
        }),
      );
    });

    it("shows the history of a cancelled invoice and opens the run that issues it again", async () => {
      api.fetchBillingRun.mockResolvedValue(CANCELLED);
      api.fetchBillingRuns.mockResolvedValue([]);
      api.startBillingRun.mockResolvedValue({ ...RELEASED, id: "12" });
      renderPage();

      await openInvoiceMenu();
      expect(
        screen.queryByRole("menuitem", { name: "Rechnung stornieren" }),
      ).not.toBeInTheDocument();
      fireEvent.click(
        await screen.findByRole("menuitem", {
          name: "Aufschlüsselung ansehen",
        }),
      );
      const panel = await screen.findByRole("dialog", {
        name: "Rechnung an Eva Muster",
      });
      expect(panel).toHaveTextContent(/Verlauf/);
      expect(panel).toHaveTextContent(/Storno FE-0002/);
      expect(panel).toHaveTextContent(/Anschrift falsch/);
      fireEvent.click(
        within(panel).getByRole("button", { name: "Neu ausstellen" }),
      );
      await waitFor(() =>
        expect(api.startBillingRun).toHaveBeenCalledWith("5"),
      );
      expect(push).toHaveBeenCalledWith("/abrechnung/12");
    });
  });
});
