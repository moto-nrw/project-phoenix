import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareInvoice,
  CareRunDetail,
} from "~/lib/traeger-care-billing-api";

// The review of a calculated monthly run (#5916): excluded invoices leave
// the school cards for their own card with reason and the way back, the
// figures count only what the release issues, and the release sends the
// reviewed state back.

const { api, review } = vi.hoisted(() => ({
  api: {
    fetchCareRun: vi.fn(),
    fetchCareLetters: vi.fn(),
    releaseCareRun: vi.fn(),
  },
  review: { includeCareInvoice: vi.fn() },
}));
vi.mock("~/lib/traeger-care-billing-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-billing-api")>()),
  ...api,
}));
vi.mock("~/lib/traeger-care-review-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-review-api")>()),
  ...review,
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/abrechnung/monatsbeitraege/7",
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import TraegerCareRunPage from "./page";

function invoice(child: string, contractId: string): CareInvoice {
  return {
    id: "",
    number: "",
    issuedOn: null,
    contractId,
    service: "care",
    schoolId: "3",
    schoolName: "Grundschule Ispringen",
    childFirstName: child,
    childLastName: "Beispiel",
    recipient: { firstName: "Eva", lastName: "Beispiel", email: "" },
    positions: [],
    reductions: [],
    totalCents: 7750,
    exclusion: null,
    mealPositions: [],
  };
}

const RUN: CareRunDetail = {
  id: "7",
  billingMonth: "2026-10-01",
  status: "calculated",
  createdAt: "2026-10-28T08:00:00Z",
  releasedAt: null,
  issuedOn: null,
  documentCount: null,
  totalCents: null,
  monthRule: "version_on_first",
  invoices: [
    invoice("Lea", "21"),
    {
      ...invoice("Mia", "22"),
      exclusion: {
        reason: "Kündigung wird geprüft",
        excludedAt: "2026-10-29T09:00:00Z",
      },
    },
  ],
  unpriced: [],
  duplicates: [],
  duplicateChildren: 0,
  billed: 0,
  previewCount: 1,
  previewCents: 7750,
  numberPattern: "MB-{JJJJ}-{NNNN}",
  numberRangeSet: true,
  invoiceDate: "2026-10-30",
  // Two of the Träger's schools; the run names them, never "Alle".
  schoolIds: ["3", "4"],
  schools: [
    { id: "3", name: "Grundschule Ispringen" },
    { id: "4", name: "Grundschule Birkenfeld" },
  ],
  excludedCount: 1,
  excludedCents: 7750,
  fingerprint: "f1",
  mealGaps: [],
  mealUnbilled: [],
};

function renderPage() {
  render(
    <ToastProvider>
      <TraegerCareRunPage />
    </ToastProvider>,
  );
}

function card(name: string) {
  const section = screen.getByRole("heading", { name }).closest("section");
  if (!section) throw new Error(`card ${name} without its section`);
  return within(section);
}

describe("monthly run review", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.fetchCareRun.mockResolvedValue(RUN);
  });

  it("shows the excluded invoice apart, with its reason, and takes it back", async () => {
    review.includeCareInvoice.mockResolvedValue({
      ...RUN,
      invoices: [invoice("Lea", "21"), invoice("Mia", "22")],
      previewCount: 2,
      previewCents: 15500,
      excludedCount: 0,
      excludedCents: 0,
    });
    renderPage();
    await screen.findByRole("heading", { name: "Ausgeschlossen" });
    const excluded = card("Ausgeschlossen");
    expect(excluded.getByText("Mia Beispiel")).toBeInTheDocument();
    expect(
      excluded.getByText("Grund: Kündigung wird geprüft"),
    ).toBeInTheDocument();
    const school = card("Grundschule Ispringen");
    expect(school.getAllByText("Lea Beispiel").length).toBeGreaterThan(0);
    expect(school.queryAllByText("Mia Beispiel")).toHaveLength(0);
    expect(screen.getByText("30.10.2026")).toBeInTheDocument();
    expect(
      screen.getByText("Grundschule Ispringen, Grundschule Birkenfeld"),
    ).toBeInTheDocument();

    fireEvent.click(
      excluded.getByRole("button", {
        name: "Wieder aufnehmen: Rechnung von Mia Beispiel",
      }),
    );
    expect(
      await screen.findByText(
        "Die Rechnung von Mia Beispiel ist wieder im Lauf.",
      ),
    ).toBeInTheDocument();
    expect(review.includeCareInvoice).toHaveBeenCalledWith("7", "22");
    expect(
      screen.queryByRole("heading", { name: "Ausgeschlossen" }),
    ).toBeNull();
  });

  it("releases the reviewed state with its fingerprint", async () => {
    api.releaseCareRun.mockResolvedValue({ ...RUN, status: "released" });
    api.fetchCareLetters.mockResolvedValue({
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
    });
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Lauf freigeben" }),
    );
    expect(
      await screen.findByText(
        "1 ausgeschlossene Rechnung bekommt keine Nummer.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));
    await screen.findByText(
      "Der Lauf ist freigegeben. Die Rechnungen sind ausgestellt.",
    );
    expect(api.releaseCareRun).toHaveBeenCalledWith(
      expect.objectContaining({ id: "7", previewCount: 1, fingerprint: "f1" }),
    );
  });
  it("closes an all-excluded run without a number range or invoice issuance", async () => {
    const excluded = RUN.invoices.filter((entry) => entry.exclusion);
    const empty = {
      ...RUN,
      invoices: excluded,
      previewCount: 0,
      previewCents: 0,
      numberRangeSet: false,
    };
    api.fetchCareRun.mockResolvedValue(empty);
    api.releaseCareRun.mockResolvedValue({
      ...empty,
      status: "released",
      invoices: [],
    });
    api.fetchCareLetters.mockResolvedValue({
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
    });
    renderPage();
    const release = await screen.findByRole("button", {
      name: "Lauf freigeben",
    });
    expect(release).toBeEnabled();
    fireEvent.click(release);
    expect(
      await screen.findByText(
        "Der Lauf wird ohne Rechnungen abgeschlossen. Die Schulen sind danach wieder frei.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));
    await screen.findByText("Der Lauf ist ohne Rechnungen abgeschlossen.");
    expect(api.releaseCareRun).toHaveBeenCalledWith(empty);
  });
});
