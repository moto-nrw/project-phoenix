import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { TEST_CLOCK_INSTANT } from "~/test/clock";
import type { TraegerSchoolsToday } from "~/lib/traeger-api";

const { mockFetchToday, mockStartSession, mockUseSession, mockPush } =
  vi.hoisted(() => ({
    mockFetchToday: vi.fn(),
    mockStartSession: vi.fn(),
    mockUseSession: vi.fn(),
    mockPush: vi.fn(),
  }));

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));

vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));
vi.mock("next/navigation", () => ({
  usePathname: () => "/uebersicht",
  useRouter: () => ({ push: mockPush }),
}));
vi.mock("~/lib/traeger-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/traeger-api")>()),
  fetchTraegerSchoolsToday: mockFetchToday,
  startTraegerSchoolSession: mockStartSession,
}));
vi.mock("~/env.client", () => ({
  clientEnv: {
    NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de",
    NEXT_PUBLIC_TRAEGER_HOSTNAME: "traeger.moto-app.de",
  },
}));

import TraegerDashboardPage from "./page";

const AS_OF = TEST_CLOCK_INSTANT.toISOString();

function figures(
  childrenTotal: number,
  childrenPresent: number,
  childrenSick: number,
  staffTotal: number,
  staffPresent: number,
) {
  return {
    childrenTotal,
    childrenPresent,
    childrenSick,
    staffTotal,
    staffPresent,
    careRatio: staffPresent > 0 ? childrenPresent / staffPresent : null,
    asOf: AS_OF,
  };
}

const TODAY: TraegerSchoolsToday = {
  schools: [
    {
      id: "2",
      name: "OGS Am Bach",
      slug: "am-bach",
      subdomain: "am-bach",
      city: "Bissendorf",
      figures: figures(120, 51, 4, 12, 6),
    },
    {
      id: "3",
      name: "OGS Lindenweg",
      slug: "lindenweg",
      subdomain: "lindenweg",
      city: "Melle",
      figures: figures(80, 29, 1, 5, 0),
    },
  ],
  total: figures(200, 80, 5, 17, 6),
};

function sessionWith(permissions: string[]) {
  return {
    data: { user: { id: "A", scope: "org", orgId: 7, permissions } },
    status: "authenticated",
  };
}

function renderPage() {
  return render(
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      <ToastProvider>
        <TraegerDashboardPage />
      </ToastProvider>
    </SWRConfig>,
  );
}

describe("TraegerDashboardPage", () => {
  beforeEach(() => {
    mockFetchToday.mockReset();
    mockStartSession.mockReset();
    mockPush.mockReset();
    mockUseSession.mockReturnValue(sessionWith(["traeger_overview:read"]));
  });

  it("sums the figures of all schools with an explaining sentence each", async () => {
    mockFetchToday.mockResolvedValue(TODAY);
    renderPage();

    expect(await screen.findByText("1 : 13,3")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Dashboard" })).toBeVisible();
    for (const [label, sentence] of [
      ["Kinder gesamt", "Alle Kinder, die Ihre 2 Schulen betreuen."],
      ["Jetzt anwesend", /^40\s%\saller Kinder sind gerade in der Schule\.$/],
      ["Heute krank", "Kinder mit einer Krankmeldung für heute."],
      [
        "Betreuungskräfte anwesend",
        "Zählt, wer in der Zeiterfassung angemeldet ist. Homeoffice zählt nicht.",
      ],
      [
        "Betreuungsschlüssel",
        "Eine anwesende Betreuungskraft kommt im Schnitt auf so viele anwesende Kinder.",
      ],
    ] as const) {
      // Die Kachel trägt ihr Label als Absatz, die Tabelle als Spaltenkopf.
      expect(screen.getByText(label, { selector: "p" })).toBeInTheDocument();
      expect(screen.getByText(sentence)).toBeInTheDocument();
    }
    expect(screen.getByText("200")).toBeInTheDocument();
    expect(screen.getByText("80")).toBeInTheDocument();
    expect(screen.getByText("6 von 17")).toBeInTheDocument();
  });

  it("lists every school with its figures and leads into its view", async () => {
    mockFetchToday.mockResolvedValue(TODAY);
    renderPage();

    const table = await screen.findByRole("table");
    const link = within(table).getByRole("link", { name: "OGS Am Bach" });
    expect(link).toHaveAttribute("href", "/traeger/schulen/2");
    expect(
      within(table).getByRole("progressbar", {
        name: "51 von 120 Kindern anwesend",
      }),
    ).toBeInTheDocument();
    expect(within(table).getByText("1 : 8,5")).toBeInTheDocument();
    const quietRow = within(table)
      .getByRole("link", { name: "OGS Lindenweg" })
      .closest("tr");
    expect(quietRow).not.toBeNull();
    expect(
      within(quietRow as HTMLElement).getByText(
        ", keine Betreuungskraft in der Zeiterfassung angemeldet",
      ),
    ).toBeInTheDocument();

    fireEvent.click(quietRow as HTMLElement);
    expect(mockPush).toHaveBeenCalledWith("/traeger/schulen/3");
  });

  it("finds a school by its town", async () => {
    mockFetchToday.mockResolvedValue(TODAY);
    renderPage();
    const table = await screen.findByRole("table");

    // Die Kopfkarte hat ein Suchfeld je Breite; beide teilen den Wert.
    const [field] = screen.getAllByPlaceholderText("Schule oder Ort suchen…");
    fireEvent.change(field as HTMLElement, { target: { value: "melle" } });

    expect(
      await within(table).findByRole("link", { name: "OGS Lindenweg" }),
    ).toBeInTheDocument();
    expect(
      within(table).queryByRole("link", { name: "OGS Am Bach" }),
    ).not.toBeInTheDocument();
  });

  it("opens the Schul-App only with the right", async () => {
    mockFetchToday.mockResolvedValue(TODAY);
    renderPage();
    const table = await screen.findByRole("table");
    expect(
      within(table).getByRole("link", { name: "OGS Am Bach" }),
    ).toBeInTheDocument();
    expect(
      within(table).queryByRole("button", {
        name: "Schul-App öffnen: OGS Am Bach, neuer Tab",
      }),
    ).not.toBeInTheDocument();
  });

  it("opens the Schul-App from the row with the right", async () => {
    mockUseSession.mockReturnValue(
      sessionWith(["traeger_overview:read", "traeger_school:switch"]),
    );
    mockFetchToday.mockResolvedValue(TODAY);
    mockStartSession.mockResolvedValue("token");
    const tab = { opener: {}, location: { href: "" }, close: vi.fn() };
    const openSpy = vi
      .spyOn(globalThis, "open")
      .mockReturnValue(tab as unknown as Window);
    renderPage();

    const table = await screen.findByRole("table");
    fireEvent.click(
      await within(table).findByRole("button", {
        name: "Schul-App öffnen: OGS Am Bach, neuer Tab",
      }),
    );
    expect(openSpy).toHaveBeenCalledWith("", "_blank");
    expect(mockStartSession).toHaveBeenCalledWith("2");
    expect(mockPush).not.toHaveBeenCalled();
    openSpy.mockRestore();
  });

  it("shows a refusal instead of figures", async () => {
    mockFetchToday.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "general.forbidden" }),
    );
    renderPage();

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByText("Kinder gesamt", { selector: "p" }),
    ).not.toBeInTheDocument();
  });
});
