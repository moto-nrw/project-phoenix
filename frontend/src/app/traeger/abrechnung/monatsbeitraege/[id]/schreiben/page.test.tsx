import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareLetterMailing,
  CareLetters,
  CareRunDetail,
} from "~/lib/traeger-care-billing-api";

const { api, push } = vi.hoisted(() => ({
  api: {
    fetchCareRun: vi.fn(),
    fetchCareLetters: vi.fn(),
    fetchCareRuns: vi.fn(),
    startCareRun: vi.fn(),
  },
  push: vi.fn(),
}));
vi.mock("~/lib/traeger-care-billing-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-billing-api")>()),
  ...api,
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  useRouter: () => ({ push }),
  usePathname: () => "/abrechnung/monatsbeitraege/7/schreiben",
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import TraegerCareRunLettersPage from "./page";

const RUN: CareRunDetail = {
  id: "7",
  billingMonth: "2027-01-01",
  status: "released",
  createdAt: "2027-01-04T08:00:00Z",
  releasedAt: "2027-01-04T08:00:00Z",
  issuedOn: "2027-01-04",
  documentCount: 1,
  totalCents: 9600,
  monthRule: "version_on_first",
  invoices: [],
  unpriced: [],
  duplicates: [],
  duplicateChildren: 0,
  billed: 0,
  previewCount: 1,
  previewCents: 9600,
  numberPattern: "MB-{NNNN}",
  numberRangeSet: true,
  schoolIds: ["3"],
  invoiceDate: null,
  schools: [],
  excludedCount: 0,
  excludedCents: 0,
  fingerprint: "",
};
const CANCELLATION: CareLetterMailing = {
  invoiceId: "41",
  kind: "cancellation",
  number: "MB-0002",
  invoiceNumber: "MB-0001",
  issuedOn: "2027-01-04",
  childFirstName: "Mia",
  childLastName: "Beispiel",
  schoolName: "OGS",
  recipientFirstName: "Eva",
  recipientLastName: "Beispiel",
  recipientEmail: "eva@example.org",
  totalCents: -9600,
  reason: "Falscher Vertrag",
  cancelled: true,
  cancellationNumber: "MB-0002",
  hasLetter: false,
  inInbox: true,
  status: "not_sent",
  sends: 0,
  attempts: 0,
  requestedAt: null,
  sentAt: null,
};
const letters = (documents: CareLetterMailing[]): CareLetters => ({
  letters: null,
  documents,
  summary: {
    letters: 0,
    sent: 0,
    queued: 0,
    failed: 0,
    notSent: 0,
    withoutEmail: 0,
    withoutInbox: 0,
  },
});

function renderPage() {
  render(
    <ToastProvider>
      <TraegerCareRunLettersPage />
    </ToastProvider>,
  );
}

describe("monthly invoice reissue", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.fetchCareRun.mockResolvedValue(RUN);
    api.fetchCareRuns.mockResolvedValue({ runs: [] });
  });

  it("hides reissue when every cancellation has a released replacement", async () => {
    api.fetchCareLetters.mockResolvedValue(
      letters([{ ...CANCELLATION, replaced: true }]),
    );
    renderPage();
    await screen.findByRole("button", { name: "Schreiben erstellen" });
    expect(
      screen.queryByRole("button", { name: "Neu ausstellen" }),
    ).not.toBeInTheDocument();
  });

  it("counts only cancellations that still need a replacement", async () => {
    api.fetchCareLetters.mockResolvedValue(
      letters([
        CANCELLATION,
        { ...CANCELLATION, invoiceId: "42", replaced: true },
      ]),
    );
    renderPage();
    expect(
      await screen.findByText("1 Rechnung ist storniert"),
    ).toBeInTheDocument();
    api.startCareRun.mockResolvedValue({ id: "8" });
    fireEvent.click(screen.getByRole("button", { name: "Neu ausstellen" }));
    await waitFor(() =>
      expect(push).toHaveBeenCalledWith(
        "/traeger/abrechnung/monatsbeitraege/8",
      ),
    );
    expect(api.startCareRun).toHaveBeenCalledWith({
      billingMonth: "2027-01-01",
      schoolIds: ["3"],
      invoiceDate: null,
    });
  });

  it("does not create an empty run when another tab already issued the replacement", async () => {
    api.fetchCareLetters
      .mockResolvedValueOnce(letters([CANCELLATION]))
      .mockResolvedValue(letters([{ ...CANCELLATION, replaced: true }]));
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Neu ausstellen" }),
    );
    await waitFor(() => expect(api.fetchCareLetters).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Neu ausstellen" }),
      ).not.toBeInTheDocument(),
    );
    expect(
      screen.getByRole("button", { name: "Schreiben erstellen" }),
    ).toBeInTheDocument();
    expect(api.startCareRun).not.toHaveBeenCalled();
    expect(api.fetchCareRuns).not.toHaveBeenCalled();
  });
});
