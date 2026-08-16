import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { EnrichedInstance } from "~/lib/timetable-types";

const {
  mockSearch,
  mockUseSession,
  mockToastSuccess,
  mockToastError,
  mockTenantMutate,
  mockUseSWRAuth,
  mockApplyDeviations,
  mockDayListProps,
  mockGridProps,
  mockEditorProps,
} = vi.hoisted(() => ({
  mockSearch: { value: "" },
  mockUseSession: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockTenantMutate: vi.fn(),
  mockUseSWRAuth: vi.fn(),
  mockApplyDeviations: vi.fn(),
  mockDayListProps: vi.fn(),
  mockGridProps: vi.fn(),
  mockEditorProps: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(mockSearch.value),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    warning: vi.fn(),
  }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mockUseSWRAuth,
  useTenantMutate: () => mockTenantMutate,
  // Vom eingebetteten BulkSubstitutionModal gebraucht (Vorschau-Cache-Reset);
  // dessen Verhalten testet bulk-substitution-modal.test.tsx.
  useTenantMutateMatching: () => () => Promise.resolve(),
}));

vi.mock("~/lib/hooks/use-timetable-day-hours", () => ({
  useTimetableDayHours: () => ({ dayStartHour: 9, dayEndHour: 17 }),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getWeek: vi.fn(),
    getGaps: vi.fn(),
    applyDeviations: mockApplyDeviations,
  },
}));

// Nur den Netzwerk-Fetcher mocken; getSettingValue & Co. bleiben die echten
// Implementierungen, damit die Tests nicht gegen eine driftende Kopie laufen.
vi.mock("~/lib/settings-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/settings-api")>()),
  fetchSettingsSchema: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: {
    getAllStaff: vi.fn(),
  },
}));

