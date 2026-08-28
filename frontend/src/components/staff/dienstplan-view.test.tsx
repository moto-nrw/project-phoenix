import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  useSession: vi.fn(),
  useSWRAuth: vi.fn(),
  isAdmin: vi.fn(),
  hasPermission: vi.fn(),
  useBerlinToday: vi.fn(),
  refreshPlanCaches: vi.fn().mockResolvedValue(undefined),
  // URL-State (d/view) wird über useSearchParams gelesen; der Stub liefert die
  // aktuell gesetzten Params. Ein leerer Wert bedeutet: keine Params -> Defaults.
  search: { value: "" },
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useSearchParams: () => new URLSearchParams(mocks.search.value),
}));

vi.mock("next-auth/react", () => ({
  useSession: mocks.useSession,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: mocks.replace }),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: mocks.isAdmin,
  hasPermission: mocks.hasPermission,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mocks.useSWRAuth,
  useTenantMutateMatching: () => mocks.refreshPlanCaches,
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: mocks.useBerlinToday,
}));

vi.mock("~/components/timetable/period-switcher-dropdown", () => ({
  PeriodSwitcherDropdown: ({
    periods,
    onSelect,
  }: {
    periods: Array<{
      startDate: string;
      endDate: string;
    }>;
    onSelect: (period: { startDate: string; endDate: string }) => void;
  }) =>
    periods[0] ? (
      <button type="button" onClick={() => onSelect(periods[0]!)}>
        Zeitraum wählen
      </button>
    ) : null,
}));

vi.mock("~/components/staff/dienstplan-resource-grid", () => ({
  DienstplanResourceGrid: ({
    weekDays,
    todayIso,
    assignmentsByStaff,
    closingDaysLoading,
    onCellClick,
  }: {
    weekDays: string[];
    todayIso: string;
    closingDaysLoading: boolean;
    assignmentsByStaff: Map<
      string,
      Map<string, Array<{ activityTitle: string }>>
    >;
    onCellClick: (
      staff: { id: string; firstName: string; lastName: string },
      date: string,
      shift: null,
    ) => void;
  }) => (
    <div data-testid="dienstplan-grid">
      <span data-testid="dienstplan-week-days">{weekDays.join(",")}</span>
      <span data-testid="dienstplan-today">{todayIso}</span>
      <span data-testid="closing-days-loading">
        {String(closingDaysLoading)}
      </span>
      <span data-testid="dienstplan-assignments">
        {[...assignmentsByStaff.values()]
          .flatMap((byDate) => [...byDate.values()])
          .flat()
          .map((assignment) => assignment.activityTitle)
          .join(",")}
      </span>
      <button
        type="button"
        onClick={() =>
          onCellClick(
            { id: "7", firstName: "Ada", lastName: "Lovelace" },
            "2026-07-06",
            null,
          )
        }
      >
        Schicht öffnen
      </button>
    </div>
  ),
}));

vi.mock("~/components/staff/dienstplan-halbjahr-grid", () => ({
  DienstplanHalbjahrGrid: ({
    dayISO,
    reducedPath,
    todayIso,
    onWeekClick,
  }: {
    dayISO: string;
    reducedPath: boolean;
    todayIso: string;
    onWeekClick: (mondayISO: string) => void;
  }) => (
    <div data-testid="halbjahr-grid">
      <span data-testid="halbjahr-day">{dayISO}</span>
      <span data-testid="halbjahr-today">{todayIso}</span>
      <span data-testid="halbjahr-reduced">{String(reducedPath)}</span>
      <button
        type="button"
        data-testid="halbjahr-week-click"
        onClick={() => onWeekClick("2026-09-14")}
      >
        open week
      </button>
    </div>
  ),
}));

