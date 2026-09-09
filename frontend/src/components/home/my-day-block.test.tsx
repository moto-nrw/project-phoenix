import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

const swr = vi.hoisted(() => ({
  data: undefined as PlannedTimetableInstance[] | undefined,
  error: undefined as Error | undefined,
  isLoading: false,
  mutate: vi.fn(),
}));
const api = vi.hoisted(() => ({
  plannedNow: vi.fn(),
  start: vi.fn(),
}));
const router = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => swr,
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => router,
}));
vi.mock("~/lib/hooks/use-day-plan-href", () => ({
  useDayPlanHref: () => "/test-tenant/tagesplan",
  useDayPlanLabel: () => "Zum Tagesplan",
}));
vi.mock("~/lib/timetable-operations-api", () => ({
  timetableOperationsApi: api,
}));
// Feste Uhrzeit: die Jetzt-Linie und das Dimmen hängen an ihr.
vi.mock("~/components/home/home-card-rows", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/components/home/home-card-rows")
  >()),
  useBerlinClock: () => "13:10",
}));

import { MyDayBlock } from "./my-day-block";

function block(
  overrides: Partial<PlannedTimetableInstance> = {},
): PlannedTimetableInstance {
  return {
    id: "1",
    title: "Lernzeit Jahrgang 1",
    date: "2026-09-08",
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
    isAssigned: true,
    isPrimary: true,
    isSubstitute: false,
    isAbsent: false,
    rosterPreview: [],
    canStart: false,
    ...overrides,
  };
}

const day: PlannedTimetableInstance[] = [
  block({
    id: "1",
    title: "Frühbetreuung",
    startTime: "07:30",
    endTime: "08:30",
    status: "completed",
    presentStudentsCount: 15,
    staffNames: [
      { staffId: "1", displayName: "Julia Klein", isSubstitute: false },
      { staffId: "2", displayName: "Sabine Weber", isSubstitute: false },
    ],
  }),
  block({
    id: "2",
    title: "Bewegungszeit",
    startTime: "08:45",
    endTime: "09:45",
    isAssigned: false,
  }),
  block({
    id: "3",
    title: "Mittagessen",
    startTime: "12:00",
    endTime: "13:00",
    roomName: "Mensa",
    groupName: "Mittagessen",
    isSubstitute: true,
  }),
  block({
    id: "4",
    title: "Hausaufgaben",
    startTime: "13:15",
    endTime: "14:00",
    roomName: "OGS-Raum 2",
    canStart: true,
  }),
  block({
    id: "5",
    title: "Fußball-AG",
    startTime: "15:00",
    endTime: "16:00",
    roomName: "Sporthalle",
  }),
];

