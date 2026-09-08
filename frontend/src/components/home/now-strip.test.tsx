import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { HomeBlockAccess, HomeBlockContext } from "~/lib/home-blocks";
import type { OwnAssignment } from "~/lib/shift-helpers";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

/**
 * Die Jetzt-Zone (#2180) fragt bis zu drei Quellen; welche, hängt an Rechten
 * und Betriebsmodus. Der Test stellt sie über den SWR-Schlüssel nach.
 */
const sources = vi.hoisted(() => ({
  own: undefined as OwnAssignment[] | undefined,
  ownError: undefined as Error | undefined,
  school: undefined as PlannedTimetableInstance[] | undefined,
  analytics: undefined as
    { studentsPresent: number; supervisorsToday: number } | undefined,
  requested: [] as (string | null)[],
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    sources.requested.push(key);
    if (key === null) return { data: undefined, error: undefined };
    if (key.startsWith("time-tracking-own-assignments-today-")) {
      return { data: sources.own, error: sources.ownError };
    }
    if (key === "home-day-flow") return { data: sources.school };
    if (key === "dashboard-analytics") return { data: sources.analytics };
    return { data: undefined, error: undefined };
  },
}));
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-09-08",
}));
vi.mock("~/components/home/home-card-rows", () => ({
  useBerlinClock: () => "10:20",
}));
vi.mock("~/lib/hooks/use-day-plan-href", () => ({
  useDayPlanHref: () => "/test-tenant/tagesplan",
  useDayPlanLabel: () => "Zum Tagesplan",
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));
const supervision = vi.hoisted(() => ({
  ownSupervision: false,
  hasGroups: true,
}));
vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => supervision,
}));
vi.mock("~/lib/shift-api", () => ({
  ownShiftService: { getOwnAssignments: vi.fn() },
}));
vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: { plannedNow: vi.fn() },
}));
vi.mock("~/lib/dashboard-api", () => ({
  fetchDashboardAnalyticsClient: vi.fn(),
}));

import { NowStrip } from "./now-strip";

const care: HomeBlockAccess = {
  isAdminScope: false,
  has: () => true,
  canOpenRequestsPage: false,
  caresForGroups: true,
  hasOwnGroups: true,
};
const lead: HomeBlockAccess = {
  isAdminScope: true,
  has: () => true,
  canOpenRequestsPage: true,
  caresForGroups: false,
  hasOwnGroups: false,
};

function context(access: HomeBlockAccess): HomeBlockContext {
  return {
    detailed: true,
    openCareGroupMode: false,
    nfcEnabled: true,
    birthdaysEnabled: true,
    timetableEnabled: true,
    remindersEnabled: true,
    messagingEnabled: false,
    staffMessagingEnabled: false,
    access,
  };
}

function assignment(overrides: Partial<OwnAssignment> = {}): OwnAssignment {
  return {
    instanceId: "1",
    date: "2026-09-08",
    startTime: "10:00",
    endTime: "11:00",
    title: "Lernzeit Jahrgang 1",
    groupName: null,
    roomName: "OGS-Raum 1",
    status: "planned",
    cancelled: false,
    isPrimary: true,
    isAbsent: false,
    isSubstitute: false,
    absenceReason: null,
    cancelReason: null,
    understaffedAck: false,
    ...overrides,
  };
}

function block(
  overrides: Partial<PlannedTimetableInstance> = {},
): PlannedTimetableInstance {
  return {
    id: "1",
    title: "Mittagessen",
    date: "2026-09-08",
    startTime: "12:00",
    endTime: "13:00",
    roomId: "7",
    roomName: "Mensa",
    status: "planned",
    isOverdue: false,
    minutesUntilStart: 100,
    ...overrides,
  } as PlannedTimetableInstance;
}

describe("NowStrip (#2180)", () => {
  beforeEach(() => {
    sources.own = undefined;
    sources.ownError = undefined;
    sources.school = undefined;
    sources.analytics = undefined;
    sources.requested = [];
    supervision.ownSupervision = false;
    supervision.hasGroups = true;
  });

  it("zeigt die Uhrzeit, den laufenden Einsatz und den Weg in den Tag", () => {
    sources.own = [
      assignment(),
      assignment({
        instanceId: "2",
        startTime: "12:00",
        endTime: "13:00",
        title: "Mittagessen",
        roomName: "Mensa",
        isSubstitute: true,
      }),
    ];

    render(<NowStrip access={care} context={context(care)} />);

    expect(screen.getByText("10:20")).toBeInTheDocument();
    expect(screen.getByText("Lernzeit Jahrgang 1")).toBeInTheDocument();
    expect(screen.getByText("Läuft")).toBeInTheDocument();
    expect(
      screen.getByText("OGS-Raum 1 · bis 11:00 · danach 12:00 Mittagessen"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Zum Tagesplan" })).toHaveAttribute(
      "href",
      "/test-tenant/tagesplan",
    );
    expect(screen.getByRole("link", { name: "Meine Gruppe" })).toHaveAttribute(
      "href",
      "/test-tenant/ogs-groups",
    );
  });

  it("nennt den nächsten Einsatz mit der Zeit bis dahin", () => {
    sources.own = [
      assignment({ startTime: "12:00", endTime: "13:00", isSubstitute: true }),
    ];

    render(<NowStrip access={care} context={context(care)} />);

    expect(
      screen.getByText("Als Nächstes: Lernzeit Jahrgang 1"),
    ).toBeInTheDocument();
    expect(screen.getByText("in 1 Std 40 Min")).toBeInTheDocument();
    expect(screen.getByText("Vertretung")).toBeInTheDocument();
  });

  it("stellt eine laufende Aufsicht als ersten Weg voran", () => {
    sources.own = [];
    supervision.ownSupervision = true;

    render(<NowStrip access={care} context={context(care)} />);

    const links = screen.getAllByRole("link");
    expect(links[0]).toHaveTextContent("Aufsicht fortsetzen");
    expect(links[0]).toHaveAttribute(
      "href",
      "/test-tenant/active-supervisions",
    );
  });

  // Die Leitung sieht die Lage der Schule: Blöcke, Kinder, Team.
  it("zeigt der Leitung die Lage der Schule", () => {
    sources.school = [
      block({ id: "1", status: "active", title: "Lernzeit" }),
      block({ id: "2", status: "active", title: "Hausaufgaben" }),
      block({ id: "3", startTime: "10:00", endTime: "11:00", isOverdue: true }),
      block({ id: "4" }),
    ];
    sources.analytics = { studentsPresent: 84, supervisorsToday: 7 };

    render(<NowStrip access={lead} context={context(lead)} />);

    expect(screen.getByText("2 Blöcke laufen")).toBeInTheDocument();
    expect(screen.getByText("1 nicht gestartet")).toBeInTheDocument();
    expect(
      screen.getByText(
        "84 Kinder da · 7 vom Team in Aufsicht · als Nächstes 12:00 Mittagessen (in 1 Std 40 Min)",
      ),
    ).toBeInTheDocument();
    // Ein reines Adminkonto hat keine Einsätze; die Abfrage bleibt aus.
    expect(
      sources.requested.some((key) =>
        key?.startsWith("time-tracking-own-assignments-today-"),
      ),
    ).toBe(false);
  });

  // Ein Fehler in einer Quelle nimmt die Zone nicht mit.
  it("trägt bei einem Ladefehler nur Uhrzeit und Wege", () => {
    sources.ownError = new Error("boom");

    render(<NowStrip access={care} context={context(care)} />);

    expect(screen.getByText("10:20")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Zum Tagesplan" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Läuft/)).not.toBeInTheDocument();
  });
});
