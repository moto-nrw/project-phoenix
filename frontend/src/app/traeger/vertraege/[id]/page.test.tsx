import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { CareContractDetail } from "~/lib/traeger-care-contracts-api";

const { api, mockPush } = vi.hoisted(() => ({
  api: {
    fetchCareContract: vi.fn(),
    downloadCareContract: vi.fn(),
    sendCareContracts: vi.fn(),
    resendCareContract: vi.fn(),
    discardCareContract: vi.fn(),
  },
  mockPush: vi.fn(),
}));

vi.mock("~/lib/traeger-care-contracts-api", () => api);
vi.mock("~/components/ui/pdf-preview", () => ({
  PdfPreview: ({ url, title }: { url: string | null; title: string }) => (
    <div data-testid="pdf-preview" data-loaded={url !== null}>
      {title}
    </div>
  ),
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  useParams: () => ({ id: "7" }),
  usePathname: () => "/vertraege/7",
}));

import TraegerVertragPage from "./page";

const DRAFT: CareContractDetail = {
  id: "7",
  status: "entwurf",
  schoolId: "3",
  schoolName: "OGS Am Lindenhof",
  childFirstName: "Lina",
  childLastName: "Muster",
  moduleNames: ["Frühbetreuung", "Offene Ganztagsschule"],
  partyName: "Erika Muster",
  partyEmail: "erika@example.org",
  effectiveFrom: "2027-08-01",
  createdAt: "2026-09-01T08:00:00Z",
  sentAt: null,
  version: {
    number: 1,
    effectiveFrom: "2027-08-01",
    endsOn: null,
    childBirthDate: "2019-03-04",
    schoolName: "OGS Am Lindenhof",
    phaseName: "2027/28",
    services: [
      {
        weekday: "mon",
        careOfferingId: "6",
        moduleName: "Frühbetreuung",
        endsAt: "08:00",
        lastDay: null,
      },
      {
        weekday: "mon",
        careOfferingId: "5",
        moduleName: "Offene Ganztagsschule",
        endsAt: "14:00",
        lastDay: null,
      },
      {
        weekday: "tue",
        careOfferingId: "5",
        moduleName: "Offene Ganztagsschule",
        endsAt: "16:30",
        lastDay: null,
      },
    ],
    priceCents: 9500,
    terms: {
      termKind: "school_year",
      termMonths: null,
      noticeMonths: 1,
      noticeTo: "month_end",
      specialTermination: true,
      specialTerminationNote: "Bei Umzug",
      statuteRequired: true,
      statuteTitle: "Satzung über die Elternbeiträge",
      statuteUrl: "https://example.org/satzung",
      contractPartner: "",
    },
    createdAt: "2026-09-01T08:00:00Z",
  },
  parties: [
    {
      firstName: "Erika",
      lastName: "Muster",
      email: "erika@example.org",
      hasAccount: true,
    },
  ],
  mailings: [],
  documentFrozen: false,
  signature: null,
  reminders: [],
};

function renderPage() {
  render(
    <ToastProvider>
      <TraegerVertragPage />
    </ToastProvider>,
  );
}

