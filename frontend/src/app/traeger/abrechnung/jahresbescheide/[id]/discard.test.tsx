import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { CareNoticeRunDetail } from "~/lib/traeger-care-notices-api";

const { api } = vi.hoisted(() => ({
  api: {
    fetchCareNoticeRun: vi.fn(),
    discardCareNoticeRun: vi.fn(),
  },
}));
vi.mock("~/lib/traeger-care-notices-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-notices-api")>()),
  ...api,
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "4" }),
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/abrechnung/jahresbescheide/4",
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import TraegerCareNoticeRunPage from "./page";

// An open notice run whose schools have no contract in 2026 (#6626).
function run(
  overrides: Partial<CareNoticeRunDetail> = {},
): CareNoticeRunDetail {
  return {
    id: "4",
    year: 2026,
    periodFrom: "2026-09-01",
    periodUntil: "2026-12-31",
    status: "calculated",
    schoolIds: ["3", "5"],
    noticeDate: null,
    createdAt: "2026-09-01T08:00:00Z",
    discardedAt: null,
    issuedOn: null,
    noticeCount: null,
    totalCents: null,
    notices: [],
    unchanged: 0,
    billed: 0,
    unpricedCount: 0,
    duplicateCount: 0,
    previewCount: 0,
    previewCents: 0,
    previewDeltaCents: 0,
    fingerprint: "f",
    numberPattern: "{NNNN}",
    numberRangeSet: true,
    legalRemedyMissing: false,
    ...overrides,
  };
}

function renderPage() {
  render(
    <ToastProvider>
      <TraegerCareNoticeRunPage />
    </ToastProvider>,
  );
}

describe("discarding an empty notice run", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("discards a run without any contract and frees the year", async () => {
    api.fetchCareNoticeRun.mockResolvedValue(run());
    api.discardCareNoticeRun.mockResolvedValue(
      run({ status: "discarded", discardedAt: "2026-11-11T09:00:00Z" }),
    );
    renderPage();

    expect(
      await screen.findByText("Dieser Lauf hat nichts abzurechnen"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Jahreslauf abschließen" }),
    ).toBeNull();
    fireEvent.click(
      screen.getByRole("button", { name: "Leeren Lauf verwerfen" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Verwerfen" }));

    expect(
      await screen.findByText("Dieser Lauf ist verworfen"),
    ).toBeInTheDocument();
    expect(api.discardCareNoticeRun).toHaveBeenCalledWith("4");
    expect(screen.getByText("Verworfen am 11.11.2026")).toBeInTheDocument();
  });

  it("closes a run whose contracts keep their notice, as before", async () => {
    api.fetchCareNoticeRun.mockResolvedValue(run({ unchanged: 2 }));
    renderPage();

    expect(
      await screen.findByRole("button", { name: "Jahreslauf abschließen" }),
    ).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "Leeren Lauf verwerfen" }),
    ).toBeNull();
  });
});
