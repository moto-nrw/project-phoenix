import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { TraegerKeyDateCounts } from "~/lib/traeger-api";

const { mockFetchCounts, mockUseSession } = vi.hoisted(() => ({
  mockFetchCounts: vi.fn(),
  mockUseSession: vi.fn(),
}));

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));
vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));
vi.mock("next/navigation", () => ({
  usePathname: () => "/stichtagszahlen",
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("~/lib/traeger-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/traeger-api")>()),
  fetchTraegerKeyDateCounts: mockFetchCounts,
}));
vi.mock("~/env.client", () => ({
  clientEnv: {
    NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de",
    NEXT_PUBLIC_TRAEGER_HOSTNAME: "traeger.moto-app.de",
  },
}));

import TraegerKeyDateCountsPage from "./page";

function count(
  month: string,
  students: number,
  terminals: number,
  recordedAt?: string,
) {
  return {
    period: `2026-${month}-01`,
    keyDate: `2026-${month}-15`,
    activeStudents: students,
    activeTerminals: terminals,
    recordedAt: recordedAt ?? `2026-${month}-15T04:00:00Z`,
  };
}

const COUNTS: TraegerKeyDateCounts = {
  schools: [
    {
      id: "2",
      name: "OGS Am Bach",
      slug: "am-bach",
      subdomain: "am-bach",
      city: "Bissendorf",
      counts: [
        count("10", 124, 3, "2026-10-16T08:00:00Z"),
        count("09", 120, 2),
      ],
    },
    {
      id: "3",
      name: "OGS Lindenweg",
      slug: "lindenweg",
      subdomain: "lindenweg",
      city: "Melle",
      counts: [count("10", 80, 1)],
    },
  ],
  months: [
    {
      period: "2026-10-01",
      keyDate: "2026-10-15",
      activeStudents: 204,
      activeTerminals: 4,
      schools: 2,
    },
    {
      period: "2026-09-01",
      keyDate: "2026-09-15",
      activeStudents: 120,
      activeTerminals: 2,
      schools: 1,
    },
  ],
};

function renderPage() {
  return render(
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      <ToastProvider>
        <TraegerKeyDateCountsPage />
      </ToastProvider>
    </SWRConfig>,
  );
}

function schoolTable() {
  return screen.getAllByRole("table")[0] as HTMLElement;
}

describe("TraegerKeyDateCountsPage", () => {
  beforeEach(() => {
    mockFetchCounts.mockReset();
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "A",
          scope: "org",
          orgId: 7,
          permissions: ["traeger_overview:read"],
        },
      },
      status: "authenticated",
    });
  });

  it("shows the newest month for all schools with the change to the month before", async () => {
    mockFetchCounts.mockResolvedValue(COUNTS);
    renderPage();

    expect(
      await screen.findByText("84 mehr als im September 2026."),
    ).toBeInTheDocument();
    expect(screen.getAllByText("204").length).toBeGreaterThan(0);
    expect(
      screen.getByRole("heading", { name: "Stichtagszahlen" }),
    ).toBeVisible();
    expect(
      screen.getByText("84 mehr als im September 2026."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("2 mehr als im September 2026."),
    ).toBeInTheDocument();
    expect(screen.getByText("2 von 2")).toBeInTheDocument();
    expect(
      screen.getByText("Für jede Schule gibt es Zahlen zu diesem Stichtag."),
    ).toBeInTheDocument();
  });

  it("lists every school with its figures, its change and a late capture", async () => {
    mockFetchCounts.mockResolvedValue(COUNTS);
    renderPage();
    await screen.findByText("84 mehr als im September 2026.");

    const bach = within(schoolTable())
      .getByRole("link", { name: "OGS Am Bach" })
      .closest("tr") as HTMLElement;
    expect(within(bach).getByText("124")).toBeInTheDocument();
    expect(within(bach).getByText("+4")).toBeInTheDocument();
    expect(within(bach).getByText("+1")).toBeInTheDocument();
    expect(within(bach).getByText("Nachgeholt")).toBeInTheDocument();
    expect(
      within(schoolTable()).getByRole("link", { name: "OGS Am Bach" }),
    ).toHaveAttribute("href", "/traeger/schulen/2");

    const linden = within(schoolTable())
      .getByRole("link", { name: "OGS Lindenweg" })
      .closest("tr") as HTMLElement;
    expect(within(linden).getByText("80")).toBeInTheDocument();
    expect(within(linden).queryByText("Nachgeholt")).not.toBeInTheDocument();
  });

  it("switches to an earlier month from the list of all months", async () => {
    mockFetchCounts.mockResolvedValue(COUNTS);
    renderPage();
    await screen.findByText("84 mehr als im September 2026.");

    const months = screen.getAllByRole("table")[1] as HTMLElement;
    fireEvent.click(
      within(months).getByText("September 2026").closest("tr") as HTMLElement,
    );

    expect(await screen.findByText("1 von 2")).toBeInTheDocument();
    expect(
      screen.getByText("Eine Schule hat zu diesem Stichtag keine Zahlen."),
    ).toBeInTheDocument();
    const linden = within(schoolTable())
      .getByRole("link", { name: "OGS Lindenweg" })
      .closest("tr") as HTMLElement;
    expect(
      within(linden).getByText("Keine Zahlen zu diesem Stichtag"),
    ).toBeInTheDocument();
  });

  it("says when no month is captured yet", async () => {
    mockFetchCounts.mockResolvedValue({
      schools: COUNTS.schools.map((school) => ({ ...school, counts: [] })),
      months: [],
    });
    renderPage();

    expect(
      await screen.findByText("Noch keine Stichtagszahlen"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Aktive Kinder", { selector: "p" }),
    ).not.toBeInTheDocument();
  });

  it("shows a refusal instead of figures", async () => {
    mockFetchCounts.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "general.forbidden" }),
    );
    renderPage();

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByText("Aktive Kinder", { selector: "p" }),
    ).not.toBeInTheDocument();
  });
});
