import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

const swr = vi.hoisted(() => ({
  data: undefined as PlannedTimetableInstance[] | undefined,
  error: undefined as Error | undefined,
  isLoading: false,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => swr,
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));
// Wohin „der Tag" führt, hängt an der Rolle und hat einen eigenen Test.
vi.mock("~/lib/hooks/use-day-plan-href", () => ({
  useDayPlanHref: () => "/test-tenant/tagesplan",
  useDayPlanLabel: () => "Zum Tagesplan",
}));
vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: { plannedNow: vi.fn() },
}));
// Feste Uhrzeit: die Karte zeigt ab dem laufenden Block, und ein Test, der an
// der Wanduhr hängt, wird am Nachmittag rot.
vi.mock("~/components/home/home-card-rows", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/components/home/home-card-rows")
  >()),
  useBerlinClock: () => "07:00",
}));

import { DayFlowBlock } from "./day-flow-block";

function block(
  overrides: Partial<PlannedTimetableInstance> = {},
): PlannedTimetableInstance {
  return {
    id: "1",
    title: "Lernzeit Jahrgang 1",
    date: "2026-09-07",
    startTime: "13:00",
    endTime: "14:00",
    roomId: "7",
    roomName: "OGS-Raum 1",
    status: "planned",
    isOverdue: false,
    minutesUntilStart: 10,
    expectedStudentsCount: 18,
    presentStudentsCount: 0,
    notScheduledStudentsCount: 0,
    assignedStaffIds: [],
    isAssigned: false,
    isPrimary: false,
    isSubstitute: false,
    isAbsent: false,
    rosterPreview: [],
    ...overrides,
  };
}

describe("DayFlowBlock (#2180)", () => {
  beforeEach(() => {
    swr.data = undefined;
    swr.error = undefined;
    swr.isLoading = false;
  });

  it("zeigt Zeit, Name, Ort und Belegung eines Blocks", () => {
    swr.data = [block()];

    render(<DayFlowBlock />);

    expect(screen.getByText("13:00–14:00")).toBeInTheDocument();
    expect(screen.getByText("Lernzeit Jahrgang 1")).toBeInTheDocument();
    expect(screen.getByText("· OGS-Raum 1 · 0/18 Kinder")).toBeInTheDocument();
    expect(screen.getByText("Geplant")).toBeInTheDocument();
  });

  // Die Karte hat Platz für drei Zeilen. Nimmt sie stumpf die ersten drei des
  // Tages, steht dort um 16 Uhr immer noch die Frühbetreuung.
  it("beginnt beim Block, der gerade läuft", () => {
    swr.data = [
      block({
        id: "1",
        title: "Frühdienst",
        startTime: "06:00",
        endTime: "06:45",
      }),
      block({
        id: "2",
        title: "Mittagessen",
        startTime: "06:45",
        endTime: "06:59",
      }),
      block({
        id: "3",
        title: "Lernzeit",
        startTime: "07:00",
        endTime: "08:00",
      }),
      block({
        id: "4",
        title: "Freispiel",
        startTime: "08:00",
        endTime: "09:00",
      }),
    ];

    // Die Uhr steht im Test auf 07:00.
    render(<DayFlowBlock />);

    expect(screen.queryByText("Frühdienst")).not.toBeInTheDocument();
    expect(screen.queryByText("Mittagessen")).not.toBeInTheDocument();
    expect(screen.getByText("Lernzeit")).toBeInTheDocument();
    expect(screen.getByText("Freispiel")).toBeInTheDocument();
  });

  // Ist der Tag vorbei, bleibt der Tag stehen: „heute war das" ist eine
  // Antwort, eine leere Karte ist keine.
  it("lässt einen vergangenen Tag stehen, statt zu leeren", () => {
    swr.data = [
      block({
        id: "1",
        title: "Frühdienst",
        startTime: "05:00",
        endTime: "05:45",
      }),
      block({
        id: "2",
        title: "Mittagessen",
        startTime: "05:45",
        endTime: "06:30",
      }),
    ];

    render(<DayFlowBlock />);

    expect(screen.getByText("Frühdienst")).toBeInTheDocument();
    expect(screen.getByText("Mittagessen")).toBeInTheDocument();
  });

  it("kennzeichnet laufende, ausgefallene und überfällige Blöcke", () => {
    swr.data = [
      block({ id: "1", status: "active" }),
      block({ id: "2", status: "cancelled" }),
      block({ id: "3", isOverdue: true }),
    ];

    render(<DayFlowBlock />);

    expect(screen.getByText("Läuft")).toBeInTheDocument();
    expect(screen.getByText("Entfällt")).toBeInTheDocument();
    expect(screen.getByText("Überfällig")).toBeInTheDocument();
  });

  it("sagt es, wenn gerade nichts ansteht", () => {
    swr.data = [];

    render(<DayFlowBlock />);

    expect(screen.getByText("Gerade steht nichts an")).toBeInTheDocument();
  });

  // Ein Ladefehler darf nicht wie "nichts geplant" aussehen: sonst verlässt
  // sich jemand auf einen leeren Tag, den es nicht gibt.
  it("unterscheidet einen Ladefehler von einem leeren Tag", () => {
    swr.error = new Error("boom");

    render(<DayFlowBlock />);

    expect(screen.getByText(/konnte nicht geladen werden/)).toBeInTheDocument();
    expect(
      screen.queryByText("Gerade steht nichts an"),
    ).not.toBeInTheDocument();
  });
});
