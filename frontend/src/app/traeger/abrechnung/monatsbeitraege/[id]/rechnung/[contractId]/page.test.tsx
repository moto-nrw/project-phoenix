import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareAmendment,
  CareInvoice,
  CareRunDetail,
} from "~/lib/traeger-care-billing-api";

// The page of one invoice in the review of a monthly run (#6197): it shows
// the invoice as the release issues it with every change, the history with
// revoked and conflicting changes, opens a change with its reason and
// changes nothing once the invoice is issued.

const { api, amend } = vi.hoisted(() => ({
  api: { fetchCareRun: vi.fn() },
  amend: { amendCareInvoice: vi.fn() },
}));
vi.mock("~/lib/traeger-care-billing-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-billing-api")>()),
  ...api,
}));
vi.mock("~/lib/traeger-care-amendments-api", () => amend);
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7", contractId: "22" }),
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/abrechnung/monatsbeitraege/7/rechnung/22",
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import TraegerCareInvoiceAmendPage from "./page";

const NONE = {
  totalCents: null,
  label: "",
  amountCents: null,
  recipient: null,
  address: null,
};

function amendment(change: Partial<CareAmendment>): CareAmendment {
  return {
    id: "1",
    kind: "amount",
    label: "Korrektur des Betrags",
    amountCents: -750,
    positionKey: "",
    before: { ...NONE, totalCents: 7750 },
    after: { ...NONE, totalCents: 7000 },
    reason: "Kulanz wegen Schließtag",
    amendedAt: "2026-11-08T15:20:00Z",
    carriedFromNumber: null,
    revocation: null,
    conflict: null,
    ...change,
  };
}

const INVOICE: CareInvoice = {
  id: "",
  number: "",
  issuedOn: null,
  contractId: "22",
  service: "care",
  schoolId: "3",
  schoolName: "Grundschule Ispringen",
  childFirstName: "Mia",
  childLastName: "Beispiel",
  recipient: {
    firstName: "Jan",
    lastName: "Beispiel",
    email: "jan@example.org",
    address: null,
  },
  positions: [
    {
      careOfferingId: "101",
      moduleName: "Betreuung bis 14:00 Uhr",
      versionNumber: 1,
      serviceFrom: "2026-10-01",
      serviceUntil: "2026-10-31",
      weekdays: [{ weekday: "mon", monthlyCents: 3200, amountCents: 3200 }],
      amountCents: 3200,
      key: "care:101:1:2026-10-01",
    },
    {
      careOfferingId: "102",
      moduleName: "Betreuung bis 16:30 Uhr",
      versionNumber: 1,
      serviceFrom: "2026-10-01",
      serviceUntil: "2026-10-31",
      weekdays: [{ weekday: "tue", monthlyCents: 4550, amountCents: 4550 }],
      amountCents: 4550,
      key: "care:102:1:2026-10-01",
    },
  ],
  reductions: [],
  mealPositions: [],
  costShares: [],
  amendments: [
    amendment({}),
    amendment({
      id: "2",
      kind: "remove_position",
      label: "Entfällt: Betreuung bis 16:30 Uhr",
      amountCents: -4550,
      positionKey: "care:102:1:2026-10-01",
      before: { ...NONE, label: "Betreuung bis 16:30 Uhr", amountCents: 4550 },
      after: NONE,
      reason: "Dienstag nicht gebucht",
    }),
    amendment({
      id: "3",
      kind: "recipient",
      label: "",
      amountCents: 0,
      before: {
        ...NONE,
        recipient: {
          firstName: "Eva",
          lastName: "Beispiel",
          secondName: "",
          email: "eva@example.org",
        },
      },
      after: {
        ...NONE,
        recipient: {
          firstName: "Jan",
          lastName: "Beispiel",
          secondName: "",
          email: "jan@example.org",
        },
      },
      reason: "Rechnung an den Vater",
      carriedFromNumber: "MB-2026-0004",
    }),
    amendment({
      id: "4",
      amountCents: 500,
      reason: "Alter Betrag",
      conflict: "basis_changed",
    }),
  ],
  totalCents: 2450,
  exclusion: null,
};

function run(change: Partial<CareRunDetail> = {}): CareRunDetail {
  return {
    id: "7",
    billingMonth: "2026-10-01",
    status: "calculated",
    createdAt: "2026-10-28T08:00:00Z",
    releasedAt: null,
    issuedOn: null,
    documentCount: null,
    totalCents: null,
    monthRule: "version_on_first",
    invoices: [INVOICE],
    unpriced: [],
    duplicates: [],
    duplicateChildren: 0,
    billed: 0,
    previewCount: 1,
    previewCents: 2450,
    numberPattern: "MB-{JJJJ}-{NNNN}",
    numberRangeSet: true,
    invoiceDate: null,
    schoolIds: ["3"],
    schools: [{ id: "3", name: "Grundschule Ispringen" }],
    excludedCount: 0,
    excludedCents: 0,
    fingerprint: "f1",
    mealGaps: [],
    mealUnbilled: [],
    mealChanged: [],
    costBearers: [],
    amendable: {
      amount: true,
      recipient: true,
      address: true,
      positions: true,
    },
    amendmentConflicts: 1,
    ...change,
  };
}

