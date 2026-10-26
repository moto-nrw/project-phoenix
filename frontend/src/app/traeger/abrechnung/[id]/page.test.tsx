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
    downloadBillingRunOverview: vi.fn(),
  },
}));
const { downloadBlob } = vi.hoisted(() => ({ downloadBlob: vi.fn() }));

vi.mock("~/lib/traeger-billing-api", () => api);
vi.mock("~/lib/file-download", () => ({ downloadBlob }));
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
  documents: [
    {
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

const RELEASED: BillingRunDetail = {
  ...PREVIEW,
  status: "released",
  issuedOn: "2026-09-09",
  documentCount: 1,
  totalCents: 11250,
  unbilled: 0,
  documents: [
    { ...PREVIEW.documents[0]!, number: "FE-0001", issuedOn: "2026-09-09" },
  ],
};

describe("TraegerBillingRunPage", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    downloadBlob.mockReset();
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
    api.fetchBillingRun.mockResolvedValue(RELEASED);
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

  it("offers the overview for the Kämmerei only once the run is released", async () => {
    renderPage();
    await screen.findByText("Eva Muster");
    expect(screen.getByText("Abrechnungslauf")).toBeInTheDocument();
    expect(
      screen.queryByText("Übersicht für die Kämmerei"),
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

    expect(
      await screen.findByText("Übersicht für die Kämmerei"),
    ).toBeInTheDocument();
    // The table renders a desktop row and a stacked phone row.
    expect(screen.getAllByText("FE-0001")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Muster, Eva")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Lia, Ben")[0]).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Als CSV herunterladen" }),
    );
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
    await screen.findByText("Übersicht für die Kämmerei");

    fireEvent.click(
      screen.getByRole("button", { name: "Als PDF herunterladen" }),
    );
    expect(
      await screen.findByText(
        catalogText("parent_billing.billing_run_not_released", "die Übersicht"),
      ),
    ).toBeInTheDocument();
    expect(downloadBlob).not.toHaveBeenCalled();
  });
});
