import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { BillingRunDetail } from "~/lib/traeger-billing-api";

const { api } = vi.hoisted(() => ({
  api: {
    fetchBillingRun: vi.fn(),
    discardBillingRun: vi.fn(),
    fetchBillingRuns: vi.fn(),
    startBillingRun: vi.fn(),
  },
}));
vi.mock("~/lib/traeger-billing-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-billing-api")>()),
  ...api,
}));
const { push } = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "9" }),
  useRouter: () => ({ push }),
  usePathname: () => "/abrechnung/9",
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { permissions: [] } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import TraegerBillingRunPage from "./page";

// An open holiday run whose registrations were all cancelled (#6628).
function run(overrides: Partial<BillingRunDetail> = {}): BillingRunDetail {
  return {
    id: "9",
    holidayPeriodId: "5",
    periodName: "Winterferien 2027",
    serviceFrom: "2027-01-04",
    serviceUntil: "2027-01-15",
    status: "calculated",
    createdAt: "2026-12-01T08:00:00Z",
    releasedAt: null,
    discardedAt: null,
    issuedOn: null,
    documentCount: null,
    totalCents: null,
    previewCount: 0,
    previewCents: 0,
    unbilled: 0,
    missingDebtorNumbers: 0,
    paymentTextMissingDebtorNumbers: 0,
    numberPattern: "FE-{NNNN}",
    numberRangeSet: true,
    fingerprint: "c0ffee",
    letters: null,
    documents: [],
    dueDates: [],
    documentKind: "invoice",
    legalRemedyMissing: false,
    ...overrides,
  } as BillingRunDetail;
}

function renderPage() {
  render(
    <ToastProvider>
      <TraegerBillingRunPage />
    </ToastProvider>,
  );
}

describe("discarding an empty holiday run", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("offers the discard instead of a locked release and confirms it", async () => {
    api.fetchBillingRun.mockResolvedValue(run());
    api.discardBillingRun.mockResolvedValue(
      run({ status: "discarded", discardedAt: "2026-12-02T09:00:00Z" }),
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
        /Der Lauf für Winterferien 2027 stellt nichts aus/,
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Verwerfen" }));

    expect(
      await screen.findByText("Dieser Lauf ist verworfen"),
    ).toBeInTheDocument();
    expect(api.discardBillingRun).toHaveBeenCalledWith("9");
    expect(screen.getByText("Verworfen am 02.12.2026")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Leeren Lauf verwerfen" }),
    ).toBeNull();
  });

  it("starts a new run of the period from a discarded run", async () => {
    api.fetchBillingRun.mockResolvedValue(
      run({ status: "discarded", discardedAt: "2026-12-02T09:00:00Z" }),
    );
    api.fetchBillingRuns.mockResolvedValue([]);
    api.startBillingRun.mockResolvedValue(run({ id: "10" }));
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Neuen Lauf starten" }),
    );
    await vi.waitFor(() =>
      expect(push).toHaveBeenCalledWith("/traeger/abrechnung/10"),
    );
    expect(api.startBillingRun).toHaveBeenCalledWith("5");
  });

  it("keeps the release for a run that waits for a contribution", async () => {
    api.fetchBillingRun.mockResolvedValue(run({ unbilled: 1 }));
    renderPage();

    expect(
      await screen.findByRole("button", { name: "Lauf freigeben" }),
    ).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.queryByRole("button", { name: "Leeren Lauf verwerfen" }),
    ).toBeNull();
    expect(screen.queryByText("Dieser Lauf hat nichts abzurechnen")).toBeNull();
  });
});
