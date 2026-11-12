import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { setTestClock } from "~/test/clock";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { catalogText } from "~/test/error-catalog-text";
import { PaperSignatureModal } from "~/components/traeger/care-contract-paper-modal";
import type { CareContractDetail } from "~/lib/traeger-care-contracts-api";

const { api, mockPush } = vi.hoisted(() => ({
  api: {
    fetchCareContract: vi.fn(),
    downloadCareContract: vi.fn(),
    sendCareContracts: vi.fn(),
    resendCareContract: vi.fn(),
    discardCareContract: vi.fn(),
    recordPaperSignature: vi.fn(),
    downloadCareContractVersion: vi.fn(),
  },
  mockPush: vi.fn(),
}));

vi.mock("~/lib/traeger-care-contracts-api", () => api);
// The SEPA mandate card of the family (#4695) answers 403 without the right
// „Abrechnung“; these tests cover the contract and keep it empty.
vi.mock("~/lib/traeger-sepa-api", () => ({
  fetchSepaMandates: vi.fn().mockResolvedValue([]),
  fetchSepaMandateHistory: vi.fn().mockResolvedValue([]),
  createSepaMandate: vi.fn(),
  revokeSepaMandate: vi.fn(),
}));
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
  modulesText: "Frühbetreuung und Offene Ganztagsschule",
  partyName: "Erika Muster",
  partyEmail: "erika@example.org",
  effectiveFrom: "2027-08-01",
  createdAt: "2026-09-01T08:00:00Z",
  sentAt: null,
  concluded: null,
  endsOn: null,
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
  termText: "bis zum Ende des Schuljahres",
  noticeText: "mit 1 Monat Frist zum Monatsende",
  feeText: "95,00 €",
  signature: null,
  reminders: [],
  versions: [],
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

  it("does not ask for signatures on an acceptance-concluded contract", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      status: "aktiv",
      documentFrozen: true,
      concluded: { method: "mit_anmeldung", at: "2026-09-01T08:00:00Z" },
    });
    renderPage();
    expect(
      await screen.findByText(
        "Der Vertrag ist bereits mit der Annahme der Anmeldung abgeschlossen.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Eine weitere Unterschrift ist nicht nötig."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Papiervertrag erfassen" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Dann erfassen Sie den/)).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        "Nach dem Senden können die Eltern online unterschreiben.",
      ),
    ).not.toBeInTheDocument();
  });

  it.each([true, false])(
    "shows checksum integrity with the shared notice (intact=%s)",
    async (intact) => {
      api.fetchCareContract.mockResolvedValue({
        ...DRAFT,
        status: "aktiv",
        documentFrozen: true,
        signature: {
          method: "drawn",
          channel: "mail_link",
          signerName: "Erika Muster",
          signedAt: "2026-09-01T08:00:00Z",
          signedOn: "2026-09-01",
          documentSha256: "a".repeat(64),
          earlyStartRequested: false,
          withdrawalEndsOn: "2026-09-15",
          imagePng: "",
          paperFilename: "",
          intact,
        },
      });
      renderPage();
      const checksum = await screen.findByText(
        intact ? "Prüfsumme stimmt" : "Prüfsumme stimmt nicht",
      );
      expect(
        checksum.closest(intact ? '[role="status"]' : '[role="alert"]'),
      ).not.toBeNull();
      expect(screen.getByText("a".repeat(64))).toBeInTheDocument();
    },
  );

  it("does not allow paper signing dates before 2000", async () => {
    setTestClock("2000-01-02T12:00:00+01:00");
    render(
      <ToastProvider>
        <PaperSignatureModal
          contract={DRAFT}
          onClose={vi.fn()}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByLabelText("Unterschrieben am"));
    const previousMonth = await screen.findByRole("button", {
      name: "Vorheriger Monat",
    });
    fireEvent.click(previousMonth);
    expect(
      await screen.findByRole("button", { name: /31. Dezember 1999/ }),
    ).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Nächster Monat" }));
    expect(
      await screen.findByRole("button", { name: "Samstag, 1. Januar 2000" }),
    ).toBeEnabled();
  });

  it("marks and focuses a refused paper signing date", async () => {
    api.recordPaperSignature.mockRejectedValue(
      new ApiError("invalid date", 400, {
        code: "care_contract.field_invalid",
        errors: [{ field: "signed_on", reason: "invalid" }],
      }),
    );
    render(
      <ToastProvider>
        <PaperSignatureModal
          contract={DRAFT}
          onClose={vi.fn()}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    fireEvent.change(screen.getByLabelText("PDF wählen"), {
      target: {
        files: [new File(["%PDF"], "vertrag.pdf", { type: "application/pdf" })],
      },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Vertrag abschließen" }),
    );
    await waitFor(() => expect(api.recordPaperSignature).toHaveBeenCalled());
    const field = screen.getByLabelText("Unterschrieben am");
    await waitFor(() =>
      expect(field).toHaveAttribute("data-form-invalid", "true"),
    );
    const errorId = field.getAttribute("aria-describedby");
    expect(errorId).toBeTruthy();
    expect(document.getElementById(errorId ?? "")).toHaveTextContent(/.+/);
    await waitFor(() => expect(field).toHaveFocus());
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
      screen.getByText("bis zum Ende des Schuljahres"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("mit 1 Monat Frist zum Monatsende"),
    ).toBeInTheDocument();
    expect(screen.getByText("95,00 €")).toBeInTheDocument();
    // A special termination always ends at the half year (#6349); the
    // note of the terms only names the cases.
    expect(
      screen.getByText("Zum Halbjahr: 31. Januar oder 31. Juli"),
    ).toBeInTheDocument();
    expect(screen.getByText("Bei Umzug")).toBeInTheDocument();
    expect(
      screen.getByText(/Die Eltern sehen den Vertrag erst/),
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
      const resend = screen.getByRole("button", { name: "Erneut senden" });
      expect(resend).toHaveAttribute("aria-disabled", "true");
      expect(resend).toHaveAccessibleDescription(expect.stringMatching(/\S/));
      fireEvent.click(resend);
      expect(api.sendCareContracts).not.toHaveBeenCalled();
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
    ).toHaveAttribute("aria-disabled", "true");
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

  it.each(["versendet", "aktiv"] as const)(
    "retries a failed %s contract at the entered address without changing it",
    async (status) => {
      api.fetchCareContract.mockResolvedValue({
        ...DRAFT,
        status,
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
        await screen.findByText(
          "Der Vertrag ist an die neue Adresse unterwegs.",
        ),
      ).toBeInTheDocument();
      expect(api.sendCareContracts).not.toHaveBeenCalled();
    },
  );

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

  it("keeps a concluded contract unchanged and allows resending", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      status: "aktiv",
      sentAt: "2026-09-03T08:00:00Z",
      concluded: { method: "mit_anmeldung", at: "2026-09-03T08:00:00Z" },
      mailings: [
        {
          recipient: "erika@example.org",
          sentAt: "2026-09-03T08:00:00Z",
          state: "sent",
        },
      ],
      documentFrozen: true,
    });
    renderPage();

    expect(
      await screen.findByText(
        /Der Vertrag gilt seit der Annahme der Anmeldung am 03\.09\.2026/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Am 03.09.2026 um 10:00 Uhr, bei der Annahme der Anmeldung",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("So steht es im abgeschlossenen Vertrag."),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Aktiv")[0]).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /PDF herunterladen/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Erneut senden" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Entwurf verwerfen/ }),
    ).not.toBeInTheDocument();
  });

  it("shows the versions of an active contract and from when a change applies", async () => {
    const first = DRAFT.version.services;
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      status: "aktiv",
      sentAt: "2026-07-20T08:00:00Z",
      concluded: { method: "mit_anmeldung", at: "2026-07-20T08:00:00Z" },
      documentFrozen: true,
      version: { ...DRAFT.version, effectiveFrom: "2026-08-01" },
      versions: [
        {
          number: 1,
          status: "abgeloest",
          source: "abschluss",
          effectiveFrom: "2026-08-01",
          endsOn: "2027-02-28",
          validUntil: "2027-02-28",
          services: first,
          createdAt: "2026-07-20T08:00:00Z",
        },
        {
          number: 2,
          status: "wirksam",
          source: "buchungsaenderung",
          effectiveFrom: "2027-03-01",
          endsOn: null,
          validUntil: null,
          services: [
            ...first.slice(0, 2),
            {
              weekday: "wed",
              careOfferingId: "5",
              moduleName: "Offene Ganztagsschule",
              endsAt: "16:30",
              lastDay: null,
            },
          ],
          createdAt: "2026-09-01T08:00:00Z",
        },
      ],
    } satisfies CareContractDetail);
    api.downloadCareContractVersion.mockResolvedValue({
      blob: new Blob(["%PDF"], { type: "application/pdf" }),
      filename: "betreuungsvertrag-7-version-2.pdf",
    });
    renderPage();

    expect(
      await screen.findByText(
        "Ab 01.03.2027 gilt Version 2. Eine Änderung der Betreuung ist freigegeben. Was sich ändert, steht unten unter „Versionen“.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Gilt jetzt")).toBeInTheDocument();
    expect(screen.getByText("Gilt ab 01.03.2027")).toBeInTheDocument();
    expect(
      screen.getByText("01.08.2026 bis 28.02.2027 · Vertragsabschluss"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "ab 01.03.2027 · Änderung der Betreuung · 2 Tage geändert",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Geändert")).toHaveLength(2);

    fireEvent.click(
      screen.getByRole("button", { name: "PDF von Version 2 herunterladen" }),
    );
    await waitFor(() =>
      expect(api.downloadCareContractVersion).toHaveBeenCalledWith("7", 2),
    );
  });

  it("announces no change for a contract that only starts later", async () => {
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      status: "aktiv",
      sentAt: "2026-07-20T08:00:00Z",
      concluded: { method: "mit_anmeldung", at: "2026-07-20T08:00:00Z" },
      documentFrozen: true,
      versions: [
        {
          number: 1,
          status: "wirksam",
          source: "abschluss",
          effectiveFrom: "2027-08-01",
          endsOn: null,
          validUntil: null,
          services: DRAFT.version.services,
          createdAt: "2026-07-20T08:00:00Z",
        },
      ],
    } satisfies CareContractDetail);
    renderPage();

    expect(await screen.findByText("Gilt ab 01.08.2027")).toBeInTheDocument();
    expect(screen.queryByText(/gilt Version 1/)).not.toBeInTheDocument();
  });

  it("lists a change that goes in front of a planned version by its date", async () => {
    const monday = DRAFT.version.services.slice(0, 2);
    const late = { ...monday[1]!, endsAt: "16:30" };
    const version = (
      number: number,
      status: "wirksam" | "abgeloest",
      effectiveFrom: string,
      validUntil: string | null,
      services: CareContractDetail["version"]["services"],
    ) => ({
      number,
      status,
      source:
        number === 1 ? ("abschluss" as const) : ("buchungsaenderung" as const),
      effectiveFrom,
      endsOn: null,
      validUntil,
      services,
      createdAt: "2026-09-01T08:00:00Z",
    });
    api.fetchCareContract.mockResolvedValue({
      ...DRAFT,
      status: "aktiv",
      sentAt: "2026-07-20T08:00:00Z",
      concluded: { method: "mit_anmeldung", at: "2026-07-20T08:00:00Z" },
      documentFrozen: true,
      versions: [
        version(1, "abgeloest", "2026-08-01", "2027-01-31", monday),
        version(2, "wirksam", "2027-03-01", null, [
          ...monday,
          { ...monday[1]!, weekday: "wed" },
        ]),
        {
          ...version(3, "abgeloest", "2027-02-01", "2027-02-28", [
            monday[0]!,
            late,
          ]),
          endsOn: "2027-07-31",
        },
      ],
    } satisfies CareContractDetail);
    renderPage();

    const headings = await screen.findAllByRole("heading", { level: 3 });
    expect(headings.map((heading) => heading.textContent)).toEqual([
      "Version 2",
      "Version 3",
      "Version 1",
    ]);
    expect(
      screen.getByText(
        "Ab 01.02.2027 gilt Version 3. Eine Änderung der Betreuung ist freigegeben. Was sich ändert, steht unten unter „Versionen“.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "01.02.2027 bis 28.02.2027 · Änderung der Betreuung · 1 Tag geändert",
      ),
    ).toBeInTheDocument();
  });
  it.each([403, 503])(
    "handles a %s contract reload after sending",
    async (status) => {
      api.fetchCareContract
        .mockResolvedValueOnce(DRAFT)
        .mockRejectedValueOnce(new ApiError("failed", status));
      api.sendCareContracts.mockResolvedValue({ sent: 1, skipped: [] });
      renderPage();
      fireEvent.click(
        await screen.findByRole("button", { name: "Vertrag senden" }),
      );
      const code =
        status === 403 ? "general.permission" : "general.unavailable";
      // Anker: der Nachladefehler bestätigt den bereits erfolgten Versand.
      expect(
        await screen.findByText(
          `${catalogText(code, "die Anzeige des Vertrags")} ${SAVED_BUT_STALE_SUFFIX}`,
        ),
      ).toBeVisible();
      if (status === 403) {
        expect(screen.queryByText("Vertragsangaben")).not.toBeInTheDocument();
        expect(screen.queryAllByText("erika@example.org")).toHaveLength(0);
        expect(screen.queryByTestId("pdf-preview")).not.toBeInTheDocument();
        expect(
          screen.queryByRole("button", { name: "PDF herunterladen" }),
        ).not.toBeInTheDocument();
      } else {
        expect(screen.getByText("Vertragsangaben")).toBeVisible();
        expect(screen.getByTestId("pdf-preview")).toBeVisible();
      }
      expect(api.sendCareContracts).toHaveBeenCalledTimes(1);
    },
  );

  // #6652: after a confirmed send a failed reload is a load error above the
  // contract, not a failed send or a page error that replaces the contract.
  describe("reload after a write (#6652)", () => {
    const reloadText = `${catalogText(
      "general.unavailable",
      "die Anzeige des Vertrags",
    )} ${SAVED_BUT_STALE_SUFFIX}`;
    const down = () =>
      new ApiError("down", 503, { code: "general.unavailable" });
    const FAILED_MAIL: CareContractDetail = {
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
    };

    it("keeps a sent contract sent when the reload fails", async () => {
      api.sendCareContracts.mockResolvedValue({ sent: ["7"], skipped: [] });
      renderPage();
      const sendButton = await screen.findByRole("button", {
        name: /Vertrag senden/,
      });
      api.fetchCareContract.mockClear();
      api.fetchCareContract
        .mockRejectedValueOnce(down())
        .mockResolvedValue({ ...DRAFT, status: "versendet" });

      fireEvent.click(sendButton);

      expect(await screen.findByText(reloadText)).toBeInTheDocument();
      expect(api.sendCareContracts).toHaveBeenCalledTimes(1);
      // Anker: the contract stays shown.
      expect(screen.getByText("Vertragsangaben")).toBeInTheDocument();
      expect(
        screen.queryByText(catalogText("general.unavailable", "der Vertrag")),
      ).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
      expect(
        await screen.findByRole("button", { name: "Erneut senden" }),
      ).toBeInTheDocument();
      expect(screen.queryByText(reloadText)).not.toBeInTheDocument();
      expect(api.fetchCareContract).toHaveBeenCalledTimes(2);
      expect(api.sendCareContracts).toHaveBeenCalledTimes(1);
    });

    it("keeps a contract resent to a new address when the reload fails", async () => {
      api.fetchCareContract.mockResolvedValue(FAILED_MAIL);
      api.resendCareContract.mockResolvedValue({ sent: ["7"], skipped: [] });
      renderPage();
      fireEvent.change(await screen.findByLabelText("Neue Versandadresse"), {
        target: { value: "neu@example.org" },
      });
      api.fetchCareContract.mockClear();
      api.fetchCareContract.mockRejectedValueOnce(down()).mockResolvedValue({
        ...FAILED_MAIL,
        mailings: [
          {
            recipient: "neu@example.org",
            sentAt: "2026-09-09T10:00:00Z",
            state: "queued",
          },
          ...FAILED_MAIL.mailings,
        ],
      });

      fireEvent.click(
        screen.getByRole("button", { name: "An diese Adresse senden" }),
      );

      expect(await screen.findByText(reloadText)).toBeInTheDocument();
      expect(api.resendCareContract).toHaveBeenCalledTimes(1);
      // Anker: the contract stays shown.
      expect(screen.getByText("Vertragsangaben")).toBeInTheDocument();
      expect(
        screen.queryByText(
          catalogText("general.unavailable", "die Versandadresse"),
        ),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(catalogText("general.unavailable", "der Vertrag")),
      ).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
      expect(
        await screen.findByText(/Die Mail wartet auf den Versand/),
      ).toBeInTheDocument();
      expect(screen.queryByText(reloadText)).not.toBeInTheDocument();
      expect(api.fetchCareContract).toHaveBeenCalledTimes(2);
      expect(api.resendCareContract).toHaveBeenCalledTimes(1);
    });
  });
});
