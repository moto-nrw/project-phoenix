import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { releaseFakeTimers, TEST_CLOCK_INSTANT } from "~/test/clock";
import type {
  TraegerAlertKind,
  TraegerSchoolAlerts,
  TraegerSchoolsAlerts,
  TraegerSchoolsToday,
} from "~/lib/traeger-api";

const {
  mockFetchToday,
  mockFetchAlerts,
  mockStartSession,
  mockUseSession,
  mockPush,
} = vi.hoisted(() => ({
  mockFetchToday: vi.fn(),
  mockFetchAlerts: vi.fn(),
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
  fetchTraegerSchoolsAlerts: mockFetchAlerts,
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

/** Am Bach is quiet; Lindenweg has children without staff and waiting ones. */
const ALERTS: TraegerSchoolsAlerts = {
  thresholds: { childrenPerStaff: 20, sickPercent: 10, openEnrollments: 1 },
  schools: [
    { ...TODAY.schools[0]!, openEnrollments: 0, alerts: [] },
    {
      ...TODAY.schools[1]!,
      openEnrollments: 4,
      alerts: ["staff_shortage", "open_enrollments"],
    },
  ],
  asOf: AS_OF,
};

function school(
  id: string,
  name: string,
  alerts: TraegerAlertKind[],
  openEnrollments = 0,
): TraegerSchoolAlerts {
  return {
    id,
    name,
    slug: name,
    subdomain: name,
    city: "Melle",
    figures: {
      childrenTotal: 40,
      childrenPresent: 30,
      childrenSick: 8,
      staffTotal: 4,
      staffPresent: 1,
      careRatio: 30,
      asOf: AS_OF,
    },
    openEnrollments,
    alerts,
  };
}

function alertsOf(schools: TraegerSchoolAlerts[]): TraegerSchoolsAlerts {
  return {
    thresholds: { childrenPerStaff: 20, sickPercent: 10, openEnrollments: 1 },
    schools,
    asOf: AS_OF,
  };
}

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
    mockFetchAlerts.mockReset();
    mockFetchAlerts.mockResolvedValue(ALERTS);
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

  it.each([403, 404])(
    "shows a refusal instead of figures for HTTP %i even when hints load",
    async (status) => {
      mockFetchToday.mockRejectedValue(new ApiError("refused", status));
      await act(async () => {
        renderPage();
      });

      // Anker: Die Zugriffssperre ist nach beiden Antworten sichtbar.
      expect(await screen.findByRole("alert")).toBeInTheDocument();
      expect(
        screen.queryByText("Kinder gesamt", { selector: "p" }),
      ).not.toBeInTheDocument();
    },
  );

  it("keeps the alert snapshot visible when today's initial request fails", async () => {
    mockFetchToday.mockRejectedValue(
      new ApiError("boom", 500, { code: "general.internal" }),
    );
    renderPage();

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(await screen.findByText("200")).toBeInTheDocument();
    expect(screen.getByText("80")).toBeInTheDocument();
    expect(screen.getByText("6 von 17")).toBeInTheDocument();
    expect(screen.getByText("Stand 12:00 Uhr")).toBeInTheDocument();
    const card = screen.getByTestId("traeger-school-alerts");
    expect(
      within(card).getByRole("link", { name: "Zur Schule: OGS Lindenweg" }),
    ).toBeInTheDocument();
    const table = screen.getByRole("table");
    expect(
      within(table).getByRole("link", { name: "OGS Am Bach" }),
    ).toBeInTheDocument();
    expect(within(table).getAllByText(", erreicht Ihre Grenze")).toHaveLength(
      2,
    );
    expect(
      screen.getByRole("button", { name: "Wiederholen" }),
    ).toBeInTheDocument();
  });

  it("keeps the load error blocking when neither request has a snapshot", async () => {
    const failure = new ApiError("boom", 500, { code: "general.internal" });
    mockFetchToday.mockRejectedValue(failure);
    mockFetchAlerts.mockRejectedValue(failure);
    renderPage();

    // Anker: Ohne bestätigte Zahlen erscheint der Ladefehler.
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByText("Kinder gesamt", { selector: "p" }),
    ).not.toBeInTheDocument();
  });

  it("shows the hints of every school and marks the figures in the table", async () => {
    mockFetchToday.mockResolvedValue(TODAY);
    renderPage();

    const card = await screen.findByTestId("traeger-school-alerts");
    expect(within(card).getByText("1 Schule")).toBeInTheDocument();
    expect(
      await within(card).findByRole("link", {
        name: "Zur Schule: OGS Lindenweg",
      }),
    ).toHaveAttribute("href", "/traeger/schulen/3");
    expect(
      within(card).queryByRole("link", { name: "Zur Schule: OGS Am Bach" }),
    ).not.toBeInTheDocument();

    const table = screen.getByRole("table");
    const lindenweg = within(table)
      .getByRole("link", { name: "OGS Lindenweg" })
      .closest("tr") as HTMLElement;
    const amBach = within(table)
      .getByRole("link", { name: "OGS Am Bach" })
      .closest("tr") as HTMLElement;
    // Wenig Personal und wartende Kinder: zwei markierte Zahlen.
    expect(
      within(lindenweg).getAllByText(", erreicht Ihre Grenze"),
    ).toHaveLength(2);
    expect(within(lindenweg).getByText("4")).toBeInTheDocument();
    expect(within(amBach).getByText("1 : 8,5")).toBeInTheDocument();
    expect(
      within(amBach).queryByText(", erreicht Ihre Grenze"),
    ).not.toBeInTheDocument();
  });

  it("uses the alert snapshot for table figures, warning markers, totals and timestamp", async () => {
    mockFetchToday.mockResolvedValue(TODAY);
    mockFetchAlerts.mockResolvedValue({
      ...ALERTS,
      asOf: new Date(TEST_CLOCK_INSTANT.getTime() - 60_000).toISOString(),
      schools: [
        {
          ...ALERTS.schools[0]!,
          figures: figures(40, 30, 8, 4, 1),
          alerts: ["staff_shortage", "sick_children"],
        },
        {
          ...ALERTS.schools[1]!,
          figures: figures(80, 20, 1, 5, 2),
          alerts: ["open_enrollments"],
        },
      ],
    } satisfies TraegerSchoolsAlerts);
    renderPage();

    const table = await screen.findByRole("table");
    const amBach = within(table)
      .getByRole("link", { name: "OGS Am Bach" })
      .closest("tr")!;
    expect(await within(amBach).findByText("1 : 30")).toBeInTheDocument();
    expect(
      within(amBach).getByRole("progressbar", {
        name: "30 von 40 Kindern anwesend",
      }),
    ).toBeInTheDocument();
    expect(within(amBach).getByText("8")).toBeInTheDocument();
    expect(within(amBach).getAllByText(", erreicht Ihre Grenze")).toHaveLength(
      3,
    );

    const lindenweg = within(table)
      .getByRole("link", { name: "OGS Lindenweg" })
      .closest("tr")!;
    // Anker: Der angezeigte Schlüssel gehört zu den bestätigten Hinweisen.
    expect(within(lindenweg).getByText("1 : 10")).toBeInTheDocument();
    expect(
      within(lindenweg).getAllByText(", erreicht Ihre Grenze"),
    ).toHaveLength(1);
    expect(screen.getByText("120")).toBeInTheDocument();
    expect(screen.getByText("50")).toBeInTheDocument();
    expect(screen.getByText("9")).toBeInTheDocument();
    expect(screen.getByText("3 von 9")).toBeInTheDocument();
    expect(screen.getByText("1 : 16,7")).toBeInTheDocument();
    expect(screen.getByText("Stand 11:59 Uhr")).toBeInTheDocument();

    const [field] = screen.getAllByPlaceholderText("Schule oder Ort suchen…");
    fireEvent.change(field!, { target: { value: "melle" } });
    expect(screen.getByText("50")).toBeInTheDocument();
  });

  it("keeps the whole alert snapshot when only today's figures refresh", async () => {
    vi.useFakeTimers();
    mockFetchToday.mockResolvedValue(TODAY);
    await act(async () => {
      renderPage();
    });
    expect(screen.getByText("Stand 12:00 Uhr")).toBeInTheDocument();

    mockFetchToday.mockResolvedValue({
      ...TODAY,
      schools: [TODAY.schools[0]!],
      total: {
        ...figures(120, 60, 2, 12, 3),
        asOf: new Date(TEST_CLOCK_INSTANT.getTime() + 60_000).toISOString(),
      },
    } satisfies TraegerSchoolsToday);
    mockFetchAlerts.mockRejectedValue(
      new ApiError("boom", 500, { code: "general.internal" }),
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    releaseFakeTimers();

    expect(mockFetchToday).toHaveBeenCalledTimes(2);
    expect(mockFetchAlerts).toHaveBeenCalledTimes(2);
    expect(screen.getByText("Stand 12:00 Uhr")).toBeInTheDocument();
    expect(screen.getByText("80")).toBeInTheDocument();
    expect(screen.getByText("6 von 17")).toBeInTheDocument();
    const table = screen.getByRole("table");
    expect(
      within(table).getByRole("link", { name: "OGS Lindenweg" }),
    ).toBeInTheDocument();
    expect(within(table).getAllByText(", erreicht Ihre Grenze")).toHaveLength(
      2,
    );
  });

  it("keeps the figures when the hints fail", async () => {
    mockFetchToday.mockResolvedValue(TODAY);
    mockFetchAlerts.mockRejectedValue(
      new ApiError("boom", 500, { code: "general.internal" }),
    );
    renderPage();

    expect(await screen.findByText("1 : 13,3")).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("table")).toBeInTheDocument();
  });
});