vi.mock("~/components/staff/shift-edit-modal", () => ({
  ShiftEditModal: ({ onSaved }: { onSaved: () => void }) => (
    <button type="button" data-testid="shift-modal" onClick={onSaved}>
      Schicht speichern
    </button>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" />,
}));

import { DienstplanView } from "./dienstplan-view";

// One loaded staff member via the full overview path, so the grid renders and
// the URL-state assertions can read the derived week (weekDays testid).
function mockOverviewLoaded() {
  mocks.useSWRAuth.mockImplementation((key: string | null) => {
    if (key?.startsWith("dienstplan-overview-")) {
      return {
        data: {
          from: "",
          to: "",
          dienstplanInUse: true,
          staff: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
          shifts: [],
          assignments: [],
        },
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      };
    }
    return { data: [], error: undefined, isLoading: false, mutate: vi.fn() };
  });
}

describe("DienstplanView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useSession.mockReturnValue({
      data: { user: { id: "1", isAdmin: true, token: "token" } },
      status: "authenticated",
    });
    mocks.isAdmin.mockReturnValue(true);
    mocks.hasPermission.mockReturnValue(true);
    mocks.useBerlinToday.mockReturnValue("2026-07-05");
    mocks.search.value = "";
    window.history.replaceState(null, "", "/acme/dienstplan");
  });

  it("shows a blocking load error instead of the editable grid", () => {
    const mutateOverview = vi.fn();
    const mutateShiftTypes = vi.fn();
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key?.startsWith("dienstplan-overview-")) {
        return {
          data: undefined,
          error: new Error("server down"),
          isLoading: false,
          mutate: mutateOverview,
        };
      }
      if (key === "dienstplan-shift-types") {
        return {
          data: [],
          error: undefined,
          isLoading: false,
          mutate: mutateShiftTypes,
        };
      }
      return {
        data: [],
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      };
    });

    render(<DienstplanView />);

    expect(
      screen.getByText(
        /Der Dienstplan konnte nicht vollständig geladen werden/,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("dienstplan-grid")).not.toBeInTheDocument();

    // Retrying reloads the batched overview and the independent shift types.
    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));
    expect(mutateOverview).toHaveBeenCalledTimes(1);
    expect(mutateShiftTypes).not.toHaveBeenCalled();
  });

  it("anchors the current week and today marker to the Berlin calendar date", () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key?.startsWith("dienstplan-overview-")) {
        return {
          data: {
            from: "2026-07-06",
            to: "2026-07-10",
            dienstplanInUse: true,
            staff: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
            shifts: [],
            assignments: [
              {
                instanceId: "91",
                staffId: "7",
                date: "2026-07-06",
                startTime: "12:00",
                endTime: "13:00",
                activityTitle: "Mensa",
                roomId: "4",
                roomName: "Speiseraum",
                status: "planned",
                isAbsent: false,
                isSubstitute: false,
                absenceReason: null,
                coverageStatus: "covered",
                coverageReason: null,
                uncoveredIntervals: [],
              },
            ],
          },
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      return {
        data: [],
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      };
    });

    render(<DienstplanView />);

    expect(screen.getByTestId("dienstplan-week-days")).toHaveTextContent(
      "2026-07-06,2026-07-07,2026-07-08,2026-07-09,2026-07-10",
    );
    expect(screen.getByTestId("dienstplan-today")).toHaveTextContent(
      "2026-07-06",
    );
    expect(screen.getByTestId("dienstplan-assignments")).toHaveTextContent(
      "Mensa",
    );
    expect(mocks.useSWRAuth).toHaveBeenCalledWith(
      "dienstplan-overview-2026-07-06-2026-07-10",
      expect.any(Function),
    );
  });

  it("keeps shift creation gated while closing days are loading", () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key?.startsWith("dienstplan-overview-")) {
        return {
          data: {
            from: "2026-07-06",
            to: "2026-07-10",
            dienstplanInUse: true,
            staff: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
            shifts: [],
            assignments: [],
          },
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      if (key === "planning-closing-days") {
        return {
          data: undefined,
          error: undefined,
          isLoading: true,
          mutate: vi.fn(),
        };
      }
      return {
        data: [],
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      };
    });

    render(<DienstplanView />);

    expect(screen.getByTestId("closing-days-loading")).toHaveTextContent(
      "true",
    );
  });

  it("keeps the legacy shift-only path when schedules:read is missing", () => {
    const keys: Array<string | null> = [];
    mocks.hasPermission.mockImplementation(
      (_session: unknown, permission: string) =>
        permission !== "schedules:read",
    );
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      keys.push(key);
      if (key === "dienstplan-staff") {
        return {
          data: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      return {
        data: [],
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      };
    });

    render(<DienstplanView />);

    expect(screen.getByTestId("dienstplan-grid")).toBeInTheDocument();
    expect(keys).toContain("dienstplan-staff");
    expect(keys).toContain("dienstplan-shifts-2026-06-29-2026-07-03");
    expect(keys.some((key) => key?.startsWith("dienstplan-overview-"))).toBe(
      false,
    );
  });

  it("keeps the overview visible when only shift types fail", () => {
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key?.startsWith("dienstplan-overview-")) {
        return {
          data: {
            from: "2026-06-29",
            to: "2026-07-03",
            dienstplanInUse: true,
            dienstplanUsedWeeks: ["2026-06-29"],
            staff: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
            shifts: [],
            assignments: [],
          },
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      if (key === "dienstplan-shift-types") {
        return {
          data: undefined,
          error: new Error("types down"),
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      return {
        data: undefined,
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      };
    });

    render(<DienstplanView />);

    expect(screen.getByTestId("dienstplan-grid")).toBeInTheDocument();
    expect(
      screen.getByText(/Schichtarten konnten nicht geladen werden/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Schichtarten verwalten" }),
    ).toBeDisabled();
  });

  it("defaults to today's week and the Woche view without URL params", () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mockOverviewLoaded();

    render(<DienstplanView />);

    expect(screen.getByTestId("dienstplan-week-days")).toHaveTextContent(
      "2026-07-06,2026-07-07,2026-07-08,2026-07-09,2026-07-10",
    );
    expect(screen.getByTestId("dienstplan-grid")).toBeInTheDocument();
    // Woche ist aktiv: kein Halbjahres-Platzhalter. Der "Heute"-Button steht
    // in der laufenden Woche deaktiviert da, statt zu verschwinden (#2031).
    expect(
      screen.queryByText("Halbjahresansicht folgt."),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Heute" })).toBeDisabled();
  });

  it("invalidates all loaded plan caches after a shift is saved", () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mockOverviewLoaded();

    render(<DienstplanView />);
    fireEvent.click(screen.getByRole("button", { name: "Schicht öffnen" }));
    fireEvent.click(screen.getByRole("button", { name: "Schicht speichern" }));

    expect(mocks.refreshPlanCaches).toHaveBeenCalledTimes(1);
  });

  it("shows the KW of the week containing d for ?d=2026-09-14&view=woche", () => {
    const originalTZ = process.env.TZ;
    process.env.TZ = "Asia/Tokyo";
    mocks.search.value = "d=2026-09-14&view=woche";
    mockOverviewLoaded();

    try {
      render(<DienstplanView />);

      // 14.09.2026 ist ein Montag; die Woche ist KW 38. The label must retain
      // those calendar days even when the browser is east of Berlin. Das
      // Etikett kommt seit der Kopfzeilen-Vereinheitlichung aus
      // formatWeekLabel und lautet damit wie in Vertretung und Betreuungsplan
      // "KW 38 · 14.09.–18.09.2026"; die Zeitzonen-Aussage dieses Tests bleibt
      // unverändert.
      // Der Zeitraum steht seit der Kopfkarten-Umstellung zweimal: in der
      // Statuszeile der Kopfkarte und als Datum in der Zeitnavigation. Beide
      // müssen dieselbe Woche nennen.
      const labels = screen.getAllByText(/KW 38 ·/);
      expect(labels.length).toBeGreaterThanOrEqual(2);
      const label = labels[labels.length - 1]!;
      expect(label).toHaveTextContent(/^KW 38 · 14\.09\./);
      expect(label).toHaveTextContent(/–18\.09\./);
      expect(screen.getByTestId("dienstplan-week-days")).toHaveTextContent(
        "2026-09-14,2026-09-15,2026-09-16,2026-09-17,2026-09-18",
      );
      expect(
        mocks.useSWRAuth.mock.calls.some(
          (call) => call[0] === "dienstplan-overview-2026-09-14-2026-09-18",
        ),
      ).toBe(true);
    } finally {
      process.env.TZ = originalTZ;
    }
  });

  it("silently falls back to today and the Woche view for invalid d/view", () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mocks.search.value = "d=garbage&view=quatsch";
    mockOverviewLoaded();

    render(<DienstplanView />);

    expect(screen.getByTestId("dienstplan-week-days")).toHaveTextContent(
      "2026-07-06,2026-07-07,2026-07-08,2026-07-09,2026-07-10",
    );
    expect(screen.getByTestId("dienstplan-grid")).toBeInTheDocument();
    expect(
      screen.queryByText("Halbjahresansicht folgt."),
    ).not.toBeInTheDocument();
  });

  it("writes the Monday of the target week to d on week navigation", () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mockOverviewLoaded();

    render(<DienstplanView />);

    fireEvent.click(screen.getByRole("button", { name: "Nächste Woche" }));
    expect(new URLSearchParams(window.location.search).get("d")).toBe(
      "2026-07-13",
    );

    fireEvent.click(screen.getByRole("button", { name: "Vorherige Woche" }));
    expect(new URLSearchParams(window.location.search).get("d")).toBe(
      "2026-06-29",
    );
  });

  it.each(["2026-08-01", "2026-08-02"])(
    "snaps a period starting on %s to its first school day",
    async (startDate) => {
      mocks.useBerlinToday.mockReturnValue("2026-07-06");
      mocks.useSWRAuth.mockImplementation((key: string | null) => {
        if (key === "database-calendar-periods-list") {
          return {
            data: [{ startDate, endDate: "2026-08-07" }],
            error: undefined,
            isLoading: false,
            mutate: vi.fn(),
          };
        }
        if (key?.startsWith("dienstplan-overview-")) {
          return {
            data: {
              from: "",
              to: "",
              dienstplanInUse: true,
              staff: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
              shifts: [],
              assignments: [],
            },
            error: undefined,
            isLoading: false,
            mutate: vi.fn(),
          };
        }
        return {
          data: null,
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      });

      render(<DienstplanView />);
      fireEvent.click(screen.getByRole("button", { name: "Zeitraum wählen" }));

      await waitFor(() =>
        expect(new URLSearchParams(window.location.search).get("d")).toBe(
          "2026-08-03",
        ),
      );
    },
  );

  it("enables the Heute button only when the visible week differs from today's", () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mockOverviewLoaded();

    // Aktuelle Woche -> Heute ist wirkungslos und deshalb deaktiviert.
    const { unmount } = render(<DienstplanView />);
    expect(screen.getByRole("button", { name: "Heute" })).toBeDisabled();
    unmount();

    // Andere Woche -> Heute wird bedienbar.
    mocks.search.value = "d=2026-09-14";
    render(<DienstplanView />);
    expect(screen.getByRole("button", { name: "Heute" })).toBeEnabled();
  });

  it("renders the Halbjahr grid for view=halbjahr and writes the param on switch", async () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mockOverviewLoaded();

    // Render-Seite: Deep-Link auf Halbjahr zeigt das Halbjahres-Grid statt des
    // Wochenrasters; der Anker-Tag und heute werden durchgereicht.
    mocks.search.value = "view=halbjahr";
    const { unmount } = render(<DienstplanView />);
    expect(screen.getByTestId("halbjahr-grid")).toBeInTheDocument();
    expect(screen.queryByTestId("dienstplan-grid")).not.toBeInTheDocument();
    expect(screen.getByTestId("halbjahr-day")).toHaveTextContent("2026-07-06");
    expect(screen.getByTestId("halbjahr-today")).toHaveTextContent(
      "2026-07-06",
    );
    unmount();

    // Schreib-Seite: aus der Wochenansicht schreibt der Umschalter view=halbjahr.
    mocks.search.value = "";
    window.history.replaceState(null, "", "/acme/dienstplan");
    render(<DienstplanView />);
    fireEvent.click(screen.getByRole("button", { name: "Halbjahr" }));
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get("view")).toBe(
        "halbjahr",
      ),
    );
  });

  it("writes the Monday to d and clears view when a half-year cell jumps into a week", async () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mocks.search.value = "view=halbjahr";
    window.history.replaceState(null, "", "/acme/dienstplan?view=halbjahr");
    mockOverviewLoaded();

    render(<DienstplanView />);
    fireEvent.click(screen.getByTestId("halbjahr-week-click"));

    await waitFor(() => {
      const params = new URLSearchParams(window.location.search);
      expect(params.get("d")).toBe("2026-09-14");
      expect(params.get("view")).toBeNull();
    });
  });

  it("hides the Halbjahr tab and falls back to Woche without schedules:read", () => {
    // Die Halbjahres-Sicht zeigt den Soll-/Plan-Abgleich aus /overview, der
    // schedules:read verlangt. Ohne die Berechtigung greift zugleich der
    // reduzierte Datenpfad, daher der Legacy-Stub (dienstplan-staff).
    mocks.hasPermission.mockImplementation(
      (_session: unknown, permission: string) =>
        permission !== "schedules:read",
    );
    mocks.search.value = "view=halbjahr";
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key === "dienstplan-staff") {
        return {
          data: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      return { data: [], error: undefined, isLoading: false, mutate: vi.fn() };
    });

    render(<DienstplanView />);

    // Deep-Link fällt still auf die Wochenansicht zurück; der Umschalter
    // entfällt komplett (ein Ein-Tab-Umschalter wäre sinnlos).
    expect(screen.queryByTestId("halbjahr-grid")).not.toBeInTheDocument();
    expect(screen.getByTestId("dienstplan-grid")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Halbjahr" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Woche" }),
    ).not.toBeInTheDocument();
  });

  it("shows the Halbjahr tab with schedules:read", () => {
    mockOverviewLoaded();

    render(<DienstplanView />);

    expect(
      screen.getByRole("button", { name: "Halbjahr" }),
    ).toBeInTheDocument();
  });

  it("hides the Halbjahr tab without time_tracking:manage", () => {
    mocks.hasPermission.mockImplementation(
      (_session: unknown, permission: string) =>
        permission !== "time_tracking:manage",
    );
    mocks.search.value = "view=halbjahr";
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key === "dienstplan-staff") {
        return {
          data: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      return { data: [], error: undefined, isLoading: false, mutate: vi.fn() };
    });

    render(<DienstplanView />);

    expect(screen.queryByTestId("halbjahr-grid")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Halbjahr" }),
    ).not.toBeInTheDocument();
  });

  it("shows the export action with the full permission triple", () => {
    mockOverviewLoaded();

    render(<DienstplanView />);

    // Die Aktion liegt im Menü neben dem Titel, nicht als zweiter Knopf
    // daneben: neben dem Titel steht eine sichtbare Aktion.
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    expect(screen.getByText("Drucken oder exportieren")).toBeInTheDocument();
  });

  it("hides export without the permission to read staff names", () => {
    // POST /api/staff-shifts/export verlangt users:read; ohne die Berechtigung
    // liefe der Dialog in ein 403.
    mocks.hasPermission.mockImplementation(
      (_session: unknown, permission: string) => permission !== "users:read",
    );
    mockOverviewLoaded();

    render(<DienstplanView />);

    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
  });

  it("renders the disabled state when timetable.enabled is false", () => {
    // Route-Gate wie Betreuungsplan/Vertretung: Direktaufruf bei
    // ausgeschaltetem Planungsbereich zeigt den Hinweis statt des Rasters.
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key === "settings-schema") {
        return {
          data: {
            tabs: [
              {
                categories: [
                  { items: [{ key: "timetable.enabled", value: false }] },
                ],
              },
            ],
          },
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      return { data: [], error: undefined, isLoading: false, mutate: vi.fn() };
    });

    render(<DienstplanView />);

    expect(screen.getByTestId("dienstplan-disabled-state")).toBeInTheDocument();
    expect(screen.getByText("Dienstplan ist deaktiviert")).toBeInTheDocument();
    expect(screen.queryByTestId("dienstplan-grid")).not.toBeInTheDocument();
  });

  it("renders normally while the settings schema is unreadable or enabled", () => {
    // fetchSettingsSchema liefert null ohne Leserecht — die Seite rendert
    // normal (Graceful Default wie Sidebar und Betreuungsplan).
    mockOverviewLoaded();

    render(<DienstplanView />);

    expect(
      screen.queryByTestId("dienstplan-disabled-state"),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("dienstplan-grid")).toBeInTheDocument();
  });

  it("does not request calendar periods for non-admins", () => {
    // Der Periods-Key ist auf canEdit gegated: bei Direktaufruf durch
    // Nicht-Admins darf vor dem Redirect kein schedules:read-Request feuern.
    mocks.isAdmin.mockReturnValue(false);
    mockOverviewLoaded();

    render(<DienstplanView />);

    expect(
      mocks.useSWRAuth.mock.calls.some(
        (call) => call[0] === "database-calendar-periods-list",
      ),
    ).toBe(false);
  });

  it("shows the blocking load error in the Halbjahr view when the schedule fails", () => {
    // Regression K1: the half-year branch used to only check for empty staff, so
    // a failed staff/overview load rendered "keine Mitarbeitenden" instead of the
    // error card. The error state now precedes the view split for both views.
    mocks.search.value = "view=halbjahr";
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key?.startsWith("dienstplan-overview-")) {
        return {
          data: undefined,
          error: new Error("server down"),
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      return { data: [], error: undefined, isLoading: false, mutate: vi.fn() };
    });

    render(<DienstplanView />);

    expect(
      screen.getByText(
        /Der Dienstplan konnte nicht vollständig geladen werden/,
      ),
    ).toBeInTheDocument();
    // Neither the half-year grid nor the empty-staff hint is shown.
    expect(screen.queryByTestId("halbjahr-grid")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine Mitarbeitenden angelegt"),
    ).not.toBeInTheDocument();
  });
});