describe("TraegerVertragPage", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    mockPush.mockReset();
    api.fetchCareContract.mockResolvedValue(DRAFT);
    api.downloadCareContract.mockResolvedValue({
      blob: new Blob(["%PDF"], { type: "application/pdf" }),
      filename: "betreuungsvertrag-7.pdf",
    });
  });

  it("shows what the draft holds", async () => {
    renderPage();

    expect(
      await screen.findByText("Frühbetreuung bis 08:00 Uhr"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Offene Ganztagsschule bis 14:00 Uhr"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Offene Ganztagsschule bis 16:30 Uhr"),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByTestId("pdf-preview")).toHaveAttribute(
        "data-loaded",
        "true",
      ),
    );
    // Wednesday to Friday carry no care.
    expect(screen.getAllByText("Keine Betreuung")).toHaveLength(3);
    expect(
      screen.getByText("Bis zum Ende des Schuljahres"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("1 Monat Frist zum Monatsende"),
    ).toBeInTheDocument();
    expect(screen.getByText("95,00 €")).toBeInTheDocument();
    expect(
      screen.getByText(/Die Eltern sehen den Vertrag erst/),
    ).toBeInTheDocument();
  });

  it("names no summed fee for a contract over several modules", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      version: { ...DRAFT.version, priceCents: null },
    });
    renderPage();

    expect(
      await screen.findByText("Laut Beitragsordnung für die gebuchten Module"),
    ).toBeInTheDocument();
  });

  it("names the last day of a module that ends before the contract", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      version: {
        ...DRAFT.version,
        services: [
          {
            weekday: "mon",
            careOfferingId: "5",
            moduleName: "Kurzbetreuung",
            endsAt: "14:00",
            lastDay: "2027-07-31",
          },
        ],
      },
    });
    renderPage();

    expect(
      await screen.findByText(
        "Kurzbetreuung bis 14:00 Uhr, gebucht bis 31.07.2027",
      ),
    ).toBeInTheDocument();
  });

  it("counts two same-named offerings as two modules", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      moduleNames: ["Nachmittagsbetreuung"],
      version: {
        ...DRAFT.version,
        priceCents: null,
        services: [
          {
            weekday: "mon",
            careOfferingId: "7",
            moduleName: "Nachmittagsbetreuung",
            endsAt: "16:00",
            lastDay: null,
          },
          {
            weekday: "tue",
            careOfferingId: "8",
            moduleName: "Nachmittagsbetreuung",
            endsAt: "16:00",
            lastDay: null,
          },
        ],
      },
    });
    renderPage();

    expect(
      await screen.findByText("Laut Beitragsordnung für die gebuchten Module"),
    ).toBeInTheDocument();
  });

  it("sends the draft", async () => {
    api.sendCareContracts.mockResolvedValue({ sent: ["7"], skipped: [] });
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: /Vertrag senden/ }),
    );

    await waitFor(() =>
      expect(api.sendCareContracts).toHaveBeenCalledWith(["7"]),
    );
    expect(
      await screen.findByText(/Der Vertrag ist unterwegs/),
    ).toBeInTheDocument();
  });

  it.each([
    ["stopped", /Die Schule hat die Mails an diese Person gestoppt/],
    ["address", /bei einer Ihrer Schulen gesperrt/],
    ["provider", /moto-Support/],
    ["provider_unknown", /später noch einmal/],
    ["delayed", /nicht noch einmal/],
  ] as const)(
    "names the reason of a %s block and offers no second mail",
    async (sendBlock, text) => {
      api.fetchCareContract.mockResolvedValue({
        ...DRAFT,
        status: "versendet",
        documentFrozen: true,
        mailings: [
          {
            recipient: "erika@example.org",
            sentAt: "2026-09-02T08:00:00Z",
            state: sendBlock === "delayed" ? "sent" : "failed",
            outcome: sendBlock === "delayed" ? "delayed" : "provider_blocked",
            sendBlock,
          },
        ],
      });
      renderPage();

      expect(await screen.findByText(text)).toBeInTheDocument();
      expect(screen.queryByText(/Prüfen Sie die Adresse/)).toBeNull();
      expect(
        screen.getByRole("button", { name: "Erneut senden" }),
      ).toBeDisabled();
    },
  );

  it.each([
    ["unsubscribed", "Abgemeldet", /abgemeldet/],
    ["complained", "Nicht zustellbar", /unerwünscht/],
  ] as const)(
    "does not read a %s mail as sent",
    async (outcome, label, text) => {
      api.fetchCareContract.mockResolvedValue({
        ...DRAFT,
        status: "versendet",
        documentFrozen: true,
        mailings: [
          {
            recipient: "erika@example.org",
            sentAt: "2026-09-02T08:00:00Z",
            state: "failed",
            outcome,
          },
        ],
      });
      renderPage();

      expect(await screen.findByText(label)).toBeInTheDocument();
      expect(screen.getByText(text)).toBeInTheDocument();
      expect(screen.queryByText("Gesendet")).toBeNull();
    },
  );

  it("offers a new mail once the block is gone", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      status: "versendet",
      documentFrozen: true,
      mailings: [
        {
          recipient: "erika@example.org",
          sentAt: "2026-09-02T08:00:00Z",
          state: "failed",
          outcome: "provider_blocked",
        },
      ],
    });
    renderPage();

    expect(await screen.findByText(/besteht nicht mehr/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Erneut senden" })).toBeEnabled();
  });

  it("refreshes a queued delivery and enables retry when it fails", async () => {
    const pending: CareContractDetail = {
      ...DRAFT,
      status: "versendet",
      sentAt: "2026-09-02T08:00:00Z",
      documentFrozen: true,
      mailings: [
        {
          recipient: "erika@example.org",
          sentAt: "2026-09-02T08:00:00Z",
          state: "queued",
        },
      ],
    };
    api.fetchCareContract.mockResolvedValueOnce(pending).mockResolvedValue({
      ...pending,
      mailings: [{ ...pending.mailings[0]!, state: "failed" }],
    });
    renderPage();
    expect(
      await screen.findByRole("button", { name: "Erneut senden" }),
    ).toBeDisabled();
    expect(
      screen.getByText(/Die Mail wartet auf den Versand/),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Versand aktualisieren" }),
    );
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Erneut senden" }),
      ).toBeEnabled(),
    );
    expect(screen.getByText(/kam nicht an/)).toBeInTheDocument();
  });

  it("retries a failed delivery at the entered address without changing the contract", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      status: "versendet",
      documentFrozen: true,
      mailings: [
        {
          recipient: "alt@example.org",
          sentAt: "2026-09-02T08:00:00Z",
          state: "failed",
        },
      ],
    });
    api.resendCareContract.mockResolvedValue({ sent: ["7"], skipped: [] });
    renderPage();
    fireEvent.change(await screen.findByLabelText("Neue Versandadresse"), {
      target: { value: "neu@example.org" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "An diese Adresse senden" }),
    );
    await waitFor(() =>
      expect(api.resendCareContract).toHaveBeenCalledWith(
        "7",
        "neu@example.org",
      ),
    );
    expect(
      await screen.findByText("Der Vertrag ist an die neue Adresse unterwegs."),
    ).toBeInTheDocument();
    expect(api.sendCareContracts).not.toHaveBeenCalled();
  });

  it("shows an unknown delivery result without claiming success", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      status: "versendet",
      sentAt: "2026-09-02T08:00:00Z",
      documentFrozen: true,
      mailings: [
        {
          recipient: "erika@example.org",
          sentAt: "2026-09-02T08:00:00Z",
          state: "unknown",
        },
      ],
    });
    renderPage();
    expect(
      await screen.findByText(/Das Ergebnis des Mail-Versands ist unbekannt/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Die Eltern haben den Vertrag/),
    ).not.toBeInTheDocument();
  });

  it("offers to send a sent contract again and no discard", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      status: "versendet",
      sentAt: "2026-09-02T08:00:00Z",
      documentFrozen: true,
      mailings: [
        {
          recipient: "erika@example.org",
          sentAt: "2026-09-02T08:00:00Z",
          state: "sent",
        },
      ],
    });
    renderPage();

    expect(
      await screen.findByRole("button", { name: /Erneut senden/ }),
    ).toBeEnabled();
    expect(screen.getByText("Gesendet")).toBeInTheDocument();
    expect(
      screen.getByText(/Die Mail ging an erika@example.org/),
    ).toBeInTheDocument();
    expect(screen.getByText("PDF für die Eltern")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Vertrag senden/ }),
    ).not.toBeInTheDocument();
  });
});