function renderAlerts(
  alerts: TraegerSchoolsAlerts | null,
  options: { error?: string | null; canChange?: boolean } = {},
) {
  mockFetchToday.mockResolvedValue(TODAY);
  mockFetchAlerts.mockResolvedValue(alerts);
  if (options.error) {
    mockFetchAlerts.mockRejectedValue(new Error(options.error));
  }
  mockUseSession.mockReturnValue(
    sessionWith([
      "traeger_overview:read",
      ...(options.canChange ? ["traeger_settings:manage"] : []),
    ]),
  );
  renderPage();
  return options.error
    ? screen.findByRole("alert")
    : alerts?.schools.length === 0
      ? screen.findAllByText("Ihrem Träger sind noch keine Schulen zugeordnet.")
      : screen.findByRole("link", {
          name: options.canChange ? "Grenzen ändern" : "Grenzen ansehen",
        });
}

describe("school alerts", () => {
  it("lists each school with its hints, the thresholds once and the way in", async () => {
    await renderAlerts(
      alertsOf([
        school("2", "OGS Am Bach", ["staff_shortage", "sick_children"]),
        school("3", "OGS Lindenweg", []),
        school("4", "OGS Mühle", ["open_enrollments"], 3),
      ]),
    );

    const card = screen.getByTestId("traeger-school-alerts");
    expect(within(card).getByText("2 Schulen")).toBeInTheDocument();
    expect(
      within(card).getAllByText(/^Ihre Grenzen: mehr als 20 Kinder/),
    ).toHaveLength(1);
    expect(within(card).getByText("Zu wenig Personal:")).toBeInTheDocument();
    expect(
      within(card).getByText(
        "30 Kinder auf 1 Betreuungskraft, Schlüssel 1 : 30.",
      ),
    ).toBeInTheDocument();
    expect(within(card).getByText("Viele Krankmeldungen:")).toBeInTheDocument();
    expect(
      within(card).getByText("3 Kinder warten auf eine Entscheidung."),
    ).toBeInTheDocument();
    expect(
      within(card).getByRole("link", { name: "Zur Schule: OGS Am Bach" }),
    ).toHaveAttribute("href", "/traeger/schulen/2");
    expect(
      within(card).queryByRole("link", { name: "Zur Schule: OGS Lindenweg" }),
    ).not.toBeInTheDocument();
    expect(
      within(card).getByRole("link", { name: "Grenzen ansehen" }),
    ).toHaveAttribute("href", "/traeger/einstellungen?bereich=dashboard");
  });

  it("names the link by what the person may do", async () => {
    await renderAlerts(
      alertsOf([school("2", "OGS Am Bach", ["sick_children"])]),
      {
        canChange: true,
      },
    );
    expect(
      screen.getByRole("link", { name: "Grenzen ändern" }),
    ).toBeInTheDocument();
  });

  it("filters the hints by school", async () => {
    await renderAlerts(
      alertsOf([
        school("2", "OGS Am Bach", ["staff_shortage"]),
        school("3", "OGS Lindenweg", []),
        school("4", "OGS Mühle", ["open_enrollments"], 3),
      ]),
    );
    const card = screen.getByTestId("traeger-school-alerts");

    fireEvent.click(
      within(card).getByRole("combobox", {
        name: "Auffälligkeiten nach Schule filtern",
      }),
    );
    fireEvent.click(screen.getByRole("option", { name: "OGS Mühle (1)" }));
    expect(
      within(card).getByRole("link", { name: "Zur Schule: OGS Mühle" }),
    ).toBeInTheDocument();
    expect(
      within(card).queryByRole("link", { name: "Zur Schule: OGS Am Bach" }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      within(card).getByRole("combobox", {
        name: "Auffälligkeiten nach Schule filtern",
      }),
    );
    fireEvent.click(screen.getByRole("option", { name: "OGS Lindenweg" }));
    expect(
      within(card).getByText("An dieser Schule ist gerade nichts auffällig."),
    ).toBeInTheDocument();
    expect(
      within(card).queryByRole("link", { name: /^Zur Schule:/ }),
    ).not.toBeInTheDocument();
  });

  it("says so when no school reaches a threshold", async () => {
    await renderAlerts(alertsOf([school("2", "OGS Am Bach", [])]));
    const card = screen.getByTestId("traeger-school-alerts");
    expect(
      within(card).getByText("Gerade ist an keiner Schule etwas auffällig."),
    ).toBeInTheDocument();
    expect(within(card).getByText("Auffälligkeiten")).toBeVisible();
    expect(within(card).queryByText(/^\d+ Schulen?$/)).not.toBeInTheDocument();
  });

  it("folds more than five schools", async () => {
    await renderAlerts(
      alertsOf(
        Array.from({ length: 7 }, (_, index) =>
          school(String(index + 10), `OGS ${index + 1}`, ["sick_children"]),
        ),
      ),
    );
    const card = screen.getByTestId("traeger-school-alerts");
    expect(
      within(card).getAllByRole("link", { name: /^Zur Schule:/ }),
    ).toHaveLength(5);
    fireEvent.click(
      within(card).getByRole("button", { name: /2 weitere anzeigen/ }),
    );
    expect(
      within(card).getAllByRole("link", { name: /^Zur Schule:/ }),
    ).toHaveLength(7);
  });

  it("shows the load error instead of an empty list", async () => {
    await renderAlerts(null, {
      error: "Die Auffälligkeiten der Schulen konnten nicht geladen werden.",
    });
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByText("Gerade ist an keiner Schule etwas auffällig."),
    ).not.toBeInTheDocument();
  });

  it("stays away while the Träger has no school", async () => {
    await renderAlerts(alertsOf([]));
    expect(screen.getByRole("heading", { name: "Dashboard" })).toBeVisible();
    expect(screen.getByText("Stand 12:00 Uhr")).toBeInTheDocument();
    expect(screen.getByText("0 von 0")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Gerade ist an keiner Schule eine Betreuungskraft in der Zeiterfassung angemeldet.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("traeger-school-alerts"),
    ).not.toBeInTheDocument();
  });
});