vi.mock("~/lib/shift-api", () => ({
  staffShiftService: {
    getOverview: vi.fn(),
  },
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/acme${path}`,
}));

// Nur die Tagesliste selbst ersetzen. Die übrigen Exporte (Klassifikation und
// Zeilendarstellung) bleiben echt, weil die Wochenliste sie importiert — ein
// Voll-Mock des Moduls würde sie beim Rendern der Wochenansicht verschlucken.
vi.mock(
  "~/components/timetable/vertretung-day-list",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("~/components/timetable/vertretung-day-list")
    >()),
    VertretungDayList: (props: {
      instances: Array<{ id: string }>;
      gapsAvailable: boolean;
      mode: string;
      canManage: boolean;
      onEdit: (id: string) => void;
    }) => {
      mockDayListProps(props);
      return (
        <div data-testid="day-list" data-mode={props.mode}>
          <button type="button" onClick={() => props.onEdit("42")}>
            day-list-edit
          </button>
        </div>
      );
    },
  }),
);

vi.mock("~/components/timetable/weekly-calendar-grid", () => ({
  WeeklyCalendarGrid: (props: {
    instances: Array<{ id: string }>;
    selectedId: string | null;
    onInstanceClick: (instance: { id: string }) => void;
  }) => {
    mockGridProps(props);
    return (
      <div data-testid="calendar-grid">
        <button
          type="button"
          onClick={() => props.onInstanceClick({ id: "42" })}
        >
          grid-click
        </button>
      </div>
    );
  },
}));

vi.mock("~/components/timetable/substitution-slide-over", () => ({
  SubstitutionSlideOver: (props: {
    instance: { id: string } | null;
    initialTab: string;
    staffLoadError?: boolean;
    canManage: boolean;
    dayAbsentStaffIds: ReadonlySet<string>;
    onClose: () => void;
    onApply: (input: unknown) => Promise<boolean>;
    onTabChange: (tab: "bearbeiten" | "verlauf") => void;
  }) => {
    mockEditorProps(props);
    if (!props.instance) return null;
    return (
      <div data-testid="editor" data-initial-tab={props.initialTab}>
        <span>editor-instance-{props.instance.id}</span>
        <button type="button" onClick={() => props.onClose()}>
          editor-close
        </button>
        <button type="button" onClick={() => props.onTabChange("verlauf")}>
          editor-history
        </button>
        <button
          type="button"
          onClick={() => void props.onApply({ cancel: true })}
        >
          editor-cancel-save
        </button>
      </div>
    );
  },
}));

import { VertretungView } from "./vertretung-view";

function makeInstance(overrides: Partial<EnrichedInstance>): EnrichedInstance {
  return {
    id: "42",
    date: "2026-07-15",
    startTime: "12:00",
    endTime: "13:00",
    title: "Mensa",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "care",
    roomId: "3",
    roomName: "Mensa",
    staff: [],
    studentIds: [],
    students: [],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 0,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    conflictWarnings: [],
    ...overrides,
  };
}

const WEEK_INSTANCES: EnrichedInstance[] = [
  makeInstance({ id: "42", date: "2026-07-15", title: "Mensa" }),
  makeInstance({ id: "43", date: "2026-07-16", title: "Fussball" }),
];

interface SwrState {
  weekData?: { from: string; to: string; instances: EnrichedInstance[] };
  weekError?: Error;
  weekLoading?: boolean;
  gapsData?: {
    from: string;
    to: string;
    gaps: unknown[];
    acknowledged: unknown[];
  };
  gapsError?: Error;
  gapsLoading?: boolean;
  staffData?: Array<{ id: string; name: string }>;
  staffError?: Error;
  settingsSchema?: unknown;
  settingsLoading?: boolean;
  /** Dienstplan-Übersicht für den Abdeckungshinweis; ohne sie kein Hinweis. */
  coverageData?: {
    dienstplanInUse: boolean;
    assignments: Array<Record<string, unknown>>;
  };
  coverageError?: Error;
}

function setupSWR(state: SwrState = {}) {
  const {
    weekData = {
      from: "2026-07-13",
      to: "2026-07-19",
      instances: WEEK_INSTANCES,
    },
    weekError,
    weekLoading = false,
    gapsData = {
      from: "2026-07-15",
      to: "2026-07-19",
      gaps: [],
      acknowledged: [],
    },
    gapsError,
    gapsLoading = false,
    staffData = [{ id: "11", name: "Ada Staff" }],
    staffError,
    settingsSchema = null,
    settingsLoading = false,
    coverageData,
    coverageError,
  } = state;

  mockUseSWRAuth.mockImplementation((key: string | null) => {
    if (key === null) return {};
    if (key === "settings-schema") {
      return settingsLoading
        ? { isLoading: true }
        : { data: settingsSchema, isLoading: false };
    }
    if (key === "vertretung-staff-list") {
      return { data: staffData, error: staffError };
    }
    if (key.startsWith("vertretung-gaps")) {
      if (gapsLoading) return { isLoading: true };
      return { data: gapsData, error: gapsError, isLoading: gapsLoading };
    }
    if (key.startsWith("vertretung-week")) {
      return {
        data: weekError ? undefined : weekData,
        error: weekError,
        isLoading: weekLoading,
      };
    }
    if (key.startsWith("dienstplan-overview")) {
      return { data: coverageData, error: coverageError };
    }
    return {};
  });
}

function urlKeys(): string[] {
  return [...new URLSearchParams(window.location.search).keys()];
}

describe("VertretungView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Mittwoch 2026-07-15, Berlin (Sommerzeit UTC+2). Woche Mo 13. – So 19.07.
    vi.setSystemTime(new Date("2026-07-15T12:00:00Z"));
    mockSearch.value = "";
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:manage"] } },
    });
    mockTenantMutate.mockResolvedValue(undefined);
    mockApplyDeviations.mockResolvedValue({
      instanceId: "42",
      cancelled: true,
      understaffedAck: false,
      affectedInstances: [],
      warnings: [],
    });
    setupSWR();
    window.history.replaceState(null, "", "/acme/vertretung");
  });

  it("defaults to today's Berlin date when d is absent", () => {
    render(<VertretungView />);

    const props = mockDayListProps.mock.calls.at(-1)?.[0];
    expect(props.instances.map((i: EnrichedInstance) => i.id)).toEqual(["42"]);
    expect(props.instances[0].date).toBe("2026-07-15");
    // Der "Heute"-Button bleibt sichtbar und ist deaktiviert, solange d der
    // heutige Tag ist (#2031: feste Geometrie der Navigationsgruppe).
    expect(screen.getByRole("button", { name: "Heute" })).toBeDisabled();
  });

  it("renders exactly the five Mo–Fr chips, never Sa/So", () => {
    render(<VertretungView />);

    for (const label of [
      "Mo 13.07.",
      "Di 14.07.",
      "Mi 15.07.",
      "Do 16.07.",
      "Fr 17.07.",
    ]) {
      expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
    }
    expect(
      screen.queryByRole("button", { name: "Sa 18.07." }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "So 19.07." }),
    ).not.toBeInTheDocument();
  });

  it("snaps a weekend deep link to the following Monday", () => {
    mockSearch.value = "d=2026-07-18"; // Samstag
    render(<VertretungView />);

    // Angezeigt wird Montag der Folgewoche, dessen Chip ist selektiert.
    expect(screen.getByRole("button", { name: "Mo 20.07." })).toHaveClass(
      "bg-gray-900",
    );
    const props = mockDayListProps.mock.calls.at(-1)?.[0];
    expect(props.instances).toEqual([]);
  });

  it("defaults to next Monday on a weekend and disables the Heute button", () => {
    // Samstag 2026-07-18, Berlin (Sommerzeit UTC+2).
    vi.setSystemTime(new Date("2026-07-18T12:00:00Z"));
    render(<VertretungView />);

    expect(screen.getByRole("button", { name: "Mo 20.07." })).toHaveClass(
      "bg-gray-900",
    );
    // dayISO == Heute-Ziel (nächster Montag) -> Heute ist wirkungslos und
    // deshalb deaktiviert, verschwindet aber nicht (#2031).
    expect(screen.getByRole("button", { name: "Heute" })).toBeDisabled();
  });

  it("navigates to next Monday when Heute is clicked on a weekend", () => {
    vi.setSystemTime(new Date("2026-07-18T12:00:00Z")); // Samstag
    mockSearch.value = "d=2026-07-15";
    render(<VertretungView />);

    fireEvent.click(screen.getByRole("button", { name: "Heute" }));
    expect(new URLSearchParams(window.location.search).get("d")).toBe(
      "2026-07-20",
    );
  });

  it("respects d, opens the editor for block, and starts on the Verlauf tab for verlauf=1", () => {
    mockSearch.value = "d=2026-07-16&block=43&verlauf=1";
    render(<VertretungView />);

    const dayList = mockDayListProps.mock.calls.at(-1)?.[0];
    expect(dayList.instances.map((i: EnrichedInstance) => i.id)).toEqual([
      "43",
    ]);

    const editor = mockEditorProps.mock.calls.at(-1)?.[0];
    expect(editor.instance?.id).toBe("43");
    expect(editor.initialTab).toBe("verlauf");
    expect(screen.getByTestId("editor")).toHaveAttribute(
      "data-initial-tab",
      "verlauf",
    );
  });

  it("falls back to today when d is not a real calendar date", () => {
    // "foo" ergäbe ohne Guard NaN-NaN-NaN-Fetchfenster, "2026-02-31" würde
    // still zum 3. März überlaufen — beide müssen auf heute zurückfallen.
    for (const bad of ["foo", "2026-02-31"]) {
      mockSearch.value = `d=${bad}`;
      const { unmount } = render(<VertretungView />);

      const props = mockDayListProps.mock.calls.at(-1)?.[0];
      expect(props.instances.map((i: EnrichedInstance) => i.id)).toEqual([
        "42",
      ]);
      expect(props.instances[0].date).toBe("2026-07-15");
      unmount();
    }
  });

  it("strips unknown query params on the first URL write", () => {
    mockSearch.value = "d=2026-07-15&utm_source=x";
    window.history.replaceState(
      null,
      "",
      "/acme/vertretung?d=2026-07-15&utm_source=x",
    );
    render(<VertretungView />);

    fireEvent.click(screen.getByRole("button", { name: "Do 16.07." }));
    expect(urlKeys()).toEqual(["d"]);
    expect(new URLSearchParams(window.location.search).get("d")).toBe(
      "2026-07-16",
    );
  });

  it("keeps the URL to at most d/block/verlauf across interactions", () => {
    mockSearch.value = "d=2026-07-15&block=42";
    window.history.replaceState(
      null,
      "",
      "/acme/vertretung?d=2026-07-15&block=42",
    );
    render(<VertretungView />);

    const allowed = ["d", "block", "verlauf"];
    const expectAllowed = () => {
      for (const key of urlKeys()) expect(allowed).toContain(key);
    };

    // Tageswechsel über einen Wochenleisten-Chip: setzt d, entfernt block+verlauf.
    fireEvent.click(screen.getByRole("button", { name: "Do 16.07." }));
    expect(urlKeys()).toEqual(["d"]);
    expect(new URLSearchParams(window.location.search).get("d")).toBe(
      "2026-07-16",
    );

    // Editor öffnen aus der Liste: setzt block.
    fireEvent.click(screen.getByText("day-list-edit"));
    expectAllowed();
    expect(new URLSearchParams(window.location.search).get("block")).toBe("42");

    // Reiterwechsel auf Verlauf: setzt verlauf=1.
    fireEvent.click(screen.getByText("editor-history"));
    expect(urlKeys().sort()).toEqual(["block", "d", "verlauf"]);

    // Editor schließen: entfernt block UND verlauf.
    fireEvent.click(screen.getByText("editor-close"));
    expect(urlKeys()).toEqual(["d"]);
  });

  it("räumt verlauf ab, wenn ein Block per Klick geöffnet wird", () => {
    mockSearch.value = "d=2026-07-15&block=42";
    window.history.replaceState(
      null,
      "",
      "/acme/vertretung?d=2026-07-15&block=42",
    );
    render(<VertretungView />);

    fireEvent.click(screen.getByText("editor-history"));
    expect(new URLSearchParams(window.location.search).get("verlauf")).toBe(
      "1",
    );

    // Block-Klick bei offenem Verlaufs-Reiter: verlauf darf nicht in der URL
    // kleben, sonst startet der nächste geöffnete Block im Verlauf statt im
    // Bearbeiten-Formular (Muster der alten handleSelectInstance).
    fireEvent.click(screen.getByText("grid-click"));
    expect(new URLSearchParams(window.location.search).get("block")).toBe("42");
    expect(new URLSearchParams(window.location.search).has("verlauf")).toBe(
      false,
    );
  });

  it("shows placeholders (never a fabricated 0) for a fully past day", () => {
    mockSearch.value = "d=2026-06-15"; // Woche komplett in der Vergangenheit
    render(<VertretungView />);

    expect(screen.getByText("Offen: –")).toBeInTheDocument();
    expect(screen.getByText("Quittiert: –")).toBeInTheDocument();
    expect(screen.queryByText("Offen: 0")).not.toBeInTheDocument();
    expect(screen.queryByText("Quittiert: 0")).not.toBeInTheDocument();
    expect(mockDayListProps.mock.calls.at(-1)?.[0].gapsAvailable).toBe(false);
  });

  it("passes unavailable gaps to the list while the gaps request is still loading", () => {
    setupSWR({ gapsLoading: true });
    render(<VertretungView />);

    expect(mockDayListProps.mock.calls.at(-1)?.[0].gapsAvailable).toBe(false);
  });

  it("passes unavailable gaps to the list when the gaps request fails", () => {
    setupSWR({ gapsError: new Error("gaps unavailable") });
    render(<VertretungView />);

    expect(mockDayListProps.mock.calls.at(-1)?.[0].gapsAvailable).toBe(false);
  });

  it("renders an error surface (not an empty plan) when the week fails to load", () => {
    setupSWR({ weekError: new Error("boom") });
    render(<VertretungView />);

    expect(screen.getByTestId("vertretung-week-error")).toBeVisible();
    expect(
      screen.getByText("Vertretung konnte nicht geladen werden"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Erneut versuchen" }),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("day-list")).not.toBeInTheDocument();
    expect(screen.queryByTestId("calendar-grid")).not.toBeInTheDocument();
  });

  it("retries week, gaps, AND staff list from the error surface", () => {
    // Ein Backend-Blip lässt typischerweise alle drei Abrufe gleichzeitig
    // scheitern; ein Retry nur des Wochen-Keys ließe Gaps-Chips und
    // Personal-Alert bis zum Reload stale.
    setupSWR({ weekError: new Error("boom") });
    render(<VertretungView />);

    fireEvent.click(screen.getByRole("button", { name: "Erneut versuchen" }));
    expect(mockTenantMutate.mock.calls.map((c) => c[0] as string)).toEqual([
      "vertretung-week-2026-07-13-2026-07-19",
      "vertretung-gaps-2026-07-15-2026-07-19",
      "vertretung-staff-list",
    ]);
  });

  it("defaults the list filter to 'Nur Störungen' and toggles to 'Ganzer Tag'", async () => {
    render(<VertretungView />);

    expect(mockDayListProps.mock.calls.at(-1)?.[0].mode).toBe("stoerungen");

    // Der Umschalter sitzt seit #2031 in der Liste, die er filtert (die hier
    // gemockt ist). Getestet wird deshalb die Verdrahtung: der Callback der
    // Liste setzt den Modus, den sie beim nächsten Rendern zurückbekommt. Die
    // Bedienung des Umschalters selbst prüft vertretung-list-filter.test.tsx.
    act(() => {
      mockDayListProps.mock.calls.at(-1)?.[0].onModeChange("ganzer-tag");
    });

    await waitFor(() =>
      expect(mockDayListProps.mock.calls.at(-1)?.[0].mode).toBe("ganzer-tag"),
    );
  });

  it("applies a cancel save in one call and clears block from the URL", async () => {
    mockSearch.value = "d=2026-07-15&block=42&verlauf=1";
    window.history.replaceState(
      null,
      "",
      "/acme/vertretung?d=2026-07-15&block=42&verlauf=1",
    );
    render(<VertretungView />);

    fireEvent.click(screen.getByText("editor-cancel-save"));

    await waitFor(() =>
      expect(mockApplyDeviations).toHaveBeenCalledWith("42", { cancel: true }),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith("Block abgesagt");
    // Cache-Refresh nach dem committeten Save.
    await waitFor(() => expect(mockTenantMutate).toHaveBeenCalled());
    // block und verlauf sind aus der URL entfernt.
    expect(urlKeys()).toEqual(["d"]);
  });

  // --- Wochenansicht (#2030) -------------------------------------------
  // Die Tagesansicht ist der Standard; die Woche ist eine zusätzliche Sicht
  // auf dieselben bereits geladenen Daten und lebt in `view=woche`.

  it("startet in der Tagesansicht und gibt dem Raster genau einen Tag", () => {
    render(<VertretungView />);

    expect(screen.getByRole("tab", { name: "Tag" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(mockGridProps.mock.calls.at(-1)?.[0].weekDays).toHaveLength(1);
    expect(screen.getByTestId("day-list")).toBeInTheDocument();
    expect(
      screen.queryByTestId("vertretung-week-list"),
    ).not.toBeInTheDocument();
    expect(urlKeys()).toEqual([]);
  });

  it("schaltet über die Kontextleiste auf die Woche und schreibt view=woche in die URL", async () => {
    render(<VertretungView />);

    // Radix-Tabs aktivieren per mousedown, nicht click.
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Woche" }), {
      button: 0,
    });

    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get("view")).toBe(
        "woche",
      ),
    );
  });

  it("stellt die Wochenansicht aus einem Deep-Link wieder her", () => {
    mockSearch.value = "view=woche";
    render(<VertretungView />);

    expect(screen.getByRole("tab", { name: "Woche" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByTestId("vertretung-week-list")).toBeInTheDocument();
    expect(screen.queryByTestId("day-list")).not.toBeInTheDocument();
    // Statt der Tagesleiste steht das Wochenlabel zwischen den Pfeilen — über
    // Mo–Fr, nicht über das Mo–So-Fetch-Fenster.
    expect(screen.getByText(/13\.07\.–17\.07\.2026/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Mi 15.07." }),
    ).not.toBeInTheDocument();
  });

  it("zeigt in der Wochenansicht alle fünf Schultage mit ihren Terminen", () => {
    mockSearch.value = "view=woche";
    render(<VertretungView />);

    const grid = mockGridProps.mock.calls.at(-1)?.[0];
    expect(grid.weekDays).toHaveLength(5);
    // Mi und Do liegen in derselben Woche — beide Termine sind im Raster.
    expect(grid.instances.map((i: EnrichedInstance) => i.id)).toEqual([
      "42",
      "43",
    ]);
    for (const iso of [
      "2026-07-13",
      "2026-07-14",
      "2026-07-15",
      "2026-07-16",
      "2026-07-17",
    ]) {
      expect(
        screen.getByTestId(`vertretung-week-list-day-${iso}`),
      ).toBeInTheDocument();
    }
  });

  it("markiert in der Wochenansicht die Lücken aller Tage und zählt sie über die Woche", () => {
    setupSWR({
      gapsData: {
        from: "2026-07-15",
        to: "2026-07-19",
        gaps: [
          { instanceId: "42", date: "2026-07-15" },
          { instanceId: "43", date: "2026-07-16" },
        ],
        acknowledged: [],
      },
    });
    mockSearch.value = "view=woche";
    render(<VertretungView />);

    const grid = mockGridProps.mock.calls.at(-1)?.[0];
    expect([...(grid.gapInstanceIds as Set<string>)].sort()).toEqual([
      "42",
      "43",
    ]);
    // Der Zähler der Kontextleiste zählt in der Wochenansicht die Woche.
    expect(screen.getByText("Offen: 2")).toBeInTheDocument();
  });

  it("zeigt am Wochenende keine erfundenen Wochenzähler für ein leeres sichtbares Lückenfenster", () => {
    // Für die laufende Woche lädt der Endpunkt ab Samstag. Die Wochenansicht
    // zeigt aber nur Mo–Fr; ein erfolgreicher Request enthält damit keine
    // sichtbaren Tage und darf nicht als bestätigte Null gewertet werden.
    vi.setSystemTime(new Date("2026-07-18T12:00:00Z")); // Samstag
    setupSWR({
      gapsData: {
        from: "2026-07-18",
        to: "2026-07-19",
        gaps: [],
        acknowledged: [],
      },
    });
    mockSearch.value = "d=2026-07-17&view=woche";
    render(<VertretungView />);

    expect(screen.getByText("Offen: –")).toBeInTheDocument();
    expect(screen.getByText("Quittiert: –")).toBeInTheDocument();
    expect(screen.queryByText("Offen: 0")).not.toBeInTheDocument();
    expect(screen.queryByText("Quittiert: 0")).not.toBeInTheDocument();
  });

  it("öffnet aus der Wochenliste denselben Editor wie die Tagesansicht", () => {
    // Der Donnerstags-Termin muss gestört sein, sonst steht er im Modus
    // "Nur Störungen" nicht in der Liste.
    setupSWR({
      gapsData: {
        from: "2026-07-15",
        to: "2026-07-19",
        gaps: [{ instanceId: "43", date: "2026-07-16" }],
        acknowledged: [],
      },
    });
    mockSearch.value = "view=woche";
    window.history.replaceState(null, "", "/acme/vertretung?view=woche");
    const { unmount } = render(<VertretungView />);

    const thursday = screen.getByTestId("vertretung-week-list-day-2026-07-16");
    fireEvent.click(
      within(thursday).getByRole("button", { name: "Bearbeiten" }),
    );

    expect(new URLSearchParams(window.location.search).get("block")).toBe("43");
    // Die Ansicht bleibt beim Öffnen des Editors erhalten.
    expect(new URLSearchParams(window.location.search).get("view")).toBe(
      "woche",
    );

    // Derselbe Editor wie in der Tagesansicht — der Klick schreibt nur `block`,
    // gerendert wird daraus (der Suchparam-Mock re-rendert nicht von selbst).
    unmount();
    mockSearch.value = "view=woche&block=43";
    render(<VertretungView />);
    expect(mockEditorProps.mock.calls.at(-1)?.[0].instance?.id).toBe("43");
    expect(screen.getByTestId("editor")).toBeInTheDocument();
  });

  it("springt per Tageskopfzeile aus der Woche in die Tagesansicht", () => {
    mockSearch.value = "view=woche";
    window.history.replaceState(null, "", "/acme/vertretung?view=woche");
    render(<VertretungView />);

    fireEvent.click(
      screen.getByRole("button", { name: "Do 16.07. als Tagesansicht öffnen" }),
    );

    const params = new URLSearchParams(window.location.search);
    expect(params.get("d")).toBe("2026-07-16");
    expect(params.has("view")).toBe(false);
  });

  it("hält die URL auch mit view auf dem erlaubten Vokabular", () => {
    mockSearch.value = "view=woche&block=42&utm_source=x";
    window.history.replaceState(
      null,
      "",
      "/acme/vertretung?view=woche&block=42&utm_source=x",
    );
    render(<VertretungView />);

    fireEvent.click(screen.getByText("editor-history"));
    expect(urlKeys().sort()).toEqual(["block", "verlauf", "view"]);
  });

  it("renders the disabled state when timetable.enabled is false", () => {
    setupSWR({
      settingsSchema: {
        tabs: [
          {
            id: "operations",
            categories: [
              {
                id: "timetable",
                items: [{ key: "timetable.enabled", value: false }],
              },
            ],
          },
        ],
      },
    });
    render(<VertretungView />);

    expect(screen.getByTestId("vertretung-disabled-state")).toBeVisible();
    expect(screen.queryByTestId("day-list")).not.toBeInTheDocument();
  });

  // Einsätze ohne Dienstplan-Abdeckung sind KEINE Störung (die Definition
  // bleibt vierteilig), sondern ein Planungshinweis über der Liste.
  describe("Dienstplan-Abdeckungshinweis", () => {
    function coverageAssignment(overrides: Record<string, unknown> = {}) {
      return {
        instanceId: "42",
        staffId: "11",
        date: "2026-07-15",
        startTime: "12:00",
        endTime: "14:00",
        activityTitle: "Mensa",
        status: "planned",
        isAbsent: false,
        isSubstitute: false,
        coverageStatus: "uncovered",
        coverageReason: null,
        uncoveredIntervals: [{ startTime: "12:30", endTime: "14:00" }],
        ...overrides,
      };
    }

    function authenticateWithOverviewAccess() {
      mockUseSession.mockReturnValue({
        status: "authenticated",
        data: {
          user: {
            roles: ["admin"],
            permissions: [
              "schedules:manage",
              "schedules:read",
              "time_tracking:manage",
              "users:read",
            ],
          },
        },
      });
    }

    it("zählt die nicht abgedeckten Einsätze und verlinkt den Dienstplan", () => {
      authenticateWithOverviewAccess();
      setupSWR({
        coverageData: {
          dienstplanInUse: true,
          assignments: [coverageAssignment()],
        },
      });
      render(<VertretungView />);

      expect(
        screen.getByText("1 Einsatz ist nicht durch den Dienstplan abgedeckt."),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("link", { name: "Dienstplan öffnen" }),
      ).toHaveAttribute("href", "/acme/dienstplan?d=2026-07-15");
    });

    it("lässt abgedeckte Einsätze aus der Zählung", () => {
      authenticateWithOverviewAccess();
      setupSWR({
        coverageData: {
          dienstplanInUse: true,
          assignments: [
            coverageAssignment(),
            coverageAssignment({
              instanceId: "44",
              staffId: "12",
              coverageStatus: "covered",
              uncoveredIntervals: [],
            }),
          ],
        },
      });
      render(<VertretungView />);

      expect(
        screen.getByText("1 Einsatz ist nicht durch den Dienstplan abgedeckt."),
      ).toBeInTheDocument();
    });

    it("zählt nicht in die Störungszähler", () => {
      authenticateWithOverviewAccess();
      setupSWR({
        coverageData: {
          dienstplanInUse: true,
          assignments: [coverageAssignment()],
        },
      });
      render(<VertretungView />);

      expect(screen.getByText("Offen: 0")).toBeInTheDocument();
      expect(screen.getByText("Quittiert: 0")).toBeInTheDocument();
    });

    it("bleibt still ohne die Rechte der Dienstplan-Übersicht", () => {
      // Standard-Session aus beforeEach: nur schedules:manage.
      setupSWR({
        coverageData: {
          dienstplanInUse: true,
          assignments: [coverageAssignment()],
        },
      });
      render(<VertretungView />);

      expect(
        screen.queryByRole("link", { name: "Dienstplan öffnen" }),
      ).not.toBeInTheDocument();
    });

    it("bleibt still für Nicht-Administratoren mit den Rechten der Dienstplan-Übersicht", () => {
      setupSWR({
        coverageData: {
          dienstplanInUse: true,
          assignments: [coverageAssignment()],
        },
      });
      mockUseSession.mockReturnValue({
        status: "authenticated",
        data: {
          user: {
            permissions: [
              "schedules:manage",
              "schedules:read",
              "time_tracking:manage",
              "users:read",
            ],
          },
        },
      });
      render(<VertretungView />);

      expect(
        screen.queryByRole("link", { name: "Dienstplan öffnen" }),
      ).not.toBeInTheDocument();
    });

    it("bleibt still, solange kein Dienstplan gepflegt ist", () => {
      authenticateWithOverviewAccess();
      setupSWR({
        coverageData: {
          dienstplanInUse: false,
          assignments: [coverageAssignment()],
        },
      });
      render(<VertretungView />);

      expect(
        screen.queryByRole("link", { name: "Dienstplan öffnen" }),
      ).not.toBeInTheDocument();
    });

    it("bleibt still, wenn die Dienstplan-Abfrage fehlschlägt", () => {
      authenticateWithOverviewAccess();
      setupSWR({
        coverageData: {
          dienstplanInUse: true,
          assignments: [coverageAssignment()],
        },
        coverageError: new Error("Netzwerkfehler"),
      });
      render(<VertretungView />);

      expect(
        screen.queryByRole("link", { name: "Dienstplan öffnen" }),
      ).not.toBeInTheDocument();
    });

    it("bleibt still für einen vergangenen Tag", () => {
      authenticateWithOverviewAccess();
      mockSearch.value = "d=2026-07-14";
      setupSWR({
        coverageData: {
          dienstplanInUse: true,
          assignments: [coverageAssignment({ date: "2026-07-14" })],
        },
      });
      render(<VertretungView />);

      expect(
        screen.queryByRole("link", { name: "Dienstplan öffnen" }),
      ).not.toBeInTheDocument();
    });

    it("meldet in der Wochenansicht denselben Satz und öffnet die gezeigte Woche", () => {
      authenticateWithOverviewAccess();
      mockSearch.value = "view=woche";
      setupSWR({
        coverageData: {
          dienstplanInUse: true,
          assignments: [
            coverageAssignment({ date: "2026-07-16", instanceId: "43" }),
          ],
        },
      });
      render(<VertretungView />);

      expect(
        screen.getByText("1 Einsatz ist nicht durch den Dienstplan abgedeckt."),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("link", { name: "Dienstplan öffnen" }),
      ).toHaveAttribute("href", "/acme/dienstplan?d=2026-07-13");
    });
  });
});
