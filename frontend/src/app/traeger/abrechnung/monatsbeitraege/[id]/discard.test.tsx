import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { CareRunDetail } from "~/lib/traeger-care-billing-api";

const { api } = vi.hoisted(() => ({
  api: {
    fetchCareRun: vi.fn(),
    fetchCareLetters: vi.fn(),
    discardCareRun: vi.fn(),
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

// An open run whose schools have nothing to bill in January 2027 (#6626).
function run(overrides: Partial<CareRunDetail> = {}): CareRunDetail {
  return {
    id: "7",
    billingMonth: "2027-01-01",
    status: "calculated",
    createdAt: "2027-01-04T08:00:00Z",
    releasedAt: null,
    discardedAt: null,
    issuedOn: null,
    documentCount: null,
    totalCents: null,
    monthRule: "version_on_first",
    invoices: [],
    unpriced: [],
    duplicates: [],
    duplicateChildren: 0,
    uncountedFamilies: 0,
    billed: 0,
    previewCount: 0,
    previewCents: 0,
    numberPattern: "MB-{NNNN}",
    numberRangeSet: true,
    mealGaps: [],
    mealUnbilled: [],
    mealChanged: [],
    schoolIds: ["3"],
    invoiceDate: null,
    schools: [{ id: "3", name: "Grundschule Ispringen" }],
    excludedCount: 0,
    excludedCents: 0,
    fingerprint: "f",
    costBearers: [],
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

describe("discarding an empty monthly run", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("offers the discard instead of a locked release and confirms it", async () => {
    api.fetchCareRun.mockResolvedValue(run());
    api.discardCareRun.mockResolvedValue(
      run({ status: "discarded", discardedAt: "2027-01-05T09:00:00Z" }),
    );
    renderPage();

    expect(
      await screen.findByText("Dieser Lauf hat nichts abzurechnen"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Lauf freigeben" })).toBeNull();
    fireEvent.click(
      screen.getByRole("button", { name: "Leeren Lauf verwerfen" }),
    );
    expect(
      await screen.findByText(
        /Der Monatslauf für Januar 2027 stellt nichts aus/,
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Verwerfen" }));

    expect(
      await screen.findByText("Dieser Lauf ist verworfen"),
    ).toBeInTheDocument();
    expect(api.discardCareRun).toHaveBeenCalledWith("7");
    expect(screen.getByText("Verworfen am 05.01.2027")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Leeren Lauf verwerfen" }),
    ).toBeNull();
  });

  it("keeps the release for a run that waits for a price", async () => {
    api.fetchCareRun.mockResolvedValue(
      run({
        unpriced: [
          {
            contractId: "9",
            schoolId: "3",
            schoolName: "Grundschule Ispringen",
            childFirstName: "Mia",
            childLastName: "Beispiel",
            careOfferingId: "1",
            moduleName: "Betreuung bis 14:00 Uhr",
            weekday: "mon",
            daysPerWeek: null,
          },
        ],
      }),
    );
    renderPage();

    expect(
      await screen.findByRole("button", { name: "Lauf freigeben" }),
    ).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.queryByRole("button", { name: "Leeren Lauf verwerfen" }),
    ).toBeNull();
    expect(screen.queryByText("Dieser Lauf hat nichts abzurechnen")).toBeNull();
  });

  it("keeps the notice hold and recovery link visible for an empty run", async () => {
    api.fetchCareRun.mockResolvedValue(
      run({
        feeNoticeHeld: [
          {
            contractId: "41",
            childFirstName: "Mia",
            childLastName: "Muster",
            noticeNumber: "MB-0003",
          },
        ],
      }),
    );
    renderPage();

    expect(
      await screen.findByText("Dieser Lauf hat nichts abzurechnen"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Leeren Lauf verwerfen" }),
    ).toBeEnabled();
    expect(
      screen.getByText("Durch aufgehobenen Bescheid gesperrt"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Der Jahresbescheid von Mia Muster setzt diesen Monat auf 0,00 €. Der Monat bleibt gesperrt. Dieser Lauf rechnet dafür keine Betreuung ab. Wollen Sie den Monat abrechnen, stellen Sie einen neuen Bescheid aus.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Neuen Bescheid ausstellen" }),
    ).toHaveAttribute("href", "/traeger/abrechnung/jahresbescheide");
  });
});
