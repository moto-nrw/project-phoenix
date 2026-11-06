import { SWRConfig, useSWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { resetSWRScope } from "~/lib/swr/scope";
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

let revalidateAll: () => Promise<unknown> = async () => undefined;

/** Hands the test a way to refetch every key, as a focus would. */
function Revalidator() {
  const { mutate } = useSWRConfig();
  revalidateAll = () => mutate(() => true);
  return null;
}

function renderPage() {
  return render(
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      <ToastProvider>
        <Revalidator />
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
    // A refusal holds per key for the whole tab; start every test clean.
    resetSWRScope({ revalidate: false });
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
    // Ein Stichtag für alle Schulen: die Überschrift nennt ihn, keine Spalte.
    expect(
      screen.getAllByRole("heading", { name: "Zahlen je Schule am 15.10.2026" })
        .length,
    ).toBeGreaterThan(0);
    expect(
      within(schoolTable()).getByRole("columnheader", {
        name: /Aktive Kinder/,
      }),
    ).toBeInTheDocument();
    expect(
      within(schoolTable()).queryByRole("columnheader", { name: /^Stichtag/ }),
    ).toBeNull();
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

  it("keeps school and city search available alongside month navigation", async () => {
    mockFetchCounts.mockResolvedValue(COUNTS);
    renderPage();
    await screen.findByText("84 mehr als im September 2026.");

    const search = screen.getByPlaceholderText("Schule oder Ort suchen…");
    fireEvent.change(search, { target: { value: "bach" } });
    expect(
      within(schoolTable()).getByRole("link", { name: "OGS Am Bach" }),
    ).toBeVisible();
    expect(
      within(schoolTable()).queryByRole("link", { name: "OGS Lindenweg" }),
    ).toBeNull();

    fireEvent.change(search, { target: { value: "melle" } });
    expect(
      within(schoolTable()).getByRole("link", { name: "OGS Lindenweg" }),
    ).toBeVisible();
    expect(
      within(schoolTable()).queryByRole("link", { name: "OGS Am Bach" }),
    ).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Monat davor" }));
    expect(await screen.findByText("1 von 2")).toBeVisible();
    expect(
      within(schoolTable()).getByRole("link", { name: "OGS Lindenweg" }),
    ).toBeVisible();
    expect(
      within(schoolTable()).queryByRole("link", { name: "OGS Am Bach" }),
    ).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Suche löschen" }));
    expect(
      within(schoolTable()).getByRole("link", { name: "OGS Am Bach" }),
    ).toBeVisible();
  });

  it("shows every actual key date when schools have different dates in one month", async () => {
    mockFetchCounts.mockResolvedValue({
      ...COUNTS,
      schools: COUNTS.schools.map((school) => ({
        ...school,
        counts: school.counts.map((count) =>
          school.id === "3"
            ? {
                ...count,
                keyDate: "2026-10-20",
                recordedAt: "2026-10-20T04:00:00Z",
              }
            : count,
        ),
      })),
    });
    renderPage();

    await screen.findByText("84 mehr als im September 2026.");
    expect(
      screen.getAllByRole("heading", {
        name: "Zahlen je Schule im Oktober 2026",
      }).length,
    ).toBeGreaterThan(0);
    const bach = within(schoolTable())
      .getByRole("link", { name: "OGS Am Bach" })
      .closest("tr") as HTMLElement;
    expect(within(bach).getByText("15.10.2026")).toBeVisible();
    const linden = within(schoolTable())
      .getByRole("link", { name: "OGS Lindenweg" })
      .closest("tr") as HTMLElement;
    expect(within(linden).getByText("20.10.2026")).toBeVisible();

    const months = screen.getAllByRole("table")[1] as HTMLElement;
    expect(within(months).getByText("15.10.2026, 20.10.2026")).toBeVisible();
    fireEvent.change(screen.getByPlaceholderText("Schule oder Ort suchen…"), {
      target: { value: "melle" },
    });
    expect(within(months).getByText("15.10.2026, 20.10.2026")).toBeVisible();
  });

  it("labels all key dates below the month in the phone layout", async () => {
    mockFetchCounts.mockResolvedValue({
      schools: [15, 20, 25].map((day) => ({
        id: String(day),
        name: `OGS ${day}`,
        slug: `ogs-${day}`,
        subdomain: `ogs-${day}`,
        city: "Melle",
        counts: [{ ...count("10", 10, 1), keyDate: `2026-10-${day}` }],
      })),
      months: [
        {
          period: "2026-10-01",
          keyDate: "2026-10-15",
          activeStudents: 30,
          activeTerminals: 3,
          schools: 3,
        },
      ],
    } satisfies TraegerKeyDateCounts);
    renderPage();

    const dates = "15.10.2026, 20.10.2026, 25.10.2026";
    await screen.findAllByText(dates);
    const month = screen.getByRole("button", {
      name: /Oktober 2026.*Aktive Kinder/,
    });
    const field = within(month).getByText(dates).closest("dd");
    expect(field).toBeInTheDocument();
    expect(field?.previousElementSibling).toHaveTextContent("Stichtage");
    fireEvent.click(month);
    expect(screen.getByText("3 von 3")).toBeVisible();
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

  it.each([403, 404])(
    "drops the loaded counts, also in the page head, when a refresh answers %i",
    async (status) => {
      mockFetchCounts.mockResolvedValueOnce(COUNTS);
      renderPage();
      await screen.findByText("84 mehr als im September 2026.");
      expect(screen.getByText("Monate erfasst")).toBeInTheDocument();

      mockFetchCounts.mockRejectedValue(new ApiError("refused", status));
      await act(async () => {
        await revalidateAll();
      });

      expect(await screen.findByRole("alert")).toBeInTheDocument();
      expect(screen.queryByText("Monate erfasst")).not.toBeInTheDocument();
      expect(
        screen.queryByText("84 mehr als im September 2026."),
      ).not.toBeInTheDocument();
    },
  );

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