function renderPage() {
  render(
    <ToastProvider>
      <TraegerCareInvoiceAmendPage />
    </ToastProvider>,
  );
}

describe("TraegerCareInvoiceAmendPage", () => {
  beforeEach(() => {
    api.fetchCareRun.mockReset();
    amend.amendCareInvoice.mockReset();
  });

  it("shows the invoice as it is issued, with its changes and their history", async () => {
    api.fetchCareRun.mockResolvedValue(run());
    renderPage();
    expect(
      await screen.findByRole("heading", {
        name: "Rechnung ändern: Mia Beispiel",
      }),
    ).toBeInTheDocument();
    const invoice = screen
      .getByRole("heading", { name: "So wird die Rechnung ausgestellt" })
      .closest("section");
    expect(invoice).not.toBeNull();
    const card = within(invoice as HTMLElement);
    expect(
      card.getByText("Entfällt", { selector: "span" }),
    ).toBeInTheDocument();
    expect(
      card.getByText("Grund: Kulanz wegen Schließtag"),
    ).toBeInTheDocument();
    expect(card.getByText("Empfänger geändert")).toBeInTheDocument();
    expect(card.getByText("24,50 €")).toBeInTheDocument();
    expect(card.queryByText("Grund: Alter Betrag")).not.toBeInTheDocument();
    expect(card.getByText("Grund: Dienstag nicht gebucht")).toBeInTheDocument();

    expect(screen.getByText("Übernommen aus MB-2026-0004")).toBeInTheDocument();
    expect(screen.getByText("Passt nicht mehr")).toBeInTheDocument();
    expect(
      screen.getByText(/Eine Änderung passt nicht mehr zur Rechnung/),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: /zurücknehmen$/ }),
    ).toHaveLength(4);
  });

  it("sends a change with its reason and the reviewed state", async () => {
    api.fetchCareRun.mockResolvedValue(run());
    amend.amendCareInvoice.mockResolvedValue(run());
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Rechnung ändern" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: /Position hinzufügen/ }),
    );
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText(/Bezeichnung/), {
      target: { value: "Ausflug Wildpark" },
    });
    fireEvent.change(within(dialog).getByLabelText(/Betrag in Euro/), {
      target: { value: "12,00" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Hinzufügen" }));
    expect(
      await within(dialog).findByText("Bitte nennen Sie einen Grund."),
    ).toBeInTheDocument();
    expect(amend.amendCareInvoice).not.toHaveBeenCalled();

    fireEvent.change(within(dialog).getByLabelText(/Grund/), {
      target: { value: "Elternbrief vom 3.1." },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Hinzufügen" }));
    await vi.waitFor(() => expect(amend.amendCareInvoice).toHaveBeenCalled());
    expect(amend.amendCareInvoice).toHaveBeenCalledWith("7", {
      kind: "add_position",
      label: "Ausflug Wildpark",
      amountCents: 1200,
      contractId: "22",
      service: "care",
      fingerprint: "f1",
      reason: "Elternbrief vom 3.1.",
    });
  });

  it("confirms the effective care deduction after reductions", async () => {
    api.fetchCareRun.mockResolvedValue(
      run({
        invoices: [
          {
            ...INVOICE,
            positions: [{ ...INVOICE.positions[0]!, payableCents: 1600 }],
            amendments: [],
          },
        ],
      }),
    );
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Betreuung bis 14:00 Uhr",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: /Position entfernen/ }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("-16,00 €")).toBeInTheDocument();
  });

  it("offers meal removal and marks an applied removal", async () => {
    api.fetchCareRun.mockResolvedValue(
      run({
        invoices: [
          {
            ...INVOICE,
            mealPositions: [
              {
                mealMonth: "2026-09-01",
                mode: "flat",
                quantity: 1,
                unitCents: 8000,
                amountCents: 8000,
                key: "meal:1:2026-09-01",
              },
            ],
          },
        ],
      }),
    );
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Mensa-Pauschale September 2026",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: /Position entfernen/ }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("-80,00 €")).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText(/Grund/), {
      target: { value: "Nicht gebucht" },
    });
    amend.amendCareInvoice.mockResolvedValue(
      run({
        invoices: [
          {
            ...INVOICE,
            mealPositions: [
              {
                mealMonth: "2026-09-01",
                mode: "flat",
                quantity: 1,
                unitCents: 8000,
                amountCents: 8000,
                key: "meal:1:2026-09-01",
              },
            ],
            amendments: [
              amendment({
                kind: "remove_position",
                positionKey: "meal:1:2026-09-01",
                amountCents: -8000,
              }),
            ],
          },
        ],
      }),
    );
    fireEvent.click(within(dialog).getByRole("button", { name: "Entfernen" }));
    await vi.waitFor(() =>
      expect(amend.amendCareInvoice).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ positionKey: "meal:1:2026-09-01" }),
      ),
    );
    const meal = await screen.findByText("Mensa-Pauschale September 2026");
    expect(meal.closest("div.line-through")).not.toBeNull();
    expect(meal.closest("div.line-through")?.parentElement).toHaveTextContent(
      "Entfällt",
    );
  });

  it.each([
    ["Betrag ändern", "total_cents", /Neuer Betrag in Euro/],
    ["Empfänger ändern", "recipient.first_name", /Vorname/],
    ["Anschrift ändern", "address.street", /Straße/],
  ])(
    "maps server errors for %s to the visible field",
    async (action, field, label) => {
      api.fetchCareRun.mockResolvedValue(run());
      amend.amendCareInvoice.mockRejectedValue(
        new ApiError("invalid", 400, {
          code: "parent_billing.care_amendment_invalid",
          errors: [{ field, reason: "invalid" }],
        }),
      );
      renderPage();
      fireEvent.click(
        await screen.findByRole("button", { name: "Rechnung ändern" }),
      );
      fireEvent.click(
        await screen.findByRole("menuitem", { name: new RegExp(action) }),
      );
      const dialog = await screen.findByRole("dialog");
      if (action === "Anschrift ändern") {
        fireEvent.change(within(dialog).getByLabelText(/Straße/), {
          target: { value: "Weg 2" },
        });
        fireEvent.change(within(dialog).getByLabelText(/PLZ/), {
          target: { value: "75175" },
        });
        fireEvent.change(within(dialog).getByLabelText(/Ort/), {
          target: { value: "Pforzheim" },
        });
      }
      fireEvent.change(within(dialog).getByLabelText(/Grund/), {
        target: { value: "Korrektur" },
      });
      fireEvent.click(within(dialog).getByRole("button", { name: action }));
      const control = within(dialog).getByLabelText(label);
      await vi.waitFor(() =>
        expect(control).toHaveAttribute("aria-invalid", "true"),
      );
      expect(control).toHaveFocus();
    },
  );

  it("uses Berlin dates for changes and revocations outside Germany", async () => {
    const localDate = Date.prototype.toLocaleDateString;
    const formatter = vi
      .spyOn(Date.prototype, "toLocaleDateString")
      .mockImplementation(function (this: Date, locale, options) {
        return localDate.call(this, locale, {
          ...options,
          timeZone: options?.timeZone ?? "America/New_York",
        });
      });
    try {
      expect(new Date("2026-11-08T00:30:00Z").toLocaleDateString("de-DE")).toBe(
        "7.11.2026",
      );
      api.fetchCareRun.mockResolvedValue(
        run({
          invoices: [
            {
              ...INVOICE,
              amendments: [
                amendment({
                  amendedAt: "2026-11-08T00:30:00Z",
                  revocation: {
                    reason: "Irrtum",
                    revokedAt: "2026-11-09T00:30:00Z",
                  },
                }),
              ],
            },
          ],
        }),
      );
      renderPage();
      expect(
        await screen.findByText(
          /Grund: Kulanz wegen Schließtag · 08.11.2026, 01:30/,
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Zurückgenommen: Irrtum · 09.11.2026, 01:30/),
      ).toBeInTheDocument();
    } finally {
      formatter.mockRestore();
    }
  });

  it("offers only the changes the Träger allows", async () => {
    api.fetchCareRun.mockResolvedValue(
      run({
        amendable: {
          amount: false,
          recipient: false,
          address: false,
          positions: false,
        },
      }),
    );
    renderPage();
    expect(
      await screen.findByText(
        /hat das Ändern einzelner Rechnungen ausgeschaltet/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Rechnung ändern: Mia Beispiel" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Rechnung ändern" }),
    ).not.toBeInTheDocument();
  });

  it("changes nothing once the invoice is issued", async () => {
    api.fetchCareRun.mockResolvedValue(
      run({
        status: "released",
        invoices: [
          {
            ...INVOICE,
            id: "40",
            number: "MB-2026-0007",
            amendments: [amendment({})],
          },
        ],
      }),
    );
    renderPage();
    expect(
      await screen.findByRole("heading", { name: "Rechnung: Mia Beispiel" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Diese Rechnung ist ausgestellt und ändert sich nicht mehr/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Rechnung MB-2026-0007" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Rechnung ändern" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /zurücknehmen$/ }),
    ).not.toBeInTheDocument();
  });
});
