import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
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