describe("MyDayBlock (#2180)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    swr.data = day;
    swr.error = undefined;
    swr.isLoading = false;
  });

  it("zeigt aktuelle und kommende eigene Einsätze mit ihren Angaben", () => {
    render(<MyDayBlock />);

    // Vergangenes macht in der kompakten Karte Platz für das, was jetzt zählt.
    expect(screen.queryByText("Frühbetreuung")).not.toBeInTheDocument();
    expect(screen.getByText("Hausaufgaben")).toBeInTheDocument();
    expect(screen.getByText("Fußball-AG")).toBeInTheDocument();
    // Fremde Blöcke gehören in den Tagesplan, nicht in „Mein Tag".
    expect(screen.queryByText("Bewegungszeit")).not.toBeInTheDocument();
    expect(screen.getByText("OGS-Raum 2 · 18 Kinder")).toBeInTheDocument();
    expect(screen.getByText("Sporthalle · 18 Kinder")).toBeInTheDocument();
  });

  it("setzt die Jetzt-Linie vor den Block, der als Nächstes zählt", () => {
    render(<MyDayBlock />);

    const rows = screen.getAllByRole("listitem");
    // 07:30 und 12:00 sind vorbei (13:10): sichtbar ist die Linie vor 13:15.
    expect(rows[0]).toHaveTextContent("Jetzt · 13:10 Uhr");
    expect(rows[0]).toHaveTextContent("Hausaufgaben");
    expect(rows[1]).not.toHaveTextContent("Jetzt");
  });

  it("zeigt bei vielen Einsätzen den laufenden und den nächsten zuerst", () => {
    swr.data = [
      block({
        id: "1",
        title: "Frühdienst",
        startTime: "07:30",
        endTime: "08:00",
        status: "completed",
      }),
      block({
        id: "2",
        title: "Frühe Lernzeit",
        startTime: "08:15",
        endTime: "09:00",
        status: "completed",
      }),
      block({
        id: "3",
        title: "Frühe Pause",
        startTime: "09:15",
        endTime: "10:00",
        status: "completed",
      }),
      block({
        id: "4",
        title: "Vormittagsbetreuung",
        startTime: "10:15",
        endTime: "11:00",
        status: "completed",
      }),
      block({
        id: "5",
        title: "Laufende Betreuung",
        startTime: "13:00",
        endTime: "14:00",
        status: "active",
      }),
      block({
        id: "6",
        title: "Nächster Einsatz",
        startTime: "14:15",
        endTime: "15:00",
      }),
    ];

    render(<MyDayBlock />);

    expect(screen.getByText("Laufende Betreuung")).toBeInTheDocument();
    expect(screen.getByText("Nächster Einsatz")).toBeInTheDocument();
    expect(screen.queryByText("Frühdienst")).not.toBeInTheDocument();
    expect(screen.getAllByRole("listitem")[0]).toHaveTextContent(
      "Jetzt · 13:10 Uhr",
    );
  });

  it("startet den eigenen Block direkt aus der Karte", async () => {
    api.start.mockResolvedValue({ activeGroupId: "77" });

    render(<MyDayBlock />);

    fireEvent.click(screen.getByRole("button", { name: "Starten" }));

    await waitFor(() => expect(api.start).toHaveBeenCalledWith("4"));
    expect(router.push).toHaveBeenCalledWith("/active-supervisions?session=77");
  });

  it("sagt es, wenn das Starten scheitert, und lädt neu", async () => {
    api.start.mockRejectedValue(new Error("boom"));

    render(<MyDayBlock />);

    fireEvent.click(screen.getByRole("button", { name: "Starten" }));

    await waitFor(() =>
      expect(
        screen.getByText(/konnte nicht gestartet werden/),
      ).toBeInTheDocument(),
    );
    expect(swr.mutate).toHaveBeenCalled();
    expect(router.push).not.toHaveBeenCalled();
  });

  it("führt aus einem laufenden Block in seine Kinderliste", () => {
    swr.data = [
      block({
        id: "9",
        title: "Frühbetreuung",
        status: "active",
        activeGroupId: "55",
        presentStudentsCount: 12,
      }),
    ];

    render(<MyDayBlock />);

    expect(
      screen.getByRole("link", {
        name: "Frühbetreuung 13:00 bis 14:00: Kinderliste öffnen",
      }),
    ).toHaveAttribute("href", "/active-supervisions?session=55");
    expect(screen.getByText("Läuft")).toBeInTheDocument();
    expect(
      screen.getByText("12 von 18 da", { exact: false }),
    ).toBeInTheDocument();
  });

  it("nennt einen abgesagten Block mit Grund", () => {
    swr.data = [
      block({ status: "cancelled", cancelReason: "Lehrkraft krank" }),
    ];

    render(<MyDayBlock />);

    expect(screen.getByText("Fällt aus · Lehrkraft krank")).toBeInTheDocument();
  });

  it("sagt es, wenn heute kein eigener Block ansteht", () => {
    swr.data = [block({ isAssigned: false })];

    render(<MyDayBlock />);

    expect(
      screen.getByText("Heute sind Sie für keinen Block eingeteilt"),
    ).toBeInTheDocument();
  });

  it("unterscheidet einen Ladefehler von einem leeren Tag", () => {
    swr.data = undefined;
    swr.error = new Error("boom");

    render(<MyDayBlock />);

    expect(screen.getByText(/konnte nicht geladen werden/)).toBeInTheDocument();
  });
});
