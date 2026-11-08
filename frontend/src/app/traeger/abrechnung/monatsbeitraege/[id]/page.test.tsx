import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareLetterMailing,
  CareLetters,
  CareRunDetail,
} from "~/lib/traeger-care-billing-api";

const { api } = vi.hoisted(() => ({
  api: {
    fetchCareRun: vi.fn(),
    fetchCareLetters: vi.fn(),
  },
}));
vi.mock("~/lib/traeger-care-billing-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-billing-api")>()),
  ...api,
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/abrechnung/monatsbeitraege/7",
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import TraegerCareRunPage from "./page";

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
  uncountedFamilies: 0,
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
const CANCELLATION = { kind: "cancellation" } as CareLetterMailing;

function letters(overrides: Partial<CareLetters> = {}): CareLetters {
  return {
    letters: null,
    documents: [],
    summary: {
      letters: 0,
      sent: 0,
      queued: 0,
      failed: 0,
      notSent: 0,
      withoutEmail: 0,
      withoutInbox: 0,
    },
    ...overrides,
  };
}

function renderPage() {
  render(
    <ToastProvider>
      <TraegerCareRunPage />
    </ToastProvider>,
  );
}

function lettersCard() {
  const heading = screen.getByRole("heading", {
    name: "Schreiben an die Familien",
  });
  const card = heading.closest("section");
  if (!card) throw new Error("letters card without its section");
  return within(card);
}

describe("monthly run page letters card", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.fetchCareRun.mockResolvedValue(RUN);
  });

  it("leads to the letters page to create them", async () => {
    api.fetchCareLetters.mockResolvedValue(letters());
    renderPage();
    expect(
      await screen.findByText("Noch keine Schreiben erstellt."),
    ).toBeInTheDocument();
    const link = screen.getByRole("link", { name: "Schreiben erstellen" });
    expect(link).toHaveAttribute(
      "href",
      "/traeger/abrechnung/monatsbeitraege/7/schreiben",
    );
    expect(
      lettersCard().getByText("Noch keine Schreiben erstellt."),
    ).toBeInTheDocument();
    // Sending, printing and Storno live on the letters page only.
    expect(screen.queryByRole("button", { name: "An alle senden" })).toBeNull();
    expect(screen.queryByText("An die Familie")).toBeNull();
  });

  it("shows the send state in one line once the letters exist", async () => {
    api.fetchCareLetters.mockResolvedValue(
      letters({
        letters: {
          paymentDueOn: "2027-01-20",
          notes: "",
          createdAt: "2027-01-04T09:00:00Z",
        },
        documents: [CANCELLATION],
        summary: {
          letters: 5,
          sent: 3,
          queued: 0,
          failed: 1,
          notSent: 1,
          withoutEmail: 1,
          withoutInbox: 0,
        },
      }),
    );
    renderPage();
    expect(
      await screen.findByText(
        "3 von 5 gesendet · 1 nicht angekommen · 1 ohne E-Mail · 1 storniert",
      ),
    ).toBeInTheDocument();
    const link = screen.getByRole("link", { name: "Schreiben öffnen" });
    expect(link).toHaveAttribute(
      "href",
      "/traeger/abrechnung/monatsbeitraege/7/schreiben",
    );
    expect(
      lettersCard().getByText(
        "3 von 5 gesendet · 1 nicht angekommen · 1 ohne E-Mail · 1 storniert",
      ),
    ).toBeInTheDocument();
  });

  it("shows no letters card before the release", async () => {
    api.fetchCareLetters.mockResolvedValue(letters());
    api.fetchCareRun.mockResolvedValue({
      ...RUN,
      status: "calculated",
      releasedAt: null,
      issuedOn: null,
    });
    renderPage();
    expect(
      await screen.findByRole("button", { name: "Lauf freigeben" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Schreiben an die Familien" }),
    ).toBeNull();
    expect(api.fetchCareLetters).not.toHaveBeenCalled();
  });
});
