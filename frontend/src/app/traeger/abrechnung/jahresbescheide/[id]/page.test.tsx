import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareNotice,
  CareNoticeRunDetail,
} from "~/lib/traeger-care-notices-api";

const { api, push } = vi.hoisted(() => ({
  api: {
    fetchCareNoticeRun: vi.fn(),
    releaseCareNoticeRun: vi.fn(),
  },
  push: vi.fn(),
}));
vi.mock("~/lib/traeger-care-notices-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-notices-api")>()),
  ...api,
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "4" }),
  useRouter: () => ({ push }),
  usePathname: () => "/abrechnung/jahresbescheide/4",
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import TraegerCareNoticeRunPage from "./page";

// The child of MK-007: 12,00 € and 20,00 € a month from September to
// December, four rates of 32,00 €.
const NOTICE: CareNotice = {
  id: "0",
  number: "",
  issuedOn: null,
  kind: "initial",
  contractId: "71",
  schoolId: "3",
  schoolName: "Grundschule Hardt",
  childFirstName: "Lena",
  childLastName: "Muster",
  recipient: {
    firstName: "Eva",
    lastName: "Muster",
    email: "eva@example.org",
    secondName: "",
    address: null,
  },
  paymentMethod: "direct_debit",
  year: 2025,
  periodFrom: "2025-09-01",
  periodUntil: "2025-12-31",
  lines: [],
  installments: ["09", "10", "11", "12"].map((month) => ({
    month: `2025-${month}-01`,
    dueOn: month === "09" ? "2025-09-24" : `2025-${month}-01`,
    immediatelyDue: month === "09",
    previousCents: 0,
    amountCents: 3200,
    deltaCents: 3200,
  })),
  previousCents: 0,
  totalCents: 12800,
  deltaCents: 12800,
  amends: null,
  amendedBy: null,
  legalRemedy: "",
};

function run(
  overrides: Partial<CareNoticeRunDetail> = {},
): CareNoticeRunDetail {
  return {
    id: "4",
    year: 2025,
    periodFrom: "2025-09-01",
    periodUntil: "2025-12-31",
    status: "calculated",
    schoolIds: ["3"],
    noticeDate: "2025-09-24",
    createdAt: "2025-09-01T08:00:00Z",
    discardedAt: null,
    issuedOn: null,
    noticeCount: null,
    totalCents: null,
    notices: [NOTICE],
    unchanged: 0,
    billed: 0,
    unpricedCount: 0,
    duplicateCount: 0,
    previewCount: 1,
    previewCents: 12800,
    previewDeltaCents: 12800,
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

describe("TraegerCareNoticeRunPage", () => {
  beforeEach(() => {
    api.fetchCareNoticeRun.mockReset();
    api.releaseCareNoticeRun.mockReset();
    push.mockReset();
  });

  it("shows each notice with its rates and opens page 1", async () => {
    api.fetchCareNoticeRun.mockResolvedValue(run());
    renderPage();

    expect((await screen.findAllByText("Lena Muster")).length).toBeGreaterThan(
      0,
    );
    const table = within(screen.getByRole("table"));
    const row = table.getByText("Lena Muster").closest("tr");
    expect(row).not.toBeNull();
    const cells = within(row as HTMLElement);
    expect(cells.getByText("01.09.–31.12.2025")).toBeInTheDocument();
    expect(cells.getByText(/4 Raten · 32,00\s€ im Monat/)).toBeInTheDocument();
    expect(cells.getByText(/128,00\s€/)).toBeInTheDocument();

    fireEvent.click(cells.getByText("Lena Muster"));
    expect(push).toHaveBeenCalledWith(
      "/traeger/abrechnung/jahresbescheide/4/bescheid/71",
    );
  });

  it("names the issued total a claim before payment, not the preview", async () => {
    // #6348: moto sees no incoming payment, so an issued total is a claim.
    const hint = "Forderung vor Zahlungseingang. Zahlungen sieht moto nicht.";
    api.fetchCareNoticeRun.mockResolvedValue(
      run({
        status: "released",
        issuedOn: "2025-09-24",
        notices: [{ ...NOTICE, number: "1.2025.000041.1" }],
      }),
    );
    renderPage();
    expect(await screen.findByText(hint)).toBeInTheDocument();
    cleanup();

    api.fetchCareNoticeRun.mockResolvedValue(run());
    renderPage();
    expect(
      await screen.findByText("Gesamtbetrag aller Bescheide"),
    ).toBeInTheDocument();
    expect(screen.queryByText(hint)).not.toBeInTheDocument();
  });

  it("holds the release back while the legal remedy is missing", async () => {
    api.fetchCareNoticeRun.mockResolvedValue(run({ legalRemedyMissing: true }));
    renderPage();

    expect(
      await screen.findByText("Rechtsbehelfsbelehrung fehlt"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Bescheide ausstellen" }),
    ).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.getByRole("link", { name: "Vorlage öffnen" }),
    ).toHaveAttribute(
      "href",
      "/traeger/einstellungen/briefkopf/vorlagen/bescheid",
    );
  });

  it("releases exactly the reviewed state", async () => {
    const preview = run();
    api.fetchCareNoticeRun.mockResolvedValue(preview);
    api.releaseCareNoticeRun.mockResolvedValue(
      run({
        status: "released",
        issuedOn: "2025-09-24",
        notices: [{ ...NOTICE, number: "1.2025.000041.1" }],
      }),
    );
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Bescheide ausstellen" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Ausstellen" }));

    expect(
      (await screen.findAllByText("1.2025.000041.1")).length,
    ).toBeGreaterThan(0);
    expect(api.releaseCareNoticeRun).toHaveBeenCalledWith(preview);
    expect(
      screen.queryByRole("button", { name: "Bescheide ausstellen" }),
    ).not.toBeInTheDocument();
  });

  it("says why a run of unchanged contracts issues nothing", async () => {
    api.fetchCareNoticeRun.mockResolvedValue(
      run({
        notices: [],
        previewCount: 0,
        previewCents: 0,
        unchanged: 2,
        billed: 1,
        numberRangeSet: false,
        legalRemedyMissing: true,
      }),
    );
    renderPage();

    expect(
      await screen.findByText(/2 Kinder mit gültigem Bescheid/),
    ).toBeInTheDocument();
    expect(screen.getByText(/1 Kind mit Monatsrechnung/)).toBeInTheDocument();
    expect(
      screen.getAllByText("Kein Bescheid auszustellen.").length,
    ).toBeGreaterThan(0);
    expect(
      screen.getByRole("button", { name: "Jahreslauf abschließen" }),
    ).toBeEnabled();
    fireEvent.click(
      screen.getByRole("button", { name: "Jahreslauf abschließen" }),
    );
    api.releaseCareNoticeRun.mockResolvedValue(
      run({
        status: "released",
        notices: [],
        previewCount: 0,
        previewCents: 0,
      }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Abschließen" }));
    expect(await screen.findByText("Abgeschlossen")).toBeInTheDocument();
  });
});
