import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  },
}));
const { downloadBlob } = vi.hoisted(() => ({ downloadBlob: vi.fn() }));
vi.mock("~/lib/file-download", () => ({ downloadBlob }));

vi.mock("~/lib/traeger-billing-api", () => api);
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "9" }),
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/abrechnung/9",
}));

import TraegerBillingRunPage from "./page";

const PREVIEW: BillingRunDetail = {
  id: "9",
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
  numberPattern: "FE-{NNNN}",
  numberRangeSet: true,
  letters: null,
  documents: [
    {
      id: "",
      number: "",
      issuedOn: null,
      registrationId: "31",
      recipientFirstName: "Eva",
      recipientLastName: "Muster",
      recipientEmail: "eva@example.org",
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

describe("TraegerBillingRunPage", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    api.fetchBillingRun.mockResolvedValue(PREVIEW);
  });

  it("shows the preview of every family with its positions", async () => {
    renderPage();

    expect(await screen.findByText("Eva Muster")).toBeInTheDocument();
    expect(screen.getByText(/Entwurf ohne Nummer/)).toBeInTheDocument();
    expect(screen.getByText("Lia Muster")).toBeInTheDocument();
    expect(screen.getAllByText("Rechnungsbetrag")[0]).toBeInTheDocument();
    expect(
      screen.getByText(
        /1 bestätigte Anmeldung\(en\) haben keinen festgehaltenen Beitrag/,
      ),
    ).toBeInTheDocument();
  });

  it("corrects the amount of a child only with a reason", async () => {
    api.correctBillingRun.mockResolvedValue(CORRECTED);
    renderPage();
    await screen.findByText("Eva Muster");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Aktionen für die Rechnung an Eva Muster",
      }),
    );
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
    expect(
      await screen.findByText("Korrektur: Erster Tag krank"),
    ).toBeInTheDocument();
    expect(screen.getByText("Korrekturen für Lia")).toBeInTheDocument();
  });

  it("releases with the checked preview and reports a changed run", async () => {
    api.releaseBillingRun.mockRejectedValue(
      new ApiError("changed", 409, {
        code: "parent_billing.billing_run_changed",
      }),
    );
    renderPage();
    await screen.findByText("Eva Muster");

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

    expect(
      await screen.findByText(/Rechnung FE-0001 vom 09.09.2026/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/lassen sich nicht mehr ändern/),
    ).toBeInTheDocument();
    expect(screen.getByText("Eva Muster")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Lauf freigeben" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Aktionen für die Rechnung an Eva Muster",
      }),
    ).not.toBeInTheDocument();
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
      await screen.findByLabelText("Hinweise für die Eltern (freiwillig)"),
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
    fireEvent.click(
      screen.getByRole("button", {
        name: "Aktionen für die Rechnung an Eva Muster",
      }),
    );
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
});
